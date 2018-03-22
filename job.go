package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	docker "github.com/docker/docker/client"
)

type Job struct {
	ID string

	// User-provided
	Project string
	// TODO: this should be its own type probably
	Params map[string]string
	Group  string

	RootBuildPath    string
	PendingBuildPath string
	ReadyBuildPath   string
	LatestBuildPath  string
	ReadyDataPath    string

	ProjectPath string

	// NOTE: after a job is complete, this points to an invalid path
	// (pending)
	BuildLogPath string
}

func NewJob(project string, params map[string]string, group string) (*Job, error) {
	if project == "" {
		return nil, errors.New("No project given")
	}

	j := new(Job)

	keys := []string{}
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	seed := project + group
	for _, v := range keys {
		seed += v + params[v]
	}
	j.ID = fmt.Sprintf("%x", sha256.Sum256([]byte(seed)))
	j.Project = project
	j.Group = group
	j.Params = params
	j.RootBuildPath = filepath.Join(cfg.BuildPath, j.Project)
	j.PendingBuildPath = filepath.Join(j.RootBuildPath, "pending", j.ID)
	j.ReadyBuildPath = filepath.Join(j.RootBuildPath, "ready", j.ID)
	j.ReadyDataPath = filepath.Join(j.RootBuildPath, "ready", j.ID, DataDir)

	if j.Group == "" {
		j.LatestBuildPath = filepath.Join(j.RootBuildPath, "latest")
	} else {
		j.LatestBuildPath = filepath.Join(j.RootBuildPath, "groups", j.Group)
	}

	j.ProjectPath = filepath.Join(cfg.ProjectsPath, j.Project)
	j.BuildLogPath = filepath.Join(j.PendingBuildPath, BuildLogName)

	return j, nil
}

func (j *Job) BuildImage(ctx context.Context, c *docker.Client, out io.Writer) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		hdr, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		err = tw.WriteHeader(hdr)
		if err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(tw, f)
		if err != nil {
			return err
		}

		err = f.Close()
		if err != nil {
			return err
		}

		return nil
	}

	err := filepath.Walk(j.ProjectPath, walkFn)
	if err != nil {
		return err
	}

	err = tw.Close()
	if err != nil {
		return err
	}

	buildArgs := make(map[string]*string)
	buildArgs["uid"] = &cfg.UID
	buildOpts := types.ImageBuildOptions{Tags: []string{j.Project}, BuildArgs: buildArgs}
	resp, err := c.ImageBuild(context.Background(), &buf, buildOpts)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// StartContainer creates and runs the container. It blocks until the container exits.
// It returns the exit code of the container command. If there was an error
// starting the container, the exit code is irrelevant.
//
// TODO: block until container exits
func (j *Job) StartContainer(ctx context.Context, c *docker.Client, out io.Writer) (int, error) {
	config := container.Config{User: cfg.UID, Image: j.Project}

	mnts := []mount.Mount{{Type: mount.TypeBind, Source: filepath.Join(j.PendingBuildPath, DataDir), Target: DataDir}}
	for src, target := range cfg.Mounts {
		mnts = append(mnts, mount.Mount{Type: mount.TypeBind, Source: src, Target: target})
	}

	hostConfig := container.HostConfig{Mounts: mnts, AutoRemove: true}

	res, err := c.ContainerCreate(ctx, &config, &hostConfig, nil, j.ID)
	if err != nil {
		return 0, err
	}

	// TODO: use an actual ctx for shutting down
	err = c.ContainerStart(ctx, res.ID, types.ContainerStartOptions{})
	if err != nil {
		return 0, err
	}

	resp, err := c.ContainerAttach(ctx, res.ID, types.ContainerAttachOptions{
		Stream: true, Stdin: true, Stdout: true, Stderr: true, Logs: true})
	if err != nil {
		return 0, err
	}
	defer resp.Close()

	_, err = io.Copy(out, resp.Reader)
	if err != nil {
		return 0, err
	}

	var result struct {
		State struct {
			ExitCode int
		}
	}

	_, inspect, err := c.ContainerInspectWithRaw(ctx, res.ID, false)
	if err != nil {
		return 0, err
	}

	err = json.Unmarshal(inspect, &result)
	if err != nil {
		return 0, err
	}

	return result.State.ExitCode, nil
}
