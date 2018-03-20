package main

import (
	"context"
	"fmt"
	"log"
	"os"
)

const (
	DataDir      = "/data"      //     - data/
	CacheDir     = "/cache"     //     |- cache/
	ArtifactsDir = "/artifacts" //     |- artifacts/
	ParamsDir    = "/params"    //     |- params/

	BuildLogName = "out.log"
)

var cfg *Config

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

	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	job, err := NewJob("yogurt-yarn", make(map[string]string), "foo")
	if err != nil {
		log.Fatal(err)
	}

	res, err := Work(context.TODO(), job)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res)
}
