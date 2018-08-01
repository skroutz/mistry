package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
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
	Rebuild bool

	RootBuildPath    string
	PendingBuildPath string
	ReadyBuildPath   string
	LatestBuildPath  string
	ReadyDataPath    string

	ProjectPath string

	// NOTE: after a job is complete, this points to an invalid (pending)
	// path
	BuildLogPath      string
	BuildInfoFilePath string

	// docker-related
	Image     string
	ImageTar  []byte
	Container string

	StartedAt time.Time

	BuildInfo *types.BuildInfo
	State     string
}

// NewJobFromRequest returns a new Job from the JobRequest
func NewJobFromRequest(jr types.JobRequest, cfg *Config) (*Job, error) {
	j, err := NewJob(jr.Project, jr.Params, jr.Group, cfg)
	if err != nil {
		return nil, err
	}
	j.Rebuild = jr.Rebuild
	return j, nil
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
		// params opaque to the build are not taken into account
		// when calculating a job's ID
		if strings.HasPrefix(k, "_") {
			continue
		}

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
	j.BuildLogPath = BuildLogPath(j.PendingBuildPath)
	j.BuildInfoFilePath = filepath.Join(j.PendingBuildPath, BuildInfoFname)

	j.Image = ImgCntPrefix + j.Project
	j.Container = ImgCntPrefix + j.ID

	j.StartedAt = time.Now()
	j.BuildInfo = new(types.BuildInfo)
	j.State = "pending"

	return j, nil
}

// BuildImage builds the Docker image denoted by j.Image. If there is an
// error, it will be of type types.ErrImageBuild.
func (j *Job) BuildImage(ctx context.Context, uid string, c *docker.Client, out io.Writer, pullParent, noCache bool) error {
	buildArgs := make(map[string]*string)
	buildArgs["uid"] = &uid
	buildOpts := dockertypes.ImageBuildOptions{
		Tags:        []string{j.Image},
		BuildArgs:   buildArgs,
		NetworkMode: "host",
		PullParent:  pullParent,
		NoCache:     noCache,
		ForceRemove: true,
	}
	resp, err := c.ImageBuild(context.Background(), bytes.NewBuffer(j.ImageTar), buildOpts)
	if err != nil {
		return types.ErrImageBuild{Image: j.Image, Err: err}
	}
	defer resp.Body.Close()

	err = jsonmessage.DisplayJSONMessagesStream(resp.Body, out, 0, false, nil)
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
func (j *Job) StartContainer(ctx context.Context, cfg *Config, c *docker.Client, out, outErr io.Writer) (int, error) {
	config := container.Config{User: cfg.UID, Image: j.Image}

	mnts := []mount.Mount{{Type: mount.TypeBind, Source: filepath.Join(j.PendingBuildPath, DataDir), Target: DataDir}}
	for src, target := range cfg.Mounts {
		mnts = append(mnts, mount.Mount{Type: mount.TypeBind, Source: src, Target: target})
	}

	hostConfig := container.HostConfig{Mounts: mnts, AutoRemove: false, NetworkMode: "host"}

	err := renameIfExists(ctx, c, j.Container)
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
	defer logs.Close()

	_, err = stdcopy.StdCopy(out, io.MultiWriter(out, outErr), logs)
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

// renameIfExists searches for containers with the passed name and renames them
// by appending a random suffix to their name
func renameIfExists(ctx context.Context, c *docker.Client, name string) error {
	filter := filters.NewArgs()
	filter.Add("name", name)
	containers, err := c.ContainerList(ctx, dockertypes.ContainerListOptions{
		Quiet:   true,
		All:     true,
		Limit:   -1,
		Filters: filter,
	})
	if err != nil {
		return err
	}
	for _, container := range containers {
		err := c.ContainerRename(ctx, container.ID, name+"-renamed-"+randomHexString())
		if err != nil {
			return err
		}
	}
	return nil
}

func randomHexString() string {
	buf := make([]byte, 16)
	rand.Read(buf)
	return hex.EncodeToString(buf)
}

func (j *Job) String() string {
	return fmt.Sprintf(
		"{project=%s group=%s id=%s}",
		j.Project, j.Group, j.ID[:7])
}

// MarshalJSON serializes the Job to JSON
func (j *Job) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID        string          `json:"id"`
		Project   string          `json:"project"`
		StartedAt string          `json:"startedAt"`
		BuildInfo types.BuildInfo `json:"buildInfo"`
		State     string          `json:"state"`
	}{
		ID:        j.ID,
		Project:   j.Project,
		StartedAt: j.StartedAt.Format(DateFmt),
		BuildInfo: *j.BuildInfo,
		State:     j.State,
	})
}

// UnmarshalJSON deserializes JSON data and updates the Job
// with them
func (j *Job) UnmarshalJSON(data []byte) error {
	jData := &struct {
		ID        string          `json:"id"`
		Project   string          `json:"project"`
		StartedAt string          `json:"startedAt"`
		BuildInfo types.BuildInfo `json:"buildInfo"`
		State     string          `json:"state"`
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
	j.BuildInfo = &jData.BuildInfo
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

// BootstrapBuildDir creates all required build directories. Cleans the
// pending directory if there were any errors.
func (j *Job) BootstrapBuildDir(fs filesystem.FileSystem, log *log.Logger) error {
	var err error

	cloneSrc := j.CloneSrcPath(log)

	if cloneSrc == "" {
		err = fs.Create(j.PendingBuildPath)
	} else {
		err = fs.Clone(cloneSrc, j.PendingBuildPath)
	}
	if err != nil {
		return workErr("could not create pending build path", err)
	}

	// if we cloned, empty the params dir
	if cloneSrc != "" {
		err = os.RemoveAll(filepath.Join(j.PendingBuildPath, DataDir, ParamsDir))
		if err != nil {
			return workErr("could not remove params dir", err)
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
			return workErr("could not ensure directory exists", err)
		}
		log.Printf("created dir: %s", dir)
	}
	return err
}

// BuildLogPath returns the path of the job logs found at jobPath
func BuildLogPath(jobPath string) string {
	return filepath.Join(jobPath, BuildLogFname)
}

// ReadJobLogs returns the job logs found at jobPath
func ReadJobLogs(jobPath string) ([]byte, error) {
	buildLogPath := BuildLogPath(jobPath)

	log, err := ioutil.ReadFile(buildLogPath)
	if err != nil {
		return nil, err
	}
	return log, nil
}

// ReadJobBuildInfo returns the BuildInfo found at jobPath
func ReadJobBuildInfo(path string, logs bool) (*types.BuildInfo, error) {
	buildInfoPath := filepath.Join(path, BuildInfoFname)
	buildInfo := types.NewBuildInfo()

	buildInfoBytes, err := ioutil.ReadFile(buildInfoPath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(buildInfoBytes, &buildInfo)
	if err != nil {
		return nil, err
	}

	if logs {
		log, err := ReadJobLogs(path)
		if err != nil {
			return nil, err
		}
		buildInfo.Log = string(log)
	}

	return buildInfo, nil
}
