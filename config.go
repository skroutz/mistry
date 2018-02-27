package main

import (
	"encoding/json"
	"io"
	"os"
	"strconv"
)

type Config struct {
	ProjectsPath string `json:"projects_path"`
	BuildPath    string `json:"build_path"`
	UID          string

	// map[source]target
	Mounts map[string]string `json:"mounts"`
}

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
