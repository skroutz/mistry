package main

import (
	"sync"
	"testing"
)

func TestBootstrapProjectRace(t *testing.T) {
	n := 10
	project := "bootstrap-concurrent"
	jobs := []*Job{}
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		j, err := NewJob(project, params, "", testcfg)
		if err != nil {
			t.Fatal(err)
		}
		jobs = append(jobs, j)
	}

	for _, j := range jobs {
		wg.Add(1)
		go func(j *Job) {
			defer wg.Done()
			err := server.BootstrapProject(j)
			if err != nil {
				panic(err)
			}
		}(j)
	}
	wg.Wait()
}
