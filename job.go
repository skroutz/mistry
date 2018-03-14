package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
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
	Group   string
	Params  map[string]string

	PendingBuildPath string
	ReadyBuildPath   string
	LatestBuildPath  string

	ProjectPath string
}

func NewJob(project string, group string, params map[string]string) (*Job, error) {
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
	j.PendingBuildPath = filepath.Join(cfg.BuildPath, "pending", j.ID)
	j.ReadyBuildPath = filepath.Join(cfg.BuildPath, "ready", j.ID)

	if j.Group == "" {
		j.LatestBuildPath = filepath.Join(cfg.BuildPath, "latest")
	} else {
		j.LatestBuildPath = filepath.Join(cfg.BuildPath, "groups", j.Group)
	}

	j.ProjectPath = filepath.Join(cfg.ProjectPath, j.Project, "Dockerfile")

	return j, nil
}

func (j *Job) BuildImage(c *docker.Client) error {
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

	// TODO: fix path
	err := filepath.Walk("/Users/agis/dev/mistry-projects/sample", walkFn)
	if err != nil {
		return err
	}

	err = tw.Close()
	if err != nil {
		return err
	}

	buildArgs := make(map[string]*string)
	buildArgs["uid"] = &cfg.UID
	buildOpts := types.ImageBuildOptions{BuildArgs: buildArgs}
	res, err := c.ImageBuild(context.Background(), &buf, buildOpts)
	if err != nil {
		return err
	}

	// TODO: REMOVE, just for debugging
	response, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
		fmt.Printf("%s", err.Error())
	}
	fmt.Println(res)
	fmt.Println(string(response))

	return nil
}

// StartContainer creates and runs the container.
func (j *Job) StartContainer(c *docker.Client) error {
	config := container.Config{User: cfg.UID, Image: j.Project}

	// TODO: maybe "/data" should go in a config?
	mnts := []mount.Mount{{Type: mount.TypeBind, Source: filepath.Join(j.PendingBuildPath, "data"), Target: "/data"}}
	for src, target := range cfg.Mounts {
		mnts = append(mnts, mount.Mount{Type: mount.TypeBind, Source: src, Target: target})
	}

	// TODO: do we want auto-remove?
	hostConfig := container.HostConfig{Mounts: mnts, AutoRemove: true}

	// TODO: use an actual ctx for shutting down
	res, err := c.ContainerCreate(context.Background(), &config, &hostConfig, nil, j.ID)
	if err != nil {
		return err
	}

	// TODO: use an actual ctx for shutting down
	err = c.ContainerStart(context.Background(), res.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	// TODO: attach and stream logs somewhere
	resp, err := c.ContainerAttach(context.Background(), res.ID, types.ContainerAttachOptions{
		Stream: true, Stdin: true, Stdout: true, Stderr: true, Logs: true})
	if err != nil {
		return err
	}

	// TODO: this goes away. debugging purposes
	foo, err := ioutil.ReadAll(resp.Reader)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", foo)

	return nil
}
