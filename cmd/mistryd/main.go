// Copyright 2018-present Skroutz S.A.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/skroutz/mistry/pkg/filesystem"
	_ "github.com/skroutz/mistry/pkg/filesystem/btrfs"
	_ "github.com/skroutz/mistry/pkg/filesystem/plainfs"
	"github.com/urfave/cli"
)

const (
	// DataDir is the root path where all the data of a given project
	// are placed.
	DataDir = "/data"

	// CacheDir is the directory inside DataDir, containing
	// user-generated files that should be persisted between builds.
	CacheDir = "/cache"

	// ArtifactsDir is the directory inside DataDir, containing the build
	// artifacts.
	ArtifactsDir = "/artifacts"

	// ParamsDir is the directory inside Datadir, containing the job
	// parameters of the build.
	ParamsDir = "/params"

	// BuildLogFname is the file inside DataDir, containing the build log.
	BuildLogFname = "out.log"

	// BuildInfoFname is the file inside DataDir, containing the build
	// info.
	BuildInfoFname = "build_info.json"

	// ImgCntPrefix is the common prefix added to the names of all
	// Docker images/containers created by mistry.
	ImgCntPrefix = "mistry-"

	// DateFmt is the date format used throughout build dates.
	DateFmt = "Mon, 02 Jan 2006 15:04:05"
)

// Version contains the release version of the server, adhering to SemVer.
const Version = "0.1.0"

// VersionSuffix is populated at build-time with -ldflags and typically
// contains the Git SHA1 of the tip that the binary is build from. It is then
// appended to Version.
var VersionSuffix string

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	availableFS := []string{}
	for fs := range filesystem.Registry {
		availableFS = append(availableFS, fs)
	}
	fs := "[" + strings.Join(availableFS, ", ") + "]"

	app := cli.NewApp()
	app.Name = "mistry"
	app.Usage = "A powerful building service"
	app.HideVersion = false
	app.Version = Version
	if VersionSuffix != "" {
		app.Version = Version + "-" + VersionSuffix[:7]
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "addr, a",
			Value: "0.0.0.0:8462",
			Usage: "Host and port to listen to",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: "config.json",
			Usage: "Load configuration from `FILE`",
		},
		cli.StringFlag{
			Name:  "filesystem",
			Value: "plain",
			Usage: "Which filesystem adapter to use. Options: " + fs,
		},
	}
	app.Action = func(c *cli.Context) error {
		cfg, err := parseConfigFromCli(c)
		if err != nil {
			return err
		}
		err = SetUp(cfg)
		if err != nil {
			return err
		}
		return StartServer(cfg)
	}
	app.Commands = []cli.Command{
		{
			Name:  "rebuild",
			Usage: "Rebuild docker images for all projects.",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "fail-fast",
					Usage: "exit immediately on first error",
				},
				cli.StringSliceFlag{
					Name:  "project, p",
					Usage: "the project to build. Multiple projects can be specified. If not passed, all projects are built",
				},
				cli.BoolFlag{
					Name:  "verbose, v",
					Usage: "print logs from docker build and run",
				},
			},
			Action: func(c *cli.Context) error {
				cfg, err := parseConfigFromCli(c.Parent())
				if err != nil {
					return err
				}

				logger := log.New(os.Stdout, "", 0)
				r, err := RebuildImages(cfg, logger, c.StringSlice("project"), c.Bool("fail-fast"), c.Bool("verbose"))
				if err != nil {
					return err
				}
				if len(r.failed) > 0 {
					return fmt.Errorf("%s", r)
				}
				fmt.Printf("Finished. %s\n", r)
				return nil
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// SetUp accepts a Config and performs necessary initialization tasks.
func SetUp(cfg *Config) error {
	err := PruneZombieBuilds(cfg)
	if err != nil {
		return err
	}

	return nil
}

func parseConfigFromCli(c *cli.Context) (*Config, error) {
	fs, err := filesystem.Get(c.String("filesystem"))
	if err != nil {
		return nil, err
	}
	f, err := os.Open(c.String("config"))
	if err != nil {
		return nil, fmt.Errorf("cannot parse configuration; %s", err)
	}
	cfg, err := ParseConfig(c.String("addr"), fs, f)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// StartServer sets up and spawns starts the HTTP server
func StartServer(cfg *Config) error {
	var wg sync.WaitGroup
	s, err := NewServer(cfg, log.New(os.Stderr, "[http] ", log.LstdFlags))
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}
	}()
	s.Log.Printf("Listening on %s...", cfg.Addr)
	wg.Wait()
	s.workerPool.Stop()
	return nil
}
