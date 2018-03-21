package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

// TODO: accept config as flag

var (
	// TODO: control this through CLI args
	fs     = PlainFS{}
	params = make(map[string]string)
)

func init() {
	path, err := os.Open(cfg.BuildPath)
	if err != nil {
		log.Fatalf("could not check build dir; %s", err)
	}
	defer path.Close()

	n, err := path.Readdirnames(1)
	if err != nil && err != io.EOF {
		log.Fatalf("could not check build dir; %s", err)
	}

	if len(n) != 0 {
		log.Fatalf("build dir not empty; %s", n)
	}
}

func TestSampleBuild(t *testing.T) {
	j, err := NewJob("simple", params, "")
	if err != nil {
		t.Fatal(err)
	}

	res, err := Work(context.TODO(), j, fs)
	if err != nil {
		t.Fatal(err)
	}

	assert(res.ExitCode, 0, t)
}

func TestConcurrentJobs(t *testing.T) {
	t.Skip("TODO: fix races")
	var wg sync.WaitGroup
	results := make(chan *BuildResult, 100)

	jobs := []struct {
		project string
		params  map[string]string
		group   string
	}{
		{"concurrent", map[string]string{"foo": "bar"}, ""},
		{"concurrent", map[string]string{"foo": "bar"}, ""},
		{"concurrent2", map[string]string{"foo": "bar"}, "foo"},
		{"concurrent", map[string]string{"foo": "baz"}, ""},
		{"concurrent", map[string]string{"foo": "abc"}, "abc"},
		{"concurrent2", map[string]string{"foo": "bar"}, "foo"},
		{"concurrent", map[string]string{"foo": "abc"}, "bca"},
		{"concurrent2", map[string]string{"foo": "bar"}, "foo"},
		{"concurrent", map[string]string{"foo": "abc"}, "abc"},
		{"concurrent", map[string]string{"foo": "abc"}, ""},
		{"concurrent2", map[string]string{"foo": "bar"}, "foo"},
		{"concurrent", map[string]string{"foo": "1"}, ""},
		{"concurrent", map[string]string{"foo": "2"}, ""},
		{"concurrent2", map[string]string{"foo": "bar"}, "foo"},
		{"concurrent", map[string]string{}, ""},
		{"concurrent2", map[string]string{"foo": "bar"}, "foo"},
		{"concurrent", map[string]string{}, ""},
		{"concurrent", map[string]string{}, ""},
		{"concurrent", map[string]string{"foo": "bar"}, "same"},
		{"concurrent2", map[string]string{"foo": "bar"}, "foo"},
		{"concurrent", map[string]string{"foo": "bar"}, "same"},
		{"concurrent2", map[string]string{"foo": "bar"}, "foo"},
		{"concurrent2", map[string]string{"foo": "bar"}, "foo"},
		{"concurrent2", map[string]string{"foo": "bar"}, "bar"},
		{"concurrent2", map[string]string{"foo": "bar"}, "bar"},
		{"concurrent2", map[string]string{"foo": "bar"}, ""},
		{"concurrent2", map[string]string{"foo": "bar"}, ""},
	}

	for _, j := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j, err := NewJob(j.project, j.params, j.group)
			if err != nil {
				log.Fatal(err)
			}
			res, err := Work(context.TODO(), j, fs)
			if err != nil {
				log.Fatal(err)
			}
			results <- res
		}()
	}

	for i := 0; i < len(jobs); i++ {
		res := <-results
		fmt.Printf("%#v\n", res)
	}

	wg.Wait()
}

func TestBuildCoalescing(t *testing.T) {
	var result1, result2 *BuildResult
	var wg sync.WaitGroup

	j1, err := NewJob("build-coalescing", make(map[string]string), "foo")
	if err != nil {
		t.Fatal(err)
	}

	j2, err := NewJob("build-coalescing", make(map[string]string), "foo")
	if err != nil {
		t.Fatal(err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		result1, err = Work(context.TODO(), j1, PlainFS{})
		if err != nil {
			log.Fatal(err)
		}
	}()

	result2, err = Work(context.TODO(), j2, PlainFS{})
	if err != nil {
		t.Fatal(err)
	}

	wg.Wait()

	if result1.Coalesced == result2.Coalesced {
		t.Fatalf("Expected exactly one of both builds to be coalesced, both were %v", result1.Coalesced)
	}

	out, err := readOut(j2, ArtifactsDir)
	if err != nil {
		t.Fatal(err)
	}

	assertEq(out, "coalescing!\n", t)
	assert(result1.ExitCode, 0, t)
	assert(result2.ExitCode, 0, t)
}

func TestExitCode(t *testing.T) {
	j, err := NewJob("exit-code", params, "")
	if err != nil {
		t.Fatal(err)
	}

	result, err := Work(context.TODO(), j, fs)
	if err != nil {
		t.Fatal(err)
	}

	assert(result.ExitCode, 77, t)
}

func TestResultCache(t *testing.T) {
	params := make(map[string]string)

	j, err := NewJob("result-cache", params, "")
	if err != nil {
		t.Fatal(err)
	}

	result1, err := Work(context.TODO(), j, fs)
	if err != nil {
		t.Fatal(err)
	}

	out1, err := readOut(j, ArtifactsDir)
	if err != nil {
		t.Fatal(err)
	}

	result2, err := Work(context.TODO(), j, fs)
	if err != nil {
		t.Fatal(err)
	}

	out2, err := readOut(j, ArtifactsDir)
	if err != nil {
		t.Fatal(err)
	}

	assertEq(out1, out2, t)
	assert(result1.Cached, false, t)
	assert(result2.Cached, true, t)
	assert(result1.ExitCode, 0, t)
	assert(result2.ExitCode, 0, t)
}

func TestBuildCache(t *testing.T) {
	params := map[string]string{"foo": "bar"}
	group := "baz"

	j, err := NewJob("build-cache", params, group)
	if err != nil {
		t.Fatal(err)
	}

	result1, err := Work(context.TODO(), j, fs)
	if err != nil {
		t.Fatal(err)
	}

	out1, err := readOut(j, ArtifactsDir)
	if err != nil {
		t.Fatal(err)
	}

	cachedOut1, err := readOut(j, CacheDir)
	if err != nil {
		t.Fatal(err)
	}

	assertEq(out1, cachedOut1, t)

	params["foo"] = "bar2"
	j, err = NewJob("build-cache", params, group)
	if err != nil {
		t.Fatal(err)
	}

	result2, err := Work(context.TODO(), j, fs)
	if err != nil {
		t.Fatal(err)
	}

	out2, err := readOut(j, ArtifactsDir)
	if err != nil {
		t.Fatal(err)
	}

	cachedOut2, err := readOut(j, CacheDir)
	if err != nil {
		t.Fatal(err)
	}

	assertEq(cachedOut1, cachedOut2, t)
	assertNotEq(out1, out2, t)
	assertNotEq(result1.Path, result2.Path, t)
	assert(result1.ExitCode, 0, t)
	assert(result2.ExitCode, 0, t)
}

func readOut(j *Job, path string) (string, error) {
	out, err := ioutil.ReadFile(filepath.Join(j.ReadyDataPath, path, "out.txt"))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func assertEq(a, b interface{}, t *testing.T) {
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("Expected %#v and %#v to be equal", a, b)
	}
}

func assert(act, exp interface{}, t *testing.T) {
	if !reflect.DeepEqual(act, exp) {
		t.Fatalf("Expected %#v to be %#v", act, exp)
	}
}

func assertNotEq(a, b interface{}, t *testing.T) {
	if reflect.DeepEqual(a, b) {
		t.Fatalf("Expected %#v and %#v to not be equal", a, b)
	}
}
