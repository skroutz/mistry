package main

import (
	"testing"
	"time"

	"github.com/skroutz/mistry/pkg/types"
)

func TestBacklogLimit(t *testing.T) {
	wp, cfg := setupQueue(t, 0, 1)
	defer wp.Stop()

	params := types.Params{"test": "pool-backlog-limit"}
	params2 := types.Params{"test": "pool-backlog-limit2"}
	project := "simple"

	sendWorkNoErr(wp, project, params, cfg, t)
	_, _, err := sendWork(wp, project, params2, cfg, t)

	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestConcurrency(t *testing.T) {
	// instatiate server with 1 worker
	wp, cfg := setupQueue(t, 1, 100)
	defer wp.Stop()

	project := "sleep"
	params := types.Params{"test": "pool-concurrency"}
	params2 := types.Params{"test": "pool-concurrency2"}

	sendWorkNoErr(wp, project, params, cfg, t)
	// give the chance for the worker to start work
	time.Sleep(1 * time.Second)

	j, _ := sendWorkNoErr(wp, project, params2, cfg, t)

	// the queue should contain only 1 item, the work item for the 2nd job
	assertEq(len(wp.queue), 1, t)
	select {
	case i, ok := <-wp.queue:
		if !ok {
			t.Fatalf("Unexpectedly closed worker pool queue")
		}
		assertEq(i.job, j, t)
	default:
		t.Fatalf("Expected to find a work item in the queue")
	}
}

func setupQueue(t *testing.T, workers, backlog int) (*WorkerPool, *Config) {
	cfg := testcfg
	cfg.Concurrency = workers
	cfg.Backlog = backlog

	s, err := NewServer(cfg, nil)
	failIfError(err, t)
	return s.workerPool, cfg
}

func sendWork(wp *WorkerPool, project string, params types.Params, cfg *Config, t *testing.T) (*Job, FutureWorkResult, error) {
	j, err := NewJob(project, params, "", cfg)
	failIfError(err, t)

	r, err := wp.SendWork(j)
	return j, r, err
}

func sendWorkNoErr(wp *WorkerPool, project string, params types.Params, cfg *Config, t *testing.T) (*Job, FutureWorkResult) {
	j, r, err := sendWork(wp, project, params, cfg, t)
	failIfError(err, t)
	return j, r
}
