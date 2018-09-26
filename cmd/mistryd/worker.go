package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/docker/distribution"
	docker "github.com/docker/docker/client"
	"github.com/skroutz/mistry/pkg/types"
	"github.com/skroutz/mistry/pkg/utils"
)

// Work performs the work denoted by j and returns a BuildInfo upon
// successful completion, or an error.
func (s *Server) Work(ctx context.Context, j *Job) (buildInfo *types.BuildInfo, err error) {
	log := log.New(os.Stderr, fmt.Sprintf("[worker] [%s] ", j), log.LstdFlags)
	start := time.Now()

	// result cache
	_, err = os.Stat(j.ReadyBuildPath)
	if err == nil {
		buildInfo, err := ReadJobBuildInfo(j.ReadyBuildPath, true)
		if err != nil {
			return nil, err
		} else if buildInfo.ExitCode != 0 {
			// previous build failed, remove its build dir to
			// restart it
			// TODO: if it's a latest link?
			err = s.cfg.FileSystem.Remove(j.ReadyBuildPath)
			if err != nil {
				return buildInfo, workErr("could not remove existing failed build", err)
			}
		} else { // if a successful result exists already, return it
			buildInfo.Cached = true
			return buildInfo, err
		}
	} else if !os.IsNotExist(err) {
		err = workErr("could not check for ready path", err)
		return
	}

	buildInfo = types.NewBuildInfo()
	j.BuildInfo = buildInfo
	j.BuildInfo.Path = filepath.Join(j.ReadyBuildPath, DataDir, ArtifactsDir)
	j.BuildInfo.TransportMethod = types.Rsync
	j.BuildInfo.Params = j.Params
	j.BuildInfo.StartedAt = j.StartedAt
	j.BuildInfo.URL = getJobURL(j)
	j.BuildInfo.Group = j.Group

	added := s.jq.Add(j)
	if added {
		defer s.jq.Delete(j)
	} else { // build coalescing
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		log.Printf("Coalescing with %s...", j.PendingBuildPath)
		for {
			select {
			case <-ctx.Done():
				err = workErr("context cancelled while waiting for pending build", nil)
				return
			case <-t.C:
				_, err = os.Stat(j.ReadyBuildPath)
				if err == nil {
					i, err := ExitCode(j)
					if err != nil {
						return j.BuildInfo, err
					}
					j.BuildInfo.ExitCode = i
					j.BuildInfo.Coalesced = true
					return j.BuildInfo, err
				}

				if os.IsNotExist(err) {
					continue
				} else {
					err = workErr("could not wait for ready build", err)
					return
				}
			}
		}
	}

	_, err = os.Stat(filepath.Join(s.cfg.ProjectsPath, j.Project))
	if err != nil {
		if os.IsNotExist(err) {
			err = workErr("Unknown project", nil)
			return
		}
		err = workErr("could not check for project", err)
		return
	}

	err = s.BootstrapProject(j)
	if err != nil {
		err = workErr("could not bootstrap project", err)
		return
	}

	err = j.BootstrapBuildDir(s.cfg.FileSystem)
	if err != nil {
		err = workErr("could not bootstrap build dir", err)
	}

	// moves the pending directory to the ready one
	defer func() {
		rerr := os.Rename(j.PendingBuildPath, j.ReadyBuildPath)
		if rerr != nil {
			errstr := "could not move pending path"
			if err == nil {
				err = fmt.Errorf("%s; %s", errstr, rerr)
			} else {
				err = fmt.Errorf("%s; %s | %s", errstr, rerr, err)
			}
		}

		// When there are no errors the LatestBuildPath can be updated
		// to point to the last build. Otherwise it remains unchanged.
		if err == nil {
			_, err = os.Lstat(j.LatestBuildPath)
			if err == nil {
				err = os.Remove(j.LatestBuildPath)
				if err != nil {
					err = workErr("could not remove latest build link", err)
					return
				}
			} else if !os.IsNotExist(err) {
				err = workErr("could not stat the latest build link", err)
				return
			}
			err = os.Symlink(j.ReadyBuildPath, j.LatestBuildPath)
			if err != nil {
				err = workErr("could not create latest build link", err)
			}
		}
	}()

	for k, v := range j.Params {
		err = ioutil.WriteFile(filepath.Join(j.PendingBuildPath, DataDir, ParamsDir, k), []byte(v), 0644)
		if err != nil {
			err = workErr("could not write param file", err)
			return
		}
	}

	out, err := os.Create(j.BuildLogPath)
	if err != nil {
		err = workErr("could not create build log file", err)
		return
	}
	defer func() {
		ferr := out.Close()
		errstr := "could not close build log file"
		if ferr != nil {
			if err == nil {
				err = fmt.Errorf("%s; %s", errstr, ferr)
			} else {
				err = fmt.Errorf("%s; %s | %s", errstr, ferr, err)
			}
		}
	}()

	biJSON, err := json.Marshal(j.BuildInfo)
	if err != nil {
		err = workErr("could not serialize build info", err)
		return
	}
	err = ioutil.WriteFile(j.BuildInfoFilePath, biJSON, 0666)
	if err != nil {
		err = workErr("could not write build info to file", err)
		return
	}

	client, err := docker.NewEnvClient()
	if err != nil {
		err = workErr("could not create docker client", err)
		return
	}
	defer func() {
		derr := client.Close()
		errstr := "could not close docker client"
		if derr != nil {
			if err == nil {
				err = fmt.Errorf("%s; %s", errstr, derr)
			} else {
				err = fmt.Errorf("%s; %s | %s", errstr, derr, err)
			}
		}
	}()

	err = j.BuildImage(ctx, s.cfg.UID, client, out, j.Rebuild, j.Rebuild)
	if err != nil {
		err = workErr("could not build docker image", err)
		return
	}

	var outErr strings.Builder
	j.BuildInfo.ExitCode, err = j.StartContainer(ctx, s.cfg, client, out, &outErr)
	if err != nil {
		err = workErr("could not start docker container", err)
		return
	}

	// fill the buildInfo Log from the freshly written log
	err = out.Sync()
	if err != nil {
		err = workErr("could not flush the output log", err)
		return
	}

	finalLog, err := ReadJobLogs(j.PendingBuildPath)
	if err != nil {
		err = workErr("could not read the job logs", err)
		return
	}

	j.BuildInfo.Log = string(finalLog)
	j.BuildInfo.ErrLog = outErr.String()
	j.BuildInfo.Duration = time.Now().Sub(start).Truncate(time.Millisecond)

	biJSON, err = json.Marshal(j.BuildInfo)
	if err != nil {
		err = workErr("could not serialize build info", err)
		return
	}
	err = ioutil.WriteFile(j.BuildInfoFilePath, biJSON, 0666)
	if err != nil {
		err = workErr("could not write build info to file", err)
		return
	}

	log.Println("Finished after", j.BuildInfo.Duration)
	return
}

// BootstrapProject bootstraps j's project if needed. BootstrapProject is
// idempotent.
func (s *Server) BootstrapProject(j *Job) error {
	s.pq.Lock(j.Project)
	defer s.pq.Unlock(j.Project)

	err := utils.EnsureDirExists(j.RootBuildPath)
	if err != nil {
		return err
	}

	err = utils.EnsureDirExists(filepath.Join(j.RootBuildPath, "pending"))
	if err != nil {
		return err
	}

	err = utils.EnsureDirExists(filepath.Join(j.RootBuildPath, "ready"))
	if err != nil {
		return err
	}

	if j.Group != "" {
		err = utils.EnsureDirExists(filepath.Join(j.RootBuildPath, "groups"))
		if err != nil {
			return err
		}
	}

	return nil
}

// ExitCode returns the exit code of the job's container build.
// If an error is returned, the exit code is irrelevant.
func ExitCode(j *Job) (int, error) {
	buildInfo, err := ReadJobBuildInfo(j.ReadyBuildPath, false)
	if err != nil {
		return types.ContainerFailureExitCode, err
	}
	return buildInfo.ExitCode, nil
}

func workErr(s string, e error) error {
	s = "work: " + s
	if e != nil {
		s += "; " + e.Error()
	}
	return errors.New(s)
}
