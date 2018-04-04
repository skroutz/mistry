package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/skroutz/mistry/types"
)

func TestJobID(t *testing.T) {
	project := "job-id-seeding"
	params := types.Params{"foo": "bar"}
	group := "zzz"

	j1, err := NewJob(project, params, group, cfg)
	if err != nil {
		t.Fatal(err)
	}

	j2, err := NewJob(project, params, group, cfg)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(j1.ID, j2.ID, t)

	// params seeding
	j3, err := NewJob(project, make(types.Params), group, cfg)
	if err != nil {
		t.Fatal(err)
	}
	assertNotEq(j1.ID, j3.ID, t)

	// group seeding
	j4, err := NewJob(project, params, "c", cfg)
	assertNotEq(j1.ID, j4.ID, t)

	// project seeding (new empty file)
	path := filepath.Join("testdata", "projects", project, "foo")
	os.Remove(path) // in case there's a leftover from a previous run
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	j5, err := NewJob(project, params, group, cfg)
	if err != nil {
		t.Fatal(err)
	}
	assertNotEq(j1.ID, j5.ID, t)

	// project seeding (new non-empty file)
	_, err = f.Write([]byte("foo"))
	if err != nil {
		t.Fatal(err)
	}
	j6, err := NewJob(project, params, group, cfg)
	if err != nil {
		t.Fatal(err)
	}
	assertNotEq(j5.ID, j6.ID, t)
	assertNotEq(j1.ID, j6.ID, t)

	err = f.Close()
	if err != nil {
		t.Fatal(err)
	}
}
