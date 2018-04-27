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

// Work performs the work denoted by j and returns a BuildInfo upon
// successful completion, or an error.
func (s *Server) Work(ctx context.Context, j *Job) (buildInfo *types.BuildInfo, err error) {
	log := log.New(os.Stderr, fmt.Sprintf("[worker] [%s] ", j), log.LstdFlags)
	start := time.Now()
	buildInfo = &types.BuildInfo{
		Path:            filepath.Join(j.ReadyBuildPath, DataDir, ArtifactsDir),
		TransportMethod: types.Rsync,
		Params:          j.Params,
		StartedAt:       j.StartedAt,
	}

	_, err = os.Stat(j.ReadyBuildPath)
	if err == nil {
		i, err := ExitCode(j)
		if err != nil {
			return buildInfo, err
		}
		if i != 0 {
			// previous build failed, remove its build dir to restart it
			err = j.RemoveBuildDir(s.cfg.FileSystem, log)
			if err != nil {
				return buildInfo, workErr("could not remove existing failed build", err)
			}
		} else {
			buildInfo.Cached = true
			buildInfo.ExitCode = i
			return buildInfo, err
		}
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
						return buildInfo, err
					}
					buildInfo.ExitCode = i
					buildInfo.Coalesced = true
					return buildInfo, err
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
	cleanupPending, err := j.BootstrapBuildDir(s.cfg.FileSystem, log)

	defer func() {
		if cleanupPending {
			derr := s.cfg.FileSystem.Remove(j.PendingBuildPath)
			if derr != nil {
				errstr := "could not clean hanging pending path"
				if err == nil {
					err = fmt.Errorf("%s; %s", errstr, derr)
				} else {
					err = fmt.Errorf("%s; %s | %s", errstr, derr, err)
				}
			}
		}
	}()

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

	infoFile, err := os.Create(j.BuildInfoFilePath)
	if err != nil {
		err = workErr("could not create build info file", err)
		return
	}
	defer func() {
		ferr := infoFile.Close()
		errstr := "could not close build info file"
		if ferr != nil {
			if err == nil {
				err = fmt.Errorf("%s; %s", errstr, ferr)
			} else {
				err = fmt.Errorf("%s; %s | %s", errstr, ferr, err)
			}
		}
	}()
	biJSON, err := json.Marshal(buildInfo)
	if err != nil {
		err = workErr("could not serialize build info", err)
		return
	}
	_, err = infoFile.Write(biJSON)
	if err != nil {
		err = workErr("could not write build info to file", err)
		return
	}

	client, err := docker.NewEnvClient()
	if err != nil {
		err = workErr("could not create docker client", err)
		return
	}

	err = j.BuildImage(ctx, s.cfg.UID, client, out)
	if err != nil {
		err = workErr("could not build docker image", err)
		return
	}

	buildInfo.ExitCode, err = j.StartContainer(ctx, s.cfg, client, out)
	if err != nil {
		err = workErr("could not start docker container", err)
		return
	}

	_, err = infoFile.Seek(0, 0)
	if err != nil {
		err = workErr("could not set the offset for the build_info file", err)
		return
	}
	biJSON, err = json.Marshal(buildInfo)
	if err != nil {
		err = workErr("could not serialize build info", err)
		return
	}
	_, err = infoFile.Write(biJSON)
	if err != nil {
		err = workErr("could not write build info to file", err)
		return
	}

	err = os.Rename(j.PendingBuildPath, j.ReadyBuildPath)
	if err != nil {
		err = workErr("could not rename pending to ready path", err)
		return
	}
	// after moving the pending to ready there's nothing to cleanup
	cleanupPending = false

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
	br := new(types.BuildInfo)
	f, err := os.Open(filepath.Join(j.ReadyBuildPath, BuildInfoFname))
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
