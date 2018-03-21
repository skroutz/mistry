package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
)

// TODO: accept config as flag
var (
	// TODO: control this through CLI args
	fs     = PlainFS{}
	params = make(map[string]string)
)

func TestWorkSuccess(t *testing.T) {
	//	j, err := NewJob("hello-world", params, "")
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//
	//	res, err := Work(context.TODO(), j, fs)
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//
	//	fmt.Println(res)
}

// test result cache
// test pending build blocking for finish
// test a successful build

func TestBuildCache(t *testing.T) {
	params := map[string]string{"foo": "bar"}
	group := "baz"

	j, err := NewJob("hello-world", params, group)
	if err != nil {
		t.Fatal(err)
	}
	path1, err := Work(context.TODO(), j, fs)
	if err != nil {
		t.Fatal(err)
	}

	params["foo"] = "bar2"
	j, err = NewJob("hello-world", params, group)
	if err != nil {
		t.Fatal(err)
	}
	path2, err := Work(context.TODO(), j, fs)
	if err != nil {
		t.Fatal(err)
	}

	if path1 == path2 {
		t.Fatalf("Expected %s and %s to be different", path1, path2)
	}

	res, err := readResult(j, ArtifactsDir, "foo.txt")
	if err != nil {
		t.Fatal(err)
	}

	// CHECK RES AND PATH
	fmt.Println(res)
}

func readResult(j *Job, path, name string) ([]byte, error) {
	out, err := ioutil.ReadFile(filepath.Join(j.ReadyDataPath, path, name))
	if err != nil {
		return nil, err
	}
	return out, nil
}
