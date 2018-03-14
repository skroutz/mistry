package main

import (
	"os"
	"path/filepath"

	_ "github.com/docker/distribution"
	docker "github.com/docker/docker/client"
)

func Work(j *Job) error {
	_, err := os.Stat(j.ReadyBuildPath)
	if err != nil {
		// IMPLEMENTME
		// already built. Just give back the result
	}

	_, err = os.Stat(j.PendingBuildPath)
	if err != nil {
		// IMPLEMENTME
		// already in progress. Block until ready and return the result.
	}

	err = BootstrapProject(j)
	if err != nil {
		return err
	}

	src, err := filepath.EvalSymlinks(j.LatestBuildPath)
	if err == nil {
		if j.Group != "" {
			_, err := RunCmd("btrfs", "snapshot", src, j.PendingBuildPath)
			if err != nil {
				return err
			}
		}
	} else {
		if os.IsNotExist(err) {
			_, err := RunCmd("btrfs", "subvolume", "create", j.PendingBuildPath)
			if err != nil {
				return err
			}
			err = EnsureDirExists(filepath.Join(j.PendingBuildPath, "cache"))
			if err != nil {
				return err
			}
			err = EnsureDirExists(filepath.Join(j.PendingBuildPath, "artifacts"))
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	client, err := docker.NewEnvClient()
	if err != nil {
		return err
	}

	err = j.BuildImage(client)
	if err != nil {
		return err
	}

	err = j.StartContainer(client)
	if err != nil {
		return err
	}
	// TODO: we must block until ready

	return nil
}

// BootstrapProject bootstraps j's project if needed. This function is
// idempotent.
func BootstrapProject(j *Job) error {
	path := filepath.Join(cfg.ProjectPath, j.Project)
	err := EnsureDirExists(path)
	if err != nil {
		return err
	}

	err = EnsureDirExists(filepath.Join(path, "pending"))
	if err != nil {
		return err
	}

	err = EnsureDirExists(filepath.Join(path, "ready"))
	if err != nil {
		return err
	}

	if j.Group != "" {
		err = EnsureDirExists(filepath.Join(path, "groups"))
		if err != nil {
			return err
		}
	}

	return nil
}
