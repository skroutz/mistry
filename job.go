package main

import "errors"

type Job struct {
	Project string
	//Project *Project

	Params map[string]string
}

func NewJob(project string, params map[string]string) (*Job, error) {
	if project == "" {
		return nil, errors.New("No project given")
	}

	return &Job{project, params}, nil
}
