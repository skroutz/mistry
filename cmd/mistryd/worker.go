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
	"time"

	_ "github.com/docker/distribution"
	docker "github.com/docker/docker/client"
	"github.com/skroutz/mistry/pkg/types"
	"github.com/skroutz/mistry/pkg/utils"
)

// Work performs the work denoted by j and returns a BuildResult upon
// successful completion, or an error.
func (s *Server) Work(ctx context.Context, j *Job) (buildResult *types.BuildResult, err error) {
	log := log.New(os.Stderr, fmt.Sprintf("[worker] [%s] ", j), log.LstdFlags)
	start := time.Now()
	buildResult = &types.BuildResult{
		Path:            filepath.Join(j.ReadyBuildPath, DataDir, ArtifactsDir),
		TransportMethod: types.Rsync,
		Params:          j.Params,
	}

	_, err = os.Stat(j.ReadyBuildPath)
	if err == nil {
		i, err := ExitCode(j)
		if err != nil {
			return buildResult, err
		}
		buildResult.Cached = true
		buildResult.ExitCode = i
		return buildResult, err
	} else if !os.IsNotExist(err) {
		err = workErr("could not check for ready path", err)
		return
	}

	added := s.jq.Add(j)
	if added {
		defer s.jq.Delete(j)
	} else {
		t := time.NewTicker(2 * time.Second)
		log.Printf("Waiting for %s to complete...", j.PendingBuildPath)
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
						return buildResult, err
					}
					buildResult.ExitCode = i
					buildResult.Coalesced = true
					return buildResult, err
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

	log.Printf("Creating new build directory...")
	shouldCleanup, err := j.BootstrapBuildDir(s.cfg.FileSystem, log)
	if shouldCleanup {
		defer func() {
			derr := s.cfg.FileSystem.Remove(j.PendingBuildPath)
			if derr != nil {
				errstr := "could not clean hanging pending path"
				if err == nil {
					err = fmt.Errorf("%s; %s", errstr, derr)
				} else {
					err = fmt.Errorf("%s; %s | %s", errstr, derr, err)
				}
			}
		}()
	}
	if err != nil {
		return
	}

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

	client, err := docker.NewEnvClient()
	if err != nil {
		err = workErr("could not create docker client", err)
		return
	}

	err = j.BuildImage(ctx, s.cfg.UID, client, out)
	if err != nil {
		return
	}

	buildResult.ExitCode, err = j.StartContainer(ctx, s.cfg, client, out)
	if err != nil {
		err = workErr("could not start docker container", err)
		return
	}

	resultFile, err := os.Create(j.BuildResultFilePath)
	if err != nil {
		err = workErr("could not create build result file", err)
		return
	}
	defer func() {
		ferr := resultFile.Close()
		errstr := "could not close build result file"
		if ferr != nil {
			if err == nil {
				err = fmt.Errorf("%s; %s", errstr, ferr)
			} else {
				err = fmt.Errorf("%s; %s | %s", errstr, ferr, err)
			}
		}
	}()
	brJSON, err := json.Marshal(buildResult)
	if err != nil {
		err = workErr("could not serialize build result", err)
		return
	}
	_, err = resultFile.Write(brJSON)
	if err != nil {
		err = workErr("could not write build result to file", err)
		return
	}

	err = os.Rename(j.PendingBuildPath, j.ReadyBuildPath)
	if err != nil {
		err = workErr("could not rename pending to ready path", err)
		return
	}

	_, err = os.Lstat(j.LatestBuildPath)
	if err == nil {
		err = os.Remove(j.LatestBuildPath)
		if err != nil {
			err = workErr("could not remove latest build link", err)
			return
		}
	}

	err = os.Symlink(j.ReadyBuildPath, j.LatestBuildPath)
	if err != nil {
		err = workErr("could not create latest build link", err)
		return
	}

	log.Println("Finished after", time.Now().Sub(start).Truncate(time.Millisecond))
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
	br := new(types.BuildResult)
	f, err := os.Open(filepath.Join(j.ReadyBuildPath, BuildResultFname))
	if err != nil {
		return -999, err
	}
	dec := json.NewDecoder(f)
	err = dec.Decode(br)
	if err != nil {
		return -999, err
	}
	return br.ExitCode, nil
}

func workErr(s string, e error) error {
	s = "work: " + s
	if e != nil {
		s += "; " + e.Error()
	}
	return errors.New(s)
}
