package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

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

	f, err := os.Open(configFlag)
	if err != nil {
		panic(err)
	}
	testcfg, err = ParseConfig(f)
	if err != nil {
		panic(err)
	}

	testcfg.Addr = addrFlag

	fs, ok := filesystem.List[filesystemFlag]
	if !ok {
		panic(fmt.Sprintf("invalid filesystem argument (%v)", filesystem.List))
	}
	testcfg.FileSystem = fs

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

	fmt.Printf("Configuration: %#v\n", testcfg)

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
	cmdOut, err := cliBuildJob("--project", project)
	if err != nil {
		t.Fatalf("Error output: %s, err: %v", cmdOut, err)
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
