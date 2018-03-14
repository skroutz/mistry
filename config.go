package main

type Config struct {
	ProjectPath string // var/lib/mistry/projects ?

	BuildPath string // var/lib/mistry/data

	UID string

	// Mounts that apply to all containers
	//
	// map[source]target
	Mounts map[string]string
}
