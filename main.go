package main

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/skroutz/mistry/btrfs"
	"github.com/skroutz/mistry/plainfs"
	"github.com/skroutz/mistry/utils"
	"github.com/urfave/cli"
)

const (
	DataDir      = "/data"      //     - data/
	CacheDir     = "/cache"     //     |- cache/
	ArtifactsDir = "/artifacts" //     |- artifacts/
	ParamsDir    = "/params"    //     |- params/
	BuildLogName = "out.log"    //     - out.log
)

var (
	cfg *Config

	// current list of pending jobs
	jobs = NewJobQueue()

	// available filesystem adapters
	fsList = make(map[string]FileSystem)

	// current filesystem adapter
	curfs FileSystem
)

func init() {
	log.SetFlags(log.Lshortfile)
	fsList["btrfs"] = btrfs.Btrfs{}
	fsList["plain"] = plainfs.PlainFS{}
}

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
			Usage: "Load configuration from `FILE`",
		},
		cli.StringFlag{
			Name:  "filesystem",
			Value: "plain",
			Usage: "Which filesystem adapter to use",
		},
	}
	app.Before = func(c *cli.Context) error {
		f, err := os.Open(c.String("config"))
		if err != nil {
			return err
		}
		cfg, err = ParseConfig(f)
		if err != nil {
			return err
		}
		err = utils.PathIsDir(cfg.ProjectsPath)
		if err != nil {
			return err
		}

		err = utils.PathIsDir(cfg.BuildPath)
		if err != nil {
			return err
		}

		fs, ok := fsList[c.String("filesystem")]
		if !ok {
			return fmt.Errorf("invalid filesystem argument (%v)", fsList)
		}
		curfs = fs
		return nil
	}
	app.Action = func(c *cli.Context) error {
		var wg sync.WaitGroup
		addr := c.String("addr")
		s := NewServer(addr, log.New(os.Stderr, "[http] ", log.LstdFlags))
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.ListenAndServe()
			if err != nil {
				log.Fatal(err)
			}
		}()
		s.Log.Printf("Listening on %s...", addr)
		wg.Wait()
		return nil
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
