package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
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
	"github.com/skroutz/mistry/utils"
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
	BuildLogPath        string
	BuildResultFilePath string

	// docker image tar
	ImageTar []byte
}

func NewJob(project string, params map[string]string, group string) (*Job, error) {
	var err error

	if project == "" {
		return nil, errors.New("No project given")
	}

	j := new(Job)
	j.Project = project
	j.Group = group
	j.Params = params
	j.ProjectPath = filepath.Join(cfg.ProjectsPath, j.Project)
	j.RootBuildPath = filepath.Join(cfg.BuildPath, j.Project)

	if j.Group == "" {
		j.LatestBuildPath = filepath.Join(j.RootBuildPath, "latest")
	} else {
		j.LatestBuildPath = filepath.Join(j.RootBuildPath, "groups", j.Group)
	}

	j.ImageTar, err = utils.Tar(j.ProjectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Unknown project '%s'", j.Project)
		}
		return nil, err
	}

	// compute ID
	keys := []string{}
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	seed := project + group
	for _, v := range keys {
		seed += v + params[v]
	}
	seed += string(j.ImageTar)

	j.ID = fmt.Sprintf("%x", sha256.Sum256([]byte(seed)))

	j.PendingBuildPath = filepath.Join(j.RootBuildPath, "pending", j.ID)
	j.ReadyBuildPath = filepath.Join(j.RootBuildPath, "ready", j.ID)
	j.ReadyDataPath = filepath.Join(j.ReadyBuildPath, DataDir)
	j.BuildLogPath = filepath.Join(j.PendingBuildPath, BuildLogFname)
	j.BuildResultFilePath = filepath.Join(j.PendingBuildPath, BuildResultFname)

	return j, nil
}

func (j *Job) BuildImage(ctx context.Context, c *docker.Client, out io.Writer) error {
	buildArgs := make(map[string]*string)
	buildArgs["uid"] = &cfg.UID
	buildOpts := types.ImageBuildOptions{Tags: []string{j.Project}, BuildArgs: buildArgs, NetworkMode: "host"}
	resp, err := c.ImageBuild(context.Background(), bytes.NewBuffer(j.ImageTar), buildOpts)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	_, _, err = c.ImageInspectWithRaw(context.Background(), j.Project)
	if err != nil {
		return err
	}

	return nil
}

// StartContainer creates and runs the container. It blocks until the container exits.
// It returns the exit code of the container command. If there was an error
// starting the container, the exit code is irrelevant.
//
// NOTE: If there was an error with the user's dockerfile, the returned exit code will be 1 and the error nil.
// TODO: block until container exits
func (j *Job) StartContainer(ctx context.Context, c *docker.Client, out io.Writer) (int, error) {
	config := container.Config{User: cfg.UID, Image: j.Project}

	mnts := []mount.Mount{{Type: mount.TypeBind, Source: filepath.Join(j.PendingBuildPath, DataDir), Target: DataDir}}
	for src, target := range cfg.Mounts {
		mnts = append(mnts, mount.Mount{Type: mount.TypeBind, Source: src, Target: target})
	}

	hostConfig := container.HostConfig{Mounts: mnts, AutoRemove: true, NetworkMode: "host"}

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
