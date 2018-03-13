package main

import (
	"log"

	docker "github.com/docker/docker/client"
)

var cfg Config

func init() {
	log.SetFlags(log.Lshortfile)

	cfg = Config{ProjectPath: "/var/lib/mistry/projects", BuildPath: "/var/lib/mistry/data"}

	//	err := PathIsDir(cfg.ProjectPath)
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//
	//	err = PathIsDir(cfg.BuildPath)
	//	if err != nil {
	//		log.Fatal(err)
	//	}
}

func main() {
	lll := make(map[string]string)
	lll["foo"] = "bar"
	lll["asemas"] = "re"
	lll["zzz"] = "yo"
	lll["aaaaaa"] = "cxzcxzcx"

	job, err := NewJob("yogurt-assets", "", lll)
	if err != nil {
		log.Fatal(err)
	}

	client, err := docker.NewEnvClient()
	if err != nil {
		log.Fatal(err)
	}
	job.BuildImage(client)
	//	err = Work(job)
	//	if err != nil {
	//		log.Fatal(err)
	//	}
}
