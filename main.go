package main

import (
	"log"
	"os"
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
	job, err := NewJob("yogurt-yarn", "", nil)
	if err != nil {
		log.Fatal(err)
	}

	//client, err := docker.NewEnvClient()
	//if err != nil {
	//	log.Fatal(err)
	//}

	//	err = job.BuildImage(client)
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//
	//	err = job.StartContainer(client)
	//	if err != nil {
	//		log.Fatal(err)
	//	}

	err = Work(job)
	if err != nil {
		log.Fatal(err)
	}
}
