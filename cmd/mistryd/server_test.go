package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/skroutz/mistry/pkg/types"
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

func TestLoad(t *testing.T) {
	n := 100
	results := make(chan *types.BuildInfo, n)
	rand.Seed(time.Now().UnixNano())

	projects := []string{"concurrent", "concurrent2", "concurrent3", "concurrent4"}
	params := []types.Params{{}, {"foo": "bar"}, {"abc": "efd", "zzz": "xxx"}}
	groups := []string{"", "foo", "abc"}

	for i := 0; i < n; i++ {
		go func() {
			project := projects[rand.Intn(len(projects))]
			params := params[rand.Intn(len(params))]
			group := groups[rand.Intn(len(groups))]

			jr := types.JobRequest{Project: project, Params: params, Group: group}
			time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
			br, err := postJob(jr)
			if err != nil {
				panic(err)
			}
			results <- br
		}()
	}

	for i := 0; i < n; i++ {
		<-results
	}
}

func TestHandleIndex(t *testing.T) {
	cmdout, cmderr, err := cliBuildJob("--project", "simple")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}

	req, err := http.NewRequest("GET", "/index", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.HandleIndex)
	handler.ServeHTTP(rr, req)
	result := rr.Result()

	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, result.StatusCode)
	}

	expected := `"state":"ready"`
	body, err := ioutil.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), expected) {
		t.Errorf("Expeced body to contain %v, got %v", expected, string(body))
	}
}

func TestHandleShowJob(t *testing.T) {
	cmdout, cmderr, err := cliBuildJob("--project", "simple")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}

	// Get a job id and project from the index page.
	req, err := http.NewRequest("GET", "/index", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.HandleIndex)
	handler.ServeHTTP(rr, req)
	result := rr.Result()
	body, err := ioutil.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	job := make([]Job, 0)
	err = json.Unmarshal([]byte(body), &job)
	if err != nil {
		t.Fatal(err)
	}
	jobID := (job[0].ID)
	project := (job[0].Project)

	// Request the show page of the job selected from the index page.
	showPath := path.Join("/job", project, jobID)
	req, err = http.NewRequest("GET", showPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-type", "application/json")
	rr = httptest.NewRecorder()
	handler = http.HandlerFunc(server.HandleShowJob)
	handler.ServeHTTP(rr, req)
	result = rr.Result()

	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, result.StatusCode)
	}

	expected := fmt.Sprintf(`"ID":"%s"`, jobID)
	body, err = ioutil.ReadAll(result.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), expected) {
		t.Errorf("Expeced body to contain %v, got %v", expected, string(body))
	}
}

func TestNewJobAsync(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/jobs?async", strings.NewReader("{\"project\": \"simple\"}"))
	server.srv.Handler.ServeHTTP(rec, req)
	resp := rec.Result()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("Error in reading response body: %s", err)
	}
	assertEq(resp.StatusCode, 201, t)
	assertEq(string(body), "", t)
}
