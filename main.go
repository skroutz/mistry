package main

import (
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
}
