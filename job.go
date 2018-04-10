package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/skroutz/mistry/types"
	"github.com/skroutz/mistry/utils"
)

// Job is the core unit of work. It is essentially something that needs to
// be executed in order to produce the desired artifacts.
type Job struct {
	ID string

	// user-provided
	Project string
	Params  types.Params
	Group   string

	RootBuildPath    string
	PendingBuildPath string
	ReadyBuildPath   string
	LatestBuildPath  string
	ReadyDataPath    string

	ProjectPath string

	// NOTE: after a job is complete, this points to an invalid (pending)
	// path
	BuildLogPath        string
	BuildResultFilePath string

	// docker-related
	Image     string
	ImageTar  []byte
	Container string
}

// NewJob returns a new Job for the given project. project and cfg cannot be
// empty.
func NewJob(project string, params types.Params, group string, cfg *Config) (*Job, error) {
	var err error

	if project == "" {
		return nil, errors.New("no project given")
	}

	if cfg == nil {
		return nil, errors.New("invalid configuration")
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

	j.Image = ImgCntPrefix + j.ID
	j.Container = ImgCntPrefix + j.ID

	return j, nil
}

// BuildImage builds the Docker image denoted by j.Image. If there is an
// error, it will be of type types.ErrImageBuild.
func (j *Job) BuildImage(ctx context.Context, uid string, c *docker.Client, out io.Writer) error {
	buildArgs := make(map[string]*string)
	buildArgs["uid"] = &uid
	buildOpts := dockertypes.ImageBuildOptions{Tags: []string{j.Image}, BuildArgs: buildArgs, NetworkMode: "host"}
	resp, err := c.ImageBuild(context.Background(), bytes.NewBuffer(j.ImageTar), buildOpts)
	if err != nil {
		return types.ErrImageBuild{Image: j.Image, Err: err}
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return types.ErrImageBuild{Image: j.Image, Err: err}
	}

	_, _, err = c.ImageInspectWithRaw(context.Background(), j.Image)
	if err != nil {
		return types.ErrImageBuild{Image: j.Image, Err: err}
	}

	return nil
}

// StartContainer creates and runs the container. It blocks until the container
// exits and returns the exit code of the container command. If there was an error
// starting the container, the exit code is irrelevant.
//
// NOTE: If there was an error with the user's dockerfile, the returned exit
// code will be 1 and the error nil.
func (j *Job) StartContainer(ctx context.Context, cfg *Config, c *docker.Client, out io.Writer) (int, error) {
	config := container.Config{User: cfg.UID, Image: j.Image}

	mnts := []mount.Mount{{Type: mount.TypeBind, Source: filepath.Join(j.PendingBuildPath, DataDir), Target: DataDir}}
	for src, target := range cfg.Mounts {
		mnts = append(mnts, mount.Mount{Type: mount.TypeBind, Source: src, Target: target})
	}

	hostConfig := container.HostConfig{Mounts: mnts, AutoRemove: false, NetworkMode: "host"}

	res, err := c.ContainerCreate(ctx, &config, &hostConfig, nil, j.Container)
	if err != nil {
		return 0, err
	}

	err = c.ContainerStart(ctx, res.ID, dockertypes.ContainerStartOptions{})
	if err != nil {
		return 0, err
	}

	defer func(id string) {
		err = c.ContainerRemove(ctx, id, dockertypes.ContainerRemoveOptions{})
		if err != nil {
			log.Printf("[%s] cannot remove container: %s", j, err)
		}
	}(res.ID)

	logs, err := c.ContainerLogs(ctx, res.ID,
		dockertypes.ContainerLogsOptions{Follow: true, ShowStdout: true, ShowStderr: true,
			Details: true})
	if err != nil {
		return 0, err
	}

	_, err = stdcopy.StdCopy(out, out, logs)
	if err != nil {
		return 0, err
	}
	logs.Close()

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

func (j *Job) String() string {
	return fmt.Sprintf(
		"{project=%s params=%s group=%s id=%s}",
		j.Project, j.Params, j.Group, j.ID[:7])
}
