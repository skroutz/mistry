package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
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
	DockerfilePath   string
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

	j.DockerfilePath = filepath.Join(cfg.ProjectPath, j.Project, "Dockerfile")

	return j, nil
}

func (j *Job) BuildImage(c *docker.Client) {
	res, err := c.ImageBuild(
		context.Background(), nil,
		types.ImageBuildOptions{Dockerfile: j.DockerfilePath})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res)
}
