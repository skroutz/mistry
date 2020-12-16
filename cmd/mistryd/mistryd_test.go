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
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	docker "github.com/docker/docker/client"
	"github.com/skroutz/mistry/pkg/filesystem"
	"github.com/skroutz/mistry/pkg/types"
)

var (
	testcfg *Config
	server  *Server
	logger  *log.Logger
	params  = make(types.Params)

	// mistry-cli args
	cliDefaultArgs CliCommonArgs

	addrFlag       string
	configFlag     string
	filesystemFlag string
)

type CliCommonArgs struct {
	host     string
	port     string
	username string
	target   string
}

func TestMain(m *testing.M) {
	flag.StringVar(&addrFlag, "addr", "127.0.0.1:8462", "")
	flag.StringVar(&configFlag, "config", "config.test.json", "")
	flag.StringVar(&filesystemFlag, "filesystem", "plain", "")
	flag.Parse()

	parts := strings.Split(addrFlag, ":")
	if len(parts) != 2 {
		panic("invalid addr argument")
	}
	cliDefaultArgs.host = parts[0]
	cliDefaultArgs.port = parts[1]

	fs, err := filesystem.Get(filesystemFlag)
	if err != nil {
		panic(err)
	}
	f, err := os.Open(configFlag)
	if err != nil {
		panic(err)
	}
	testcfg, err = ParseConfig(addrFlag, fs, f)
	if err != nil {
		panic(err)
	}

	tmpdir, err := ioutil.TempDir("", "mistry-tests")
	if err != nil {
		panic(err)
	}
	// on macOS '/tmp' is a symlink to '/private/tmp'
	testcfg.BuildPath, err = filepath.EvalSymlinks(tmpdir)
	if err != nil {
		panic(err)
	}

	user, err := user.Current()
	if err != nil {
		panic(err)
	}
	cliDefaultArgs.username = user.Username

	logger = log.New(os.Stderr, "[http] ", log.LstdFlags)

	server, err = NewServer(testcfg, logger, false)
	if err != nil {
		panic(err)
	}

	go func() {
		err := SetUp(testcfg)
		if err != nil {
			panic(err)
		}
		err = StartServer(testcfg)
		if err != nil {
			panic(err)
		}

	}()
	waitForServer(cliDefaultArgs.port)

	cliDefaultArgs.target, err = ioutil.TempDir("", "mistry-test-artifacts")
	if err != nil {
		panic(err)
	}

	result := m.Run()
	if result == 0 {
		err = os.RemoveAll(testcfg.BuildPath)
		if err != nil {
			panic(err)
		}
		err = os.RemoveAll(cliDefaultArgs.target)
		if err != nil {
			panic(err)
		}
	}

	os.Exit(result)
}

func TestPruneZombieBuilds(t *testing.T) {
	project := "hanging-pending"
	cmdout, cmderr, err := cliBuildJob("--project", project)
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}
	path := filepath.Join(testcfg.BuildPath, project, "pending")
	err = testcfg.FileSystem.Create(filepath.Join(path, "foo"))
	if err != nil {
		t.Fatal(err)
	}
	err = testcfg.FileSystem.Create(filepath.Join(path, "bar"))
	if err != nil {
		t.Fatal(err)
	}

	err = PruneZombieBuilds(testcfg)
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

func TestRebuildImages(t *testing.T) {
	// run a job, fetch its build time
	params := types.Params{"test": "rebuild-server"}
	cmdout, cmderr, err := cliBuildJob("--project", "simple", "--", toCli(params)[0])
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}

	j, err := NewJob("simple", params, "", testcfg)
	if err != nil {
		t.Fatalf("%s", err)
	}

	client, err := docker.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}

	i, _, err := client.ImageInspectWithRaw(context.Background(), j.Image)
	if err != nil {
		t.Fatal(err)
	}

	r, err := RebuildImages(testcfg, logger, []string{"simple"}, true, true)
	failIfError(err, t)
	assertEq(r.successful, 1, t)
	assertEq(len(r.failed), 0, t)

	// fetch last build time, make sure it is different
	i2, _, err := client.ImageInspectWithRaw(context.Background(), j.Image)
	if err != nil {
		t.Fatal(err)
	}
	assertNotEq(i.Created, i2.Created, t)
}

func TestRebuildImagesNonExistingProject(t *testing.T) {
	r, err := RebuildImages(testcfg, logger, []string{"shouldnotexist"}, true, true)
	assertEq(r.successful, 0, t)
	assertEq(r.failed, []string{"shouldnotexist"}, t)
	if err == nil {
		t.Fatal("Expected unknown project error")
	}
}

func readOut(bi *types.BuildInfo, path string) (string, error) {
	s := strings.Replace(bi.Path, "/data/artifacts", "", -1)
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
func postJob(jr types.JobRequest) (*types.BuildInfo, error) {
	body, err := json.Marshal(jr)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal %#v; %s", jr, err)
	}

	req := httptest.NewRequest("POST", "http://example.com/foo", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.HandleNewJob(w, req)

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

	buildInfo := new(types.BuildInfo)
	err = json.Unmarshal(body, buildInfo)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal %#v; %s", string(body), err)
	}

	return buildInfo, nil
}

func waitForServer(port string) {
	backoff := 50 * time.Millisecond

	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", ":"+port, 1*time.Second)
		if err != nil {
			time.Sleep(backoff)
			continue
		}
		err = conn.Close()
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	log.Fatalf("Server on port %s not up after 10 retries", port)
}

func parseClientJSON(s string) (*types.BuildInfo, error) {
	bi := new(types.BuildInfo)
	err := json.Unmarshal([]byte(s), bi)
	if err != nil {
		return nil, fmt.Errorf("Couldn't unmarshall '%s'", s)
	}
	return bi, nil
}

// cliBuildJob uses the CLI binary to issue a new job request to the server.
// It returns an error if the request could not be issued or if the job
// failed to build.
//
// NOTE: The CLI binary is expected to be present in the directory denoted by
// MISTRY_CLIENT_PATH environment variable or, if empty,  from the current
// working directory where the tests are ran from.
func cliBuildJob(args ...string) (string, string, error) {
	return cliBuildJobArgs(cliDefaultArgs, args...)
}

func cliBuildJobArgs(cliArgs CliCommonArgs, args ...string) (string, string, error) {
	clientPath := os.Getenv("MISTRY_CLIENT_PATH")
	if clientPath == "" {
		clientPath = "./mistry"
	}
	args = append([]string{
		clientPath, "build",
		"--verbose",
		"--host", cliArgs.host,
		"--port", cliArgs.port,
		"--target", cliArgs.target,
		"--transport-user", cliArgs.username},
		args...)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
