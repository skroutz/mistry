package main

import (
	"context"
	"fmt"
	"log"
	"os"

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
	cfg  *Config
	jobs = NewJobQueue()
)

func init() {
	log.SetFlags(log.Lshortfile)

	f, err := os.Open("config.sample.json")
	if err != nil {
		log.Fatal(err)
	}
	cfg, err = ParseConfig(f)
	if err != nil {
		log.Fatal(err)
	}

	err = PathIsDir(cfg.ProjectsPath)
	if err != nil {
		log.Fatal(err)
	}

	err = PathIsDir(cfg.BuildPath)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "mistry"
	app.Usage = "A powerful building service"
	app.HideVersion = true
	app.Action = func(c *cli.Context) error {
		// TEMP
		f := make(map[string]string)
		f["foo"] = "barxz"
		j, err := NewJob("simple", f, "x1xx1xxfoo")
		if err != nil {
			panic(err)
		}

		out, err := Work(context.TODO(), j, PlainFS{})
		if err != nil {
			panic(err)

		}
		fmt.Println(out)
		return nil
	}

	app.Run(os.Args)

}
