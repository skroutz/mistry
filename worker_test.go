package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

// TODO: remove once tests are converted to end-to-end tests
var server = NewServer("localhost:8462", log.New(os.Stdout, "test", log.Lshortfile))
var params = make(map[string]string)

func init() {
	flag.String("config", "", "")
	flag.String("filesystem", "", "")
}

func TestMain(m *testing.M) {
	var err error

	go func() {
		main()
	}()
	waitForServer("8462")

	cfg.BuildPath, err = ioutil.TempDir("", "mistry-tests")
	if err != nil {
		panic(err)
	}

	cfg.BuildPath, err = filepath.EvalSymlinks(cfg.BuildPath)
	if err != nil {
		panic(err)
	}

	result := m.Run()

	err = os.RemoveAll(cfg.BuildPath)
	if err != nil {
		panic(err)
	}

	os.Exit(result)
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
			res, err := Work(context.TODO(), j, curfs)
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

func TestSimpleBuild(t *testing.T) {
	jr := JobRequest{Project: "simple", Params: params, Group: ""}
	res, err := postJob(jr)
	if err != nil {
		t.Fatal(err)
	}
	assert(res.ExitCode, 0, t)
}

func TestBuildCoalescing(t *testing.T) {
	var result1, result2 *BuildResult
	var wg sync.WaitGroup

	jr := JobRequest{"build-coalescing", params, "foo"}

	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		result1, err = postJob(jr)
		if err != nil {
			log.Fatal(err)
		}
	}()

	var err error
	result2, err = postJob(jr)
	if err != nil {
		t.Fatal(err)
	}

	wg.Wait()

	if result1.Coalesced == result2.Coalesced {
		t.Fatalf("Expected exactly one of both builds to be coalesced, both were %v", result1.Coalesced)
	}

	out, err := readOut(result2, ArtifactsDir)
	if err != nil {
		t.Fatal(err)
	}

	assertEq(out, "coalescing!\n", t)
	assert(result1.ExitCode, 0, t)
	assert(result2.ExitCode, 0, t)
}

func TestExitCode(t *testing.T) {
	result, err := postJob(JobRequest{"exit-code", params, ""})
	if err != nil {
		t.Fatal(err)
	}

	assert(result.ExitCode, 77, t)
}

func TestResultCache(t *testing.T) {
	result1, err := postJob(JobRequest{"result-cache", params, ""})
	if err != nil {
		t.Fatal(err)
	}

	out1, err := readOut(result1, ArtifactsDir)
	if err != nil {
		t.Fatal(err)
	}

	result2, err := postJob(JobRequest{"result-cache", params, ""})
	if err != nil {
		t.Fatal(err)
	}

	out2, err := readOut(result2, ArtifactsDir)
	if err != nil {
		t.Fatal(err)
	}

	assertEq(out1, out2, t)
	assert(result1.Cached, false, t)
	assert(result2.Cached, true, t)
	assert(result1.ExitCode, 0, t)
	assert(result2.ExitCode, 0, t)
}

func TestBuildParams(t *testing.T) {
	params := map[string]string{"foo": "zxc"}

	result, err := postJob(JobRequest{"params", params, ""})
	if err != nil {
		t.Fatal(err)
	}

	out, err := readOut(result, ArtifactsDir)
	if err != nil {
		t.Fatal(err)
	}

	assert(out, "zxc", t)
}

func TestBuildCache(t *testing.T) {
	params := map[string]string{"foo": "bar"}
	group := "baz"

	result1, err := postJob(JobRequest{"build-cache", params, group})
	if err != nil {
		t.Fatal(err)
	}

	out1, err := readOut(result1, ArtifactsDir)
	if err != nil {
		t.Fatal(err)
	}

	cachedOut1, err := readOut(result1, CacheDir)
	if err != nil {
		t.Fatal(err)
	}

	assertEq(out1, cachedOut1, t)

	params["foo"] = "bar2"
	result2, err := postJob(JobRequest{"build-cache", params, group})
	if err != nil {
		t.Fatal(err)
	}

	out2, err := readOut(result2, ArtifactsDir)
	if err != nil {
		t.Fatal(err)
	}

	cachedOut2, err := readOut(result2, CacheDir)
	if err != nil {
		t.Fatal(err)
	}

	assertEq(cachedOut1, cachedOut2, t)
	assertNotEq(out1, out2, t)
	assertNotEq(result1.Path, result2.Path, t)
	assert(result1.ExitCode, 0, t)
	assert(result2.ExitCode, 0, t)
}

func readOut(br *BuildResult, path string) (string, error) {
	out, err := ioutil.ReadFile(filepath.Join(br.Path, "data", path, "out.txt"))
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

func postJob(jr JobRequest) (*BuildResult, error) {
	body, err := json.Marshal(jr)
	if err != nil {
		return nil, err
	}

	req := httptest.NewRequest("POST", "http://example.com/foo", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.handleNewJob(w, req)

	resp := w.Result()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	buildResult := new(BuildResult)
	err = json.Unmarshal(body, buildResult)
	if err != nil {
		return nil, err
	}

	return buildResult, nil
}

func waitForServer(port string) {
	backoff := 50 * time.Millisecond

	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", ":"+port, 1*time.Second)
		if err != nil {
			time.Sleep(backoff)
			continue
		}
		conn.Close()
		return
	}
	log.Fatalf("Server on port %s not up after 10 retries", port)
}
