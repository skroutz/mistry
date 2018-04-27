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
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/skroutz/mistry/pkg/filesystem"
	_ "github.com/skroutz/mistry/pkg/filesystem/btrfs"
	_ "github.com/skroutz/mistry/pkg/filesystem/plainfs"
	"github.com/skroutz/mistry/pkg/utils"
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

	// DateFmt the date format
	DateFmt = "Mon, 02 Jan 2006 15:04:05"
)

func main() {
	app := cli.NewApp()
	app.Name = "mistry"
	app.Usage = "A powerful building service"
	app.HideVersion = true
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
			Usage: "Which filesystem adapter to use",
		},
	}
	app.Action = func(c *cli.Context) error {
		fs, err := filesystem.Get(c.String("filesystem"))
		if err != nil {
			return err
		}

		f, err := os.Open(c.String("config"))
		if err != nil {
			return fmt.Errorf("cannot parse configuration; %s", err)
		}
		cfg, err := ParseConfig(c.String("addr"), fs, f)
		if err != nil {
			return err
		}

		err = SetUp(cfg)
		if err != nil {
			return err
		}

		return StartServer(cfg)
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

// SetUp accepts a Config and performs necessary initialization tasks.
func SetUp(cfg *Config) error {
	err := utils.PathIsDir(cfg.ProjectsPath)
	if err != nil {
		return err
	}

	err = utils.PathIsDir(cfg.BuildPath)
	if err != nil {
		return err
	}

	err = PruneZombieBuilds(cfg)
	if err != nil {
		return err
	}

	return nil
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
	return nil
}

// PruneZombieBuilds removes any pending builds from the filesystem.
func PruneZombieBuilds(cfg *Config) error {
	projects, err := ioutil.ReadDir(cfg.ProjectsPath)
	if err != nil {
		return err
	}
	l := log.New(os.Stderr, "[cleanup] ", log.LstdFlags)

	for _, p := range projects {
		pendingPath := filepath.Join(cfg.BuildPath, p.Name(), "pending")
		pendingBuilds, err := ioutil.ReadDir(pendingPath)
		for _, pending := range pendingBuilds {
			pendingBuildPath := filepath.Join(pendingPath, pending.Name())
			err = cfg.FileSystem.Remove(pendingBuildPath)
			if err != nil {
				return fmt.Errorf("Error pruning zombie build '%s' of project '%s'", pending.Name(), p.Name())
			}
			l.Printf("Pruned zombie build '%s' of project '%s'", pending.Name(), p.Name())
		}
	}

	return nil
}
