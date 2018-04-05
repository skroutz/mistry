package main

import (
	"bytes"
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
	"testing"
	"time"

	"github.com/skroutz/mistry/filesystem"
	"github.com/skroutz/mistry/types"
	"github.com/skroutz/mistry/utils"
)

var (
	testcfg *Config
	server  *Server
	params  = make(types.Params)

	// mistry-cli args
	host     string
	port     string
	username string
	target   string

	addrFlag       string
	configFlag     string
	filesystemFlag string
)

func TestMain(m *testing.M) {
	flag.StringVar(&addrFlag, "addr", "localhost:8462", "")
	flag.StringVar(&configFlag, "config", "config.test.json", "")
	flag.StringVar(&filesystemFlag, "filesystem", "plain", "")
	flag.Parse()

	parts := strings.Split(addrFlag, ":")
	if len(parts) != 2 {
		panic("invalid addr argument")
	}
	host = parts[0]
	port = parts[1]

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
	username = user.Username

	server = NewServer(testcfg, log.New(os.Stderr, "[http] ", log.LstdFlags))

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
	waitForServer(port)

	target, err = ioutil.TempDir("", "mistry-test-artifacts")
	if err != nil {
		panic(err)
	}

	result := m.Run()

	if result == 0 {
		err = os.RemoveAll(testcfg.BuildPath)
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

func TestPruneZombieBuilds(t *testing.T) {
	project := "hanging-pending"
	cmdout, cmderr, err := cliBuildJob("--project", project)
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}
	path := filepath.Join(testcfg.BuildPath, project, "pending")
	_, err = utils.RunCmd(testcfg.FileSystem.Create(filepath.Join(path, "foo")))
	if err != nil {
		t.Fatal(err)
	}
	_, err = utils.RunCmd(testcfg.FileSystem.Create(filepath.Join(path, "bar")))
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
		err = conn.Close()
		if err != nil {
			log.Fatal(err)
		}
		return
	}
	log.Fatalf("Server on port %s not up after 10 retries", port)
}

func parseClientJSON(s string) (*types.BuildResult, error) {
	br := new(types.BuildResult)
	err := json.Unmarshal([]byte(s), br)
	if err != nil {
		return nil, fmt.Errorf("Couldn't unmarshall '%s'", s)
	}
	return br, nil
}
