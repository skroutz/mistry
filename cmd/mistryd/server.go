package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/filters"
	docker "github.com/docker/docker/client"
	"github.com/docker/go-units"
	"github.com/skroutz/mistry/pkg/broker"
	"github.com/skroutz/mistry/pkg/types"
)

func staticFs() (http.FileSystem, error) {
	return http.Dir("./cmd/mistryd/public"), nil
}

// Server is the component that performs the actual work (builds images, runs
// commands etc.). It also exposes the JSON API by which users interact with
// mistry.
type Server struct {
	Log *log.Logger

	srv        *http.Server
	jq         *JobQueue
	pq         *ProjectQueue
	cfg        *Config
	workerPool *WorkerPool

	// web-view related

	br *broker.Broker
	fs http.FileSystem
}

// NewServer accepts a non-nil configuration and an optional logger, and
// returns a new Server.
// If logger is nil, server logs are disabled.
func NewServer(cfg *Config, logger *log.Logger) (*Server, error) {
	var err error

	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}

	if logger == nil {
		logger = log.New(ioutil.Discard, "", 0)
	}

	s := new(Server)
	mux := http.NewServeMux()

	//	s.fs, err = fs.New()
	//	if err != nil {
	//		logger.Fatal(err)
	//	}

	fs, err := staticFs()
	if err != nil {
		panic(err)
	}
	s.fs = fs
	mux.Handle("/", http.StripPrefix("/", http.FileServer(s.fs)))
	mux.HandleFunc("/jobs", s.HandleNewJob)
	mux.HandleFunc("/index/", s.HandleIndex)
	mux.HandleFunc("/job/", s.HandleShowJob)
	mux.HandleFunc("/log/", s.HandleServerPush)

	s.srv = &http.Server{Handler: mux, Addr: cfg.Addr}
	s.cfg = cfg
	s.Log = logger
	s.jq = NewJobQueue()
	s.pq = NewProjectQueue()
	s.br = broker.NewBroker(s.Log)
	s.workerPool = NewWorkerPool(s, cfg.Concurrency, cfg.Backlog, logger)
	return s, nil
}

// HandleNewJob receives requests for new jobs and builds them.
func (s *Server) HandleNewJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Expected POST, got "+r.Method, http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()

	jr := types.JobRequest{}
	err = json.Unmarshal(body, &jr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error unmarshalling body '%s' to Job: %s", body, err),
			http.StatusBadRequest)
		return
	}
	j, err := NewJobFromRequest(jr, s.cfg)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating new job %v: %s", jr, err),
			http.StatusInternalServerError)
		return
	}

	// send the work item to the worker pool
	future, err := s.workerPool.SendWork(j)
	if err != nil {
		// the in-memory queue is overloaded, we have to wait for the workers to pick
		// up new items.
		// return a 503 to signal that the server is overloaded and for clients to try
		// again later
		// 503 is an appropriate status code to signal that the server is overloaded
		// for all users, while 429 would have been used if we implemented user-specific
		// throttling
		s.Log.Print("Failed to send message to work queue")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// if async, we're done, otherwise wait for the result in the result channel
	_, async := r.URL.Query()["async"]
	if async {
		s.Log.Printf("Scheduled %s", j)
		w.WriteHeader(http.StatusCreated)
	} else {
		s.Log.Printf("Waiting for result of %s...", j)
		s.writeWorkResult(j, future.Wait(), w)
	}
}

func (s *Server) writeWorkResult(j *Job, r WorkResult, w http.ResponseWriter) {
	if r.Err != nil {
		http.Error(w, fmt.Sprintf("Error building %s: %s", j, r.Err),
			http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")

	resp, err := json.Marshal(r.BuildInfo)
	if err != nil {
		s.Log.Print(err)
	}
	_, err = w.Write([]byte(resp))
	if err != nil {
		s.Log.Printf("Error writing response for %s: %s", j, err)
	}
}

// HandleIndex returns all available jobs.
func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Expected GET, got "+r.Method, http.StatusMethodNotAllowed)
		return
	}

	jobs, err := s.getJobs()
	if err != nil {
		s.Log.Printf("cannot get jobs for path %s; %s", s.cfg.BuildPath, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[j].StartedAt.Before(jobs[i].StartedAt)
	})

	resp, err := json.Marshal(jobs)
	if err != nil {
		s.Log.Print("cannot marshal jobs '%#v'; %s", jobs, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	_, err = w.Write(resp)
	if err != nil {
		s.Log.Print("cannot write response %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// HandleShowJob receives requests for a job and produces the appropriate output
// based on the content type of the request.
func (s *Server) HandleShowJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Expected GET, got "+r.Method, http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	project := parts[2]
	id := parts[3]

	state, err := GetState(s.cfg.BuildPath, project, id)
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	jPath := filepath.Join(s.cfg.BuildPath, project, state, id)

	buildInfo, err := ReadJobBuildInfo(jPath, true)
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	j := Job{
		BuildInfo: buildInfo,
		ID:        id,
		Project:   project,
		State:     state,
	}

	if r.Header.Get("Content-type") == "application/json" {
		jData, err := json.Marshal(j)
		if err != nil {
			s.Log.Print(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(jData)
		return
	}

	f, err := s.fs.Open("/templates/show.html")
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tmplBody, err := ioutil.ReadAll(f)
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tmpl := template.New("jobshow")
	tmpl, err = tmpl.Parse(string(tmplBody))
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, j)
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = buf.WriteTo(w)
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func getJobURL(j *Job) string {
	return strings.Join([]string{"job", j.Project, j.ID}, "/")
}

// HandleServerPush emits build logs as Server-SentEvents (SSE).
func (s *Server) HandleServerPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Expected GET, got "+r.Method, http.StatusMethodNotAllowed)
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 4 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	project := parts[2]
	id := parts[3]

	state, err := GetState(s.cfg.BuildPath, project, id)
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Decide whether to tail the log file and keep the connection alive for
	// sending server side events.
	if state != "pending" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	jPath := filepath.Join(s.cfg.BuildPath, project, state, id)
	buildLogPath := filepath.Join(jPath, BuildLogFname)
	client := &broker.Client{ID: id, Data: make(chan []byte), Extra: buildLogPath}
	s.br.NewClients <- client

	go func() {
		<-w.(http.CloseNotifier).CloseNotify()
		s.br.ClosingClients <- client
	}()

	for {
		msg, ok := <-client.Data
		if !ok {
			break
		}
		fmt.Fprintf(w, "data: %s\n\n", msg)
		flusher.Flush()
	}
}

// ListenAndServe listens on the TCP network address s.srv.Addr and handle
// requests on incoming connections. ListenAndServe always returns a
// non-nil error.
func (s *Server) ListenAndServe() error {
	s.Log.Printf("Configuration: %#v", s.cfg)
	go s.br.ListenForClients()
	return s.srv.ListenAndServe()
}

type pruneResult struct {
	prunedImages     int
	prunedContainers int
	reclaimedSpace   uint64
}

// RebuildResult contains result data on the rebuild operation
type RebuildResult struct {
	successful int
	failed     []string
	pruneResult
}

func (r RebuildResult) String() string {
	var failedNames string
	if len(r.failed) > 0 {
		failedNames = ", Failed names: " + strings.Join(r.failed, ", ")
	}

	return fmt.Sprintf(
		"Rebuilt: %d, Pruned images: %d, Pruned containers: %d, Reclaimed: %s, Failed: %d%s",
		r.successful, r.prunedImages, r.prunedContainers, units.HumanSize(float64(r.reclaimedSpace)),
		len(r.failed), failedNames)
}

// RebuildImages rebuilds images for all projects, and prunes any dangling images
func RebuildImages(cfg *Config, log *log.Logger, projects []string, stopErr, verbose bool) (RebuildResult, error) {
	var err error
	r := RebuildResult{}
	if len(projects) == 0 {
		projects, err = getProjects(cfg.ProjectsPath)
		if err != nil {
			return r, err
		}
	}

	client, err := docker.NewEnvClient()
	if err != nil {
		return r, err
	}

	ctx := context.Background()
	for _, project := range projects {
		start := time.Now()
		log.Printf("Rebuilding %s...\n", project)
		j, err := NewJob(project, types.Params{}, "", cfg)
		if err != nil {
			r.failed = append(r.failed, project)
			if stopErr {
				return r, err
			}
			log.Printf("Failed to instantiate %s job with error: %s\n", project, err)
		} else {
			var buildErr error
			if verbose {
				// pipe image build logs to the logger
				pr, pw := io.Pipe()
				buildResult := make(chan error)

				go func() {
					err := j.BuildImage(ctx, cfg.UID, client, pw, true, true)
					pErr := pw.Close()
					if pErr != nil {
						// as of Go 1.10 this is never non-nil
						log.Printf("Unexpected PipeWriter.Close() error: %s\n", pErr)
					}
					buildResult <- err
				}()

				scanner := bufio.NewScanner(pr)
				for scanner.Scan() {
					log.Print(scanner.Text())
				}
				buildErr = <-buildResult
			} else {
				// discard image build logs
				buildErr = j.BuildImage(ctx, cfg.UID, client, ioutil.Discard, true, true)
			}

			if buildErr != nil {
				r.failed = append(r.failed, project)
				if stopErr {
					return r, buildErr
				}
				log.Printf("Failed to build %s job %s with error: %s\n", project, j.ID, buildErr)
			} else {
				log.Printf("Rebuilt %s in %s\n", project, time.Now().Sub(start).Truncate(time.Millisecond))
				r.successful++
			}
		}
	}
	r.pruneResult, err = dockerPruneUnused(ctx, client)
	if err != nil {
		return r, err
	}
	return r, nil
}

// dockerPruneUnused prunes stopped containers and unused images
func dockerPruneUnused(ctx context.Context, c *docker.Client) (pruneResult, error) {
	// prune containters before images, this will allow more images to be eligible for clean up
	noFilters := filters.NewArgs()
	cr, err := c.ContainersPrune(ctx, noFilters)
	if err != nil {
		return pruneResult{}, err
	}
	ir, err := c.ImagesPrune(ctx, noFilters)
	if err != nil {
		return pruneResult{}, err
	}
	return pruneResult{
		prunedImages:     len(ir.ImagesDeleted),
		prunedContainers: len(cr.ContainersDeleted),
		reclaimedSpace:   ir.SpaceReclaimed + cr.SpaceReclaimed}, nil
}

// PruneZombieBuilds removes any pending builds from the filesystem.
func PruneZombieBuilds(cfg *Config) error {
	projects, err := getProjects(cfg.ProjectsPath)
	if err != nil {
		return err
	}
	l := log.New(os.Stderr, "[cleanup] ", log.LstdFlags)

	for _, p := range projects {
		pendingPath := filepath.Join(cfg.BuildPath, p, "pending")
		pendingBuilds, err := ioutil.ReadDir(pendingPath)
		for _, pending := range pendingBuilds {
			pendingBuildPath := filepath.Join(pendingPath, pending.Name())
			err = cfg.FileSystem.Remove(pendingBuildPath)
			if err != nil {
				return fmt.Errorf("Error pruning zombie build '%s' of project '%s'", pending.Name(), p)
			}
			l.Printf("Pruned zombie build '%s' of project '%s'", pending.Name(), p)
		}
	}
	return nil
}

func getProjects(path string) ([]string, error) {
	folders, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	projects := []string{}
	for _, f := range folders {
		if f.IsDir() {
			projects = append(projects, f.Name())
		}
	}
	return projects, nil
}

// getJobs returns all pending and ready jobs.
func (s *Server) getJobs() ([]Job, error) {
	var jobs []Job
	var pendingJobs []os.FileInfo
	var readyJobs []os.FileInfo

	projects, err := getProjects(s.cfg.BuildPath)
	if err != nil {
		return nil, fmt.Errorf("cannot scan projects; %s", err)
	}

	for _, p := range projects {
		pendingPath := filepath.Join(s.cfg.BuildPath, p, "pending")
		_, err := os.Stat(pendingPath)
		pendingExists := !os.IsNotExist(err)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("cannot check if pending path exists; %s", err)
		}
		readyPath := filepath.Join(s.cfg.BuildPath, p, "ready")
		_, err = os.Stat(readyPath)
		readyExists := !os.IsNotExist(err)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("cannot check if ready path exists; %s", err)
		}

		if pendingExists {
			pendingJobs, err = ioutil.ReadDir(pendingPath)
			if err != nil {
				return nil, fmt.Errorf("cannot scan pending jobs of project %s; %s", p, err)
			}
		}
		if readyExists {
			readyJobs, err = ioutil.ReadDir(readyPath)
			if err != nil {
				return nil, fmt.Errorf("cannot scan ready jobs of project %s; %s", p, err)
			}
		}

		getJob := func(path, jobID, project, state string) (Job, error) {
			bi, err := ReadJobBuildInfo(filepath.Join(path, jobID), false)
			if err != nil {
				return Job{}, err
			}

			return Job{
				ID:        jobID,
				Project:   project,
				StartedAt: bi.StartedAt,
				State:     state,
				BuildInfo: bi}, nil
		}

		for _, j := range pendingJobs {
			job, err := getJob(pendingPath, j.Name(), p, "pending")
			if err != nil {
				return nil, fmt.Errorf("cannot find job %s; %s", j.Name(), err)
			}
			jobs = append(jobs, job)
		}

		for _, j := range readyJobs {
			job, err := getJob(readyPath, j.Name(), p, "ready")
			if err != nil {
				return nil, fmt.Errorf("cannot find job %s; %s", j.Name(), err)
			}
			jobs = append(jobs, job)
		}
	}

	return jobs, nil
}
