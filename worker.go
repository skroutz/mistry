package main

import (
	"os"
	"path/filepath"

	_ "github.com/docker/distribution"
	docker "github.com/docker/docker/client"
)

// x check for in-progress or comleted build
// x bootstrap project
// x btrfs snapshot/create
// - build image
// - create container
// - start container
func Work(j *Job) error {
	_, err := os.Stat(j.ReadyBuildPath)
	if err != nil {
		// already built. Just give back the result
	}

	_, err = os.Stat(j.PendingBuildPath)
	if err != nil {
		// already in progress. Block until ready and return the result.
	}

	err = BootstrapProject(j)
	if err != nil {
		return err
	}

	//	src, err := filepath.EvalSymlinks(j.LatestBuildPath())
	//	if err == nil {
	//		if j.Group != "" {
	//			_, err := RunCmd("btrfs", "snapshot", src, j.PendingBuildPath)
	//			if err != nil {
	//				return err
	//			}
	//		}
	//	} else {
	//		if os.IsNotExist(err) {
	//			_, err := RunCmd("btrfs", "subvolume", "create", j.PendingBuildPath)
	//			if err != nil {
	//				return err
	//			}
	//			err = EnsureDirExists(filepath.Join(j.PendingBuildPath, "cache"))
	//			if err != nil {
	//				return err
	//			}
	//			err = EnsureDirExists(filepath.Join(j.PendingBuildPath, "artifacts"))
	//			if err != nil {
	//				return err
	//			}
	//		} else {
	//			return err
	//		}
	//	}

	client, err := docker.NewEnvClient()
	if err != nil {
		return err
	}

	err = j.BuildImage(client)
	if err != nil {
		return err
	}
	//	config := container.Config{User: "502", Image: j.Project}
	//
	//	mnts := []mount.Mount{
	//		{Type: mount.TypeBind, Source: "/tmp", Target: "/tmp"},
	//	}
	//	hostConfig := container.HostConfig{Mounts: mnts}
	//
	//	container, err := cli.ContainerCreate(context.Background(), &config, &hostConfig, nil, "rzec")
	//	if err != nil {
	//		return err
	//	}
	//fmt.Printf("%#v", container)

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
