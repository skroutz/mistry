package main

import (
	"encoding/json"
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

// TODO: we should somehow make sure that FileSystem and Addr is set
func ParseConfig(r io.Reader) (*Config, error) {
	c := new(Config)
	dec := json.NewDecoder(r)
	err := dec.Decode(c)
	if err != nil {
		return nil, err
	}

	if c.UID == "" {
		c.UID = strconv.Itoa(os.Getuid())
	}

	return c, nil
}
