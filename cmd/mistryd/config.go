package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"strconv"

	"github.com/skroutz/mistry/pkg/filesystem"
	"github.com/skroutz/mistry/pkg/utils"
)

// Config holds the configuration values that the Server needs in order to
// function.
type Config struct {
	Addr       string
	FileSystem filesystem.FileSystem
	UID        string

	ProjectsPath string            `json:"projects_path"`
	BuildPath    string            `json:"build_path"`
	Mounts       map[string]string `json:"mounts"`

	Concurrency int `json:"job_concurrency"`
	Backlog     int `json:"job_backlog"`
}

// ParseConfig accepts the listening address, a filesystem adapter and a
// reader from which to parse the configuration, and returns a valid
// Config or an error.
func ParseConfig(addr string, fs filesystem.FileSystem, r io.Reader) (*Config, error) {
	if addr == "" {
		return nil, errors.New("addr must be provided")
	}

	cfg := new(Config)
	cfg.Addr = addr
	cfg.FileSystem = fs

	dec := json.NewDecoder(r)
	err := dec.Decode(cfg)
	if err != nil {
		return nil, err
	}

	if cfg.UID == "" {
		cfg.UID = strconv.Itoa(os.Getuid())
	}

	err = utils.PathIsDir(cfg.ProjectsPath)
	if err != nil {
		return nil, err
	}

	err = utils.PathIsDir(cfg.BuildPath)
	if err != nil {
		return nil, err
	}

	if cfg.Concurrency == 0 {
		// our work is CPU bound so number of cores is OK
		cfg.Concurrency = runtime.NumCPU()
	}

	if cfg.Backlog == 0 {
		// by default allow a request spike double the worker capacity
		cfg.Backlog = cfg.Concurrency * 2
	}

	return cfg, nil
}
