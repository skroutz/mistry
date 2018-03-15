package main

import (
	"fmt"
	"os"
	"path/filepath"
	"log"

	_ "github.com/docker/distribution"
	docker "github.com/docker/docker/client"
)

func Work(j *Job) error {
	_, err := os.Stat(j.PendingBuildPath)
	if err == nil {
		// IMPLEMENTME
		panic("build in progress. block until ready and return the result")
	} else if !os.IsNotExist(err) {
		log.Fatal(err)
	}

	_, err = os.Stat(j.ReadyBuildPath)
	if err == nil {
		// IMPLEMENTME
		fmt.Println(j.ReadyBuildPath)
		fmt.Println(err)
		panic("build ready. give back result")
	} else if !os.IsNotExist(err) {
		log.Fatal(err)
	}


	fmt.Println("boostrapping")
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
			err = EnsureDirExists(filepath.Join(j.PendingBuildPath, DataDir))
			if err != nil {
				return err
			}
			err = EnsureDirExists(filepath.Join(j.PendingBuildPath, DataDir, CacheDir))
			if err != nil {
				return err
			}
			err = EnsureDirExists(filepath.Join(j.PendingBuildPath, DataDir, ArtifactsDir))
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

	fmt.Println("building image")
	err = j.BuildImage(client)
	if err != nil {
		return err
	}

	fmt.Println("starting container")
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
