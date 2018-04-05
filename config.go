package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"strconv"

	"github.com/skroutz/mistry/filesystem"
)

type Config struct {
	Addr       string
	FileSystem filesystem.FileSystem
	UID        string

	ProjectsPath string            `json:"projects_path"`
	BuildPath    string            `json:"build_path"`
	Mounts       map[string]string `json:"mounts"`
}

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

	return cfg, nil
}
