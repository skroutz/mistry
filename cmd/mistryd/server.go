//go:generate statik -src=./public -f
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sync"

	"github.com/rakyll/statik/fs"
	_ "github.com/skroutz/mistry/cmd/mistryd/statik"
	"github.com/skroutz/mistry/pkg/broker"
	"github.com/skroutz/mistry/pkg/tailer"
	"github.com/skroutz/mistry/pkg/types"
)

// Server is the component that performs the actual work (builds images, runs
// commands etc.). It also exposes the JSON API by which users interact with
// mistry.
type Server struct {
	Log *log.Logger

	srv *http.Server
	jq  *JobQueue
	pq  *ProjectQueue
	cfg *Config

	// web-view related

	// Queue used to track all open tailers by their id. Every tailer id
	// matches a job id.
	// The stored map type is [string]bool.
	tq *sync.Map
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

	s.fs, err = fs.New()
	if err != nil {
		logger.Fatal(err)
	}

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
	s.tq = new(sync.Map)
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

	jr := &types.JobRequest{}
	err = json.Unmarshal(body, jr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error unmarshalling body '%s' to Job: %s", body, err),
			http.StatusBadRequest)
		return
	}
	j, err := NewJobFromRequest(*jr, s.cfg)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating new job %v: %s", jr, err),
			http.StatusInternalServerError)
		return
	}

	if _, isAsync := r.URL.Query()["async"]; isAsync {
		s.handleNewJobAsync(j, w)
	} else {
		s.handleNewJobSync(j, w)
	}
}

// handleNewJobSync triggers the build synchronously, and writes the
// build result JSON to the response
func (s *Server) handleNewJobSync(j *Job, w http.ResponseWriter) {
	s.Log.Printf("Building %s...", j)
	buildInfo, err := s.Work(context.Background(), j)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error building %s: %s", j, err),
			http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")

	resp, err := json.Marshal(buildInfo)
	if err != nil {
		s.Log.Print(err)
	}
	_, err = w.Write([]byte(resp))
	if err != nil {
		s.Log.Printf("Error writing response for %s: %s", j, err)
	}
}

func (s *Server) handleNewJobAsync(j *Job, w http.ResponseWriter) {
	s.Log.Printf("Scheduling %s...", j)
	go s.Work(context.Background(), j)
	w.WriteHeader(http.StatusCreated)
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
	var log []byte
	var buildInfo []byte

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
	buildLogPath := filepath.Join(jPath, BuildLogFname)
	buildInfoPath := filepath.Join(jPath, BuildInfoFname)

	buildInfo, err = ioutil.ReadFile(buildInfoPath)
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log, err = ioutil.ReadFile(buildLogPath)
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	j := Job{
		Output:  string(buildInfo),
		Log:     template.HTML(strings.Replace(string(log), "\n", "<br />", -1)),
		ID:      id,
		Project: project,
		State:   state,
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

	if state == "pending" {
		// For each job id there is only one tailer responsible for
		// emitting the read bytes to the s.br.Notifier channel.
		_, ok := s.tq.Load(id)
		if !ok {
			// Create a channel to communicate the closure of all connections
			// for the job id to the spawned tailer goroutine.
			_, ok := s.br.CloseClientC[id]
			if !ok {
				s.br.CloseClientC[id] = make(chan struct{})
			}

			tl, err := tailer.New(buildLogPath)
			if err != nil {
				s.Log.Print(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Mark the id to the tailers' queue to identify that a
			// tail reader has been spawned.
			s.tq.Store(id, true)

			go func() {
				s.Log.Printf("[tailer] Starting for %s", id)

				scanner := bufio.NewScanner(tl)
				for scanner.Scan() {
					s.br.Notifier <- &broker.Event{Msg: []byte(scanner.Text()), ID: id}
				}
			}()

			go func() {
				tick := time.NewTicker(time.Second * 3)
				defer tick.Stop()

			TAIL_CLOSE_LOOP:
				for {
					select {
					case <-tick.C:
						state, err := GetState(s.cfg.BuildPath, project, id)
						if err != nil {
							s.Log.Print(err)
						}
						if state == "ready" {
							break TAIL_CLOSE_LOOP
						}
					case <-s.br.CloseClientC[id]:
						break TAIL_CLOSE_LOOP
					}
				}
				s.Log.Printf("[tailer] Exiting for: %s", id)
				s.tq.Delete(id)
				err = tl.Close()
				if err != nil {
					s.Log.Print(err)
				}
			}()
		}
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

	client := &broker.Client{ID: id, EventC: make(chan []byte)}
	s.br.NewClients <- client

	defer func() {
		s.br.ClosingClients <- client
	}()

	go func() {
		<-w.(http.CloseNotifier).CloseNotify()
		s.br.ClosingClients <- client
	}()

	for {
		fmt.Fprintf(w, "data: %s\n\n", <-client.EventC)
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

// getJobs returns all pending and ready jobs.
func (s *Server) getJobs() ([]Job, error) {
	var jobs []Job
	var pendingJobs []os.FileInfo
	var readyJobs []os.FileInfo

	projects, err := ioutil.ReadDir(s.cfg.BuildPath)
	if err != nil {
		return nil, fmt.Errorf("cannot scan projects; %s", err)
	}

	for _, p := range projects {
		pendingPath := filepath.Join(s.cfg.BuildPath, p.Name(), "pending")
		_, err := os.Stat(pendingPath)
		pendingExists := !os.IsNotExist(err)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("cannot check if pending path exists; %s", err)
		}
		readyPath := filepath.Join(s.cfg.BuildPath, p.Name(), "ready")
		_, err = os.Stat(readyPath)
		readyExists := !os.IsNotExist(err)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("cannot check if ready path exists; %s", err)
		}

		if pendingExists {
			pendingJobs, err = ioutil.ReadDir(pendingPath)
			if err != nil {
				return nil, fmt.Errorf("cannot scan pending jobs of project %s; %s", p.Name(), err)
			}
		}
		if readyExists {
			readyJobs, err = ioutil.ReadDir(readyPath)
			if err != nil {
				return nil, fmt.Errorf("cannot scan ready jobs of project %s; %s", p.Name(), err)
			}
		}

		getJob := func(path, jobID, project, state string) (Job, error) {
			bi := types.BuildInfo{}
			biBlob, err := ioutil.ReadFile(filepath.Join(path, jobID, BuildInfoFname))
			if err != nil {
				return Job{}, fmt.Errorf("cannot read build_info file; %s", err)
			}
			err = json.Unmarshal(biBlob, &bi)
			if err != nil {
				return Job{}, fmt.Errorf("cannot read build_info file of job %s; %s", jobID, err)
			}

			return Job{
				ID:        jobID,
				Project:   project,
				StartedAt: bi.StartedAt,
				State:     state}, nil
		}

		for _, j := range pendingJobs {
			job, err := getJob(pendingPath, j.Name(), p.Name(), "pending")
			if err != nil {
				return nil, fmt.Errorf("cannot find job %s; %s", j.Name(), err)
			}
			jobs = append(jobs, job)
		}

		for _, j := range readyJobs {
			job, err := getJob(readyPath, j.Name(), p.Name(), "ready")
			if err != nil {
				return nil, fmt.Errorf("cannot find job %s; %s", j.Name(), err)
			}
			jobs = append(jobs, job)
		}
	}

	return jobs, nil
}
