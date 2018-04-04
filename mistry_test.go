package main

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/skroutz/mistry/utils"
)

func TestPruneZombieBuilds(t *testing.T) {
	project := "hanging-pending"
	cmdOut, err := cliBuildJob("--project", project)
	if err != nil {
		t.Fatalf("Error output: %s, err: %v", cmdOut, err)
	}
	path := filepath.Join(cfg.BuildPath, project, "pending")
	_, err = utils.RunCmd(cfg.FileSystem.Create(filepath.Join(path, "foo")))
	if err != nil {
		t.Fatal(err)
	}
	_, err = utils.RunCmd(cfg.FileSystem.Create(filepath.Join(path, "bar")))
	if err != nil {
		t.Fatal(err)
	}

	err = PruneZombieBuilds(cfg)
	if err != nil {
		t.Fatal(err)
	}

	hangingPendingBuilds, err := ioutil.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hangingPendingBuilds) != 0 {
		t.Fatalf("Expected to have cleaned up all zombie pending builds, found %d", len(hangingPendingBuilds))
	}
}
