package main

import (
	"log"
	"os"
	"strconv"

	docker "github.com/docker/docker/client"
)

var cfg Config

func init() {
	log.SetFlags(log.Lshortfile)

	// TODO: these should be read from config.json
	cfg = Config{ProjectPath: "/var/lib/mistry/projects", BuildPath: "/var/lib/mistry/data"}
	cfg.UID = strconv.Itoa(os.Getuid())
	cfg.Mounts = make(map[string]string)
	// TODO: also support readonly option
	cfg.Mounts["/var/lib/mistry/.ssh"] = "/home/mistry/.ssh"
	//cmd = "docker run --user #{Process.uid} --mount type=bind,source=/var/lib/mistry/.ssh,target=/home/mistry/.ssh,readonly --mount type=bind,source=#{pending_build_path}/data,target=/data "

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

	job, err := NewJob("yogurtyarn", "", lll)
	if err != nil {
		log.Fatal(err)
	}

	client, err := docker.NewEnvClient()
	if err != nil {
		log.Fatal(err)
	}

	err = job.BuildImage(client)
	if err != nil {
		log.Fatal(err)
	}

	err = job.StartContainer(client)
	if err != nil {
		log.Fatal(err)
	}

	//	err = Work(job)
	//	if err != nil {
	//		log.Fatal(err)
	//	}
}
