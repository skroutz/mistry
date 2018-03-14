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

	err := filepath.Walk("/Users/agis/dev/mistry-projects/sample", walkFn)
	if err != nil {
		return err
	}

	err = tw.Close()
	if err != nil {
		return err
	}

	_, err = c.ImageBuild(context.Background(), &buf, types.ImageBuildOptions{})
	if err != nil {
		return err
	}

	return nil
}
