package main

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	_ "github.com/docker/distribution"
	docker "github.com/docker/docker/client"
)

// Work performs the work denoted by j and returns the result path upon
// successful completion.
//
// TODO: introduce build type
func Work(ctx context.Context, j *Job) (string, error) {
	// TODO: this is racy; do this using a map and a mutex
	_, err := os.Stat(j.PendingBuildPath)
	if err == nil {
		t := time.NewTicker(1 * time.Second)
		select {
		case <-ctx.Done():
			return "", errors.New("work: context cancelled while waiting for pending build")
		case <-t.C:
			_, err = os.Stat(j.ReadyBuildPath)
			if err == nil {
				return j.ReadyBuildPath, nil
			} else if !os.IsNotExist(err) {
				return "", errors.New("work: could not wait for ready build; " + err.Error())
			}
		}
	} else if !os.IsNotExist(err) {
		return "", errors.New("work: could not check for pending build; " + err.Error())
	}

	_, err = os.Stat(j.ReadyBuildPath)
	if err == nil {
		return j.ReadyBuildPath, nil
	} else if !os.IsNotExist(err) {
		return "", errors.New("work: could not check for ready path; " + err.Error())
	}

	err = BootstrapProject(j)
	if err != nil {
		return "", errors.New("work: could not bootstrap project; " + err.Error())
	}

	src, err := filepath.EvalSymlinks(j.LatestBuildPath)
	if err == nil {
		if j.Group != "" {
			_, err := RunCmd("btrfs", "subvolume", "snapshot", src, j.PendingBuildPath)
			if err != nil {
				return "", errors.New("work: could not snapshot subvolume; " + err.Error())
			}
			err = os.RemoveAll(filepath.Join(j.PendingBuildPath, DataDir, ParamsDir))
			if err != nil {
				return "", errors.New("work: could not remove params dir; " + err.Error())
			}
			err = EnsureDirExists(filepath.Join(j.PendingBuildPath, DataDir, ParamsDir))
			if err != nil {
				return "", errors.New("work: could not ensure directory exists; " + err.Error())
			}
		}
	} else if os.IsNotExist(err) {
		_, err := RunCmd("btrfs", "subvolume", "create", j.PendingBuildPath)
		if err != nil {
			return "", errors.New("work: could not create subvolume; " + err.Error())
		}
		err = EnsureDirExists(filepath.Join(j.PendingBuildPath, DataDir))
		if err != nil {
			return "", errors.New("work: could not ensure directory exists; " + err.Error())
		}
		err = EnsureDirExists(filepath.Join(j.PendingBuildPath, DataDir, CacheDir))
		if err != nil {
			return "", errors.New("work: could not ensure directory exists; " + err.Error())
		}
		err = EnsureDirExists(filepath.Join(j.PendingBuildPath, DataDir, ArtifactsDir))
		if err != nil {
			return "", errors.New("work: could not ensure directory exists; " + err.Error())
		}
		err = EnsureDirExists(filepath.Join(j.PendingBuildPath, DataDir, ParamsDir))
		if err != nil {
			return "", errors.New("work: could not ensure directory exists; " + err.Error())
		}
	} else {
		return "", errors.New("work: could not read latest build link; " + err.Error())
	}

	for k, v := range j.Params {
		err = ioutil.WriteFile(filepath.Join(j.PendingBuildPath, DataDir, ParamsDir, k), []byte(v), 0644)
		if err != nil {
			return "", errors.New("work: could not write param file; " + err.Error())
		}
	}

	out, err := os.Create(j.BuildLogPath)
	if err != nil {
		return "", errors.New("work: could not create build log file; " + err.Error())
	}

	// TODO: we should check the error here. However, it's not so simple
	// cause we must always close the file even if eg. BuildImage() failed
	defer out.Close()

	client, err := docker.NewEnvClient()
	if err != nil {
		return "", errors.New("work: could not create docker client; " + err.Error())
	}

	err = j.BuildImage(ctx, client, out)
	if err != nil {
		return "", errors.New("work: could not build docker image; " + err.Error())
	}

	err = j.StartContainer(ctx, client, out)
	if err != nil {
		return "", errors.New("work: could not start docker container; " + err.Error())
	}

	err = os.Rename(j.PendingBuildPath, j.ReadyBuildPath)
	if err != nil {
		return "", errors.New("work: could not rename pending to ready path; " + err.Error())
	}

	_, err = os.Lstat(j.LatestBuildPath)
	if err == nil {
		err = os.Remove(j.LatestBuildPath)
		if err != nil {
			return "", errors.New("work: could not remove latest build link; " + err.Error())
		}
	}

	err = os.Symlink(j.ReadyBuildPath, j.LatestBuildPath)
	if err != nil {
		return "", errors.New("work: could not create latest build link;" + err.Error())
	}

	return j.ReadyBuildPath, nil
}

// BootstrapProject bootstraps j's project if needed. This function is
// idempotent.
func BootstrapProject(j *Job) error {
	err := EnsureDirExists(j.RootBuildPath)
	if err != nil {
		return err
	}

	err = EnsureDirExists(filepath.Join(j.RootBuildPath, "pending"))
	if err != nil {
		return err
	}

	err = EnsureDirExists(filepath.Join(j.RootBuildPath, "ready"))
	if err != nil {
		return err
	}

	if j.Group != "" {
		err = EnsureDirExists(filepath.Join(j.RootBuildPath, "groups"))
		if err != nil {
			return err
		}
	}

	return nil
}
