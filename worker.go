package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	_ "github.com/docker/distribution"
	docker "github.com/docker/docker/client"
	"github.com/skroutz/mistry/types"
	"github.com/skroutz/mistry/utils"
)

// Work performs the work denoted by j and returns the BuildResult upon
// successful completion.
//
// TODO: return error on unsuccessful command
// TODO: introduce build type
// TODO: log fs command outputs
// TODO: logs
// TODO: set BuildResult type correctly
func Work(ctx context.Context, j *Job, fs FileSystem) (buildResult *types.BuildResult, err error) {
	buildResult = &types.BuildResult{Path: filepath.Join(j.ReadyBuildPath, DataDir, ArtifactsDir), Type: "rsync"}

	_, err = os.Stat(j.ReadyBuildPath)
	if err == nil {
		cachedResult := new(types.BuildResult)
		f, err := os.Open(filepath.Join(j.ReadyBuildPath, BuildResultFname))
		if err != nil {
			return buildResult, err
		}
		dec := json.NewDecoder(f)
		dec.Decode(cachedResult)
		buildResult.Cached = true
		buildResult.ExitCode = cachedResult.ExitCode
		return buildResult, err
	} else if !os.IsNotExist(err) {
		err = workErr("could not check for ready path", err)
		return
	}

	added := jobs.Add(j)
	if added {
		defer jobs.Delete(j)
	} else {
		t := time.NewTicker(1 * time.Second)
		fmt.Printf("Waiting for %s to complete\n", j.PendingBuildPath)
		for {
			select {
			case <-ctx.Done():
				err = workErr("context cancelled while waiting for pending build", nil)
				return
			case <-t.C:
				_, err = os.Stat(j.ReadyBuildPath)
				if err == nil {
					buildResult.Coalesced = true
					return
				} else {
					if os.IsNotExist(err) {
						continue
					} else {
						err = workErr("could not wait for ready build", err)
						return
					}
				}
			}
		}
	}

	_, err = os.Stat(filepath.Join(cfg.ProjectsPath, j.Project))
	if err != nil {
		if os.IsNotExist(err) {
			err = workErr("Unknown project", nil)
			return
		} else {
			err = workErr("could not check for project", err)
			return
		}
	}

	err = BootstrapProject(j)
	if err != nil {
		err = workErr("could not bootstrap project", err)
		return
	}

	src, err := filepath.EvalSymlinks(j.LatestBuildPath)
	if err == nil {
		if j.Group != "" {
			out, err := utils.RunCmd(fs.Clone(src, j.PendingBuildPath))
			// TODO: log out only if there is any
			fmt.Println(out)
			if err != nil {
				err = workErr("could not clone latest build result", err)
				return buildResult, err
			}
			defer func() {
				derr := fs.Remove(j.PendingBuildPath)
				if derr != nil {
					errstr := "could not clean hanging pending path"
					if err == nil {
						err = fmt.Errorf("%s; %s", errstr, derr)
					} else {
						err = fmt.Errorf("%s; %s | %s", errstr, derr, err)
					}
				}
			}()
			err = os.RemoveAll(filepath.Join(j.PendingBuildPath, DataDir, ParamsDir))
			if err != nil {
				err = workErr("could not remove params dir", err)
				return buildResult, err
			}
			err = utils.EnsureDirExists(filepath.Join(j.PendingBuildPath, DataDir, ParamsDir))
			if err != nil {
				err = workErr("could not ensure directory exists", err)
				return buildResult, err
			}
		}
	} else if os.IsNotExist(err) {
		out, err := utils.RunCmd(fs.Create(j.PendingBuildPath))
		// TODO: log out only if there is any
		fmt.Println(out)
		if err != nil {
			err = workErr("could not create pending build path", err)
			return buildResult, err
		}
		defer func() {
			derr := fs.Remove(j.PendingBuildPath)
			if derr != nil {
				errstr := "could not clean hanging pending path"
				if err == nil {
					err = fmt.Errorf("%s; %s", errstr, derr)
				} else {
					err = fmt.Errorf("%s; %s | %s", errstr, derr, err)
				}
			}
		}()
		err = utils.EnsureDirExists(filepath.Join(j.PendingBuildPath, DataDir))
		if err != nil {
			err = workErr("could not ensure directory exists", err)
			return buildResult, err
		}
		err = utils.EnsureDirExists(filepath.Join(j.PendingBuildPath, DataDir, CacheDir))
		if err != nil {
			err = workErr("could not ensure directory exists", err)
			return buildResult, err
		}
		err = utils.EnsureDirExists(filepath.Join(j.PendingBuildPath, DataDir, ArtifactsDir))
		if err != nil {
			err = workErr("could not ensure directory exists", err)
			return buildResult, err
		}
		err = utils.EnsureDirExists(filepath.Join(j.PendingBuildPath, DataDir, ParamsDir))
		if err != nil {
			err = workErr("could not ensure directory exists", err)
			return buildResult, err
		}
	} else {
		err = workErr("could not read latest build link", err)
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

	// TODO: we should check the error here. However, it's not so simple
	// cause we must always close the file even if eg. BuildImage() failed
	defer out.Close()

	client, err := docker.NewEnvClient()
	if err != nil {
		err = workErr("could not create docker client", err)
		return
	}

	err = j.BuildImage(ctx, client, out)
	if err != nil {
		err = workErr("could not build docker image", err)
		return
	}

	buildResult.ExitCode, err = j.StartContainer(ctx, client, out)
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
	brJson, err := json.Marshal(buildResult)
	if err != nil {
		err = workErr("could not serialize build result", err)
		return
	}
	_, err = resultFile.Write(brJson)
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

	return
}

// BootstrapProject bootstraps j's project if needed. This function is
// idempotent.
func BootstrapProject(j *Job) error {
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

func workErr(s string, e error) error {
	s = "work: " + s
	if e != nil {
		s += "; " + e.Error()
	}
	return errors.New(s)
}
