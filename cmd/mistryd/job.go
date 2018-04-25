package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/skroutz/mistry/pkg/filesystem"
	"github.com/skroutz/mistry/pkg/types"
	"github.com/skroutz/mistry/pkg/utils"
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

	StartedAt time.Time

	// webview-related
	Output string
	Log    template.HTML
	State  string
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

func (j *Job) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID        string        `json:"id"`
		Project   string        `json:"project"`
		StartedAt string        `json:"startedAt"`
		Output    string        `json:"output"`
		Log       template.HTML `json:"log"`
		State     string        `json:"state"`
	}{
		ID:        j.ID,
		Project:   j.Project,
		StartedAt: j.StartedAt.Format(DateFmt),
		Output:    j.Output,
		Log:       template.HTML(j.Log),
		State:     j.State,
	})
}

func (j *Job) UnmarshalJSON(data []byte) error {
	jData := &struct {
		ID        string `json:"id"`
		Project   string `json:"project"`
		StartedAt string `json:"startedAt"`
		Output    string `json:"output"`
		Log       string `json:"log"`
		State     string `json:"state"`
	}{}
	err := json.Unmarshal(data, &jData)
	if err != nil {
		return err
	}
	j.ID = jData.ID
	j.Project = jData.Project
	j.StartedAt, err = time.Parse(DateFmt, jData.StartedAt)
	if err != nil {
		return err
	}
	j.Output = jData.Output
	j.Log = template.HTML(jData.Log)
	j.State = jData.State

	return nil
}

// GetState determines the job's current state by using it's path in the filesystem.
func GetState(path, project, id string) (string, error) {
	pPath := filepath.Join(path, project, "pending", id)
	rPath := filepath.Join(path, project, "ready", id)
	_, err := os.Stat(pPath)
	if err == nil {
		return "pending", nil
	}
	_, err = os.Stat(rPath)
	if err == nil {
		return "ready", nil
	}
	return "", fmt.Errorf("job with id=%s not found error", id)
}

// CloneSrcPath determines if we should use the build cache and return the path that
// should be used.
//
// Using the build cache should happen when
// 1. the job is invoked with a group
// 2. the symlink pointing to the latest build is valid
func (j *Job) CloneSrcPath(log *log.Logger) string {
	cloneSrc := ""
	if j.Group != "" {
		var symlinkErr error
		cloneSrc, symlinkErr = filepath.EvalSymlinks(j.LatestBuildPath)
		if symlinkErr != nil {
			// dont clone anything if we get an error reading the symlink
			cloneSrc = ""
			if os.IsNotExist(symlinkErr) {
				log.Printf("no latest build was found: %s", symlinkErr)
			} else {
				log.Printf("could not read latest build link, error: %s", symlinkErr)
			}
		}
	}
	return cloneSrc
}

// BootstrapBuildDir creates all required build directories. Returns true if directories have
// been partially or fully created and cleanup is required
func (j *Job) BootstrapBuildDir(fs filesystem.FileSystem, log *log.Logger) (bool, error) {
	shouldCleanup := false
	var cmd []string

	cloneSrc := j.CloneSrcPath(log)

	if cloneSrc == "" {
		cmd = fs.Create(j.PendingBuildPath)
	} else {
		cmd = fs.Clone(cloneSrc, j.PendingBuildPath)
	}
	out, err := utils.RunCmd(cmd)
	if out != "" {
		log.Println(out)
	}
	if err != nil {
		return shouldCleanup, workErr("could not create pending build path", err)
	}
	shouldCleanup = true

	// if we cloned, empty the params dir
	if cloneSrc != "" {
		err = os.RemoveAll(filepath.Join(j.PendingBuildPath, DataDir, ParamsDir))
		if err != nil {
			return shouldCleanup, workErr("could not remove params dir", err)
		}
	}

	dirs := [4]string{
		filepath.Join(j.PendingBuildPath, DataDir),
		filepath.Join(j.PendingBuildPath, DataDir, CacheDir),
		filepath.Join(j.PendingBuildPath, DataDir, ArtifactsDir),
		filepath.Join(j.PendingBuildPath, DataDir, ParamsDir),
	}

	for _, dir := range dirs {
		err = utils.EnsureDirExists(dir)
		if err != nil {
			return shouldCleanup, workErr("could not ensure directory exists", err)
		}
		log.Printf("created dir: %s", dir)
	}
	return shouldCleanup, nil
}