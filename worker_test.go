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
	"net/http"
	"net/http/httptest"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/skroutz/mistry/types"
)

const (
	host = "localhost"
	port = "8462"
)

var server = NewServer("localhost:8462", log.New(os.Stdout, "test", log.Lshortfile))
var params = make(map[string]string)
var username, target string

func init() {
	flag.String("config", "", "")
	flag.String("filesystem", "", "")
	user, err := user.Current()
	if err != nil {
		panic(err)
	}
	username = user.Username
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

	target, err = ioutil.TempDir("", "mistry-tests-results")
	if err != nil {
		panic(err)
	}

	result := m.Run()

	if result == 0 {
		err = os.RemoveAll(cfg.BuildPath)
		if err != nil {
			panic(err)
		}
		err = os.RemoveAll(target)
		if err != nil {
			panic(err)
		}
	}

	os.Exit(result)
}

// TODO: do this using error types on BuildResult, instead of string comparison
func TestImageBuildFailure(t *testing.T) {
	expErr := "could not build docker image"

	_, err := postJob(types.JobRequest{"image-build-failure", params, ""})
	if !strings.Contains(err.Error(), expErr) {
		t.Fatalf("Expected '%s' to contain '%s'", err.Error(), expErr)
	}
}

// TODO convert to end-to-end. The CLI must know about exit codes in order
// to do that.
func TestExitCode(t *testing.T) {
	result, err := postJob(types.JobRequest{"exit-code", params, ""})
	if err != nil {
		t.Fatal(err)
	}

	assert(result.ExitCode, 77, t)
}

func TestBuildCache(t *testing.T) {
	params := map[string]string{"foo": "bar"}
	group := "baz"

	result1, err := postJob(types.JobRequest{"build-cache", params, group})
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
	result2, err := postJob(types.JobRequest{"build-cache", params, group})
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

func TestFailedPendingBuildCleanup(t *testing.T) {
	var err error
	project := "failed-build-cleanup"
	expected := "unknown instruction: INVALIDCOMMAND"

	for i := 0; i < 3; i++ {
		_, err = postJob(types.JobRequest{project, params, ""})
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("Expected '%s' to contain '%s'", err.Error(), expected)
		}
	}
}

func TestConcurrentJobs(t *testing.T) {
	t.Skip("TODO: fix races")
	var wg sync.WaitGroup
	results := make(chan *types.BuildResult, 100)

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

func readOut(br *types.BuildResult, path string) (string, error) {
	s := strings.Replace(br.Path, "/data/artifacts", "", -1)
	out, err := ioutil.ReadFile(filepath.Join(s, "data", path, "out.txt"))
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

// postJob issues an HTTP request with jr to the server. It returns an error if
// the request was not successful.
func postJob(jr types.JobRequest) (*types.BuildResult, error) {
	body, err := json.Marshal(jr)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal %#v; %s", jr, err)
	}

	req := httptest.NewRequest("POST", "http://example.com/foo", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.handleNewJob(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal("Could not read response body + ", err.Error())
		}
		return nil, fmt.Errorf("Expected status=201, got %d | body: %s", resp.StatusCode, body)
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	buildResult := new(types.BuildResult)
	err = json.Unmarshal(body, buildResult)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal %#v; %s", string(body), err)
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
