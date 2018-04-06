package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alecthomas/template"
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
}

// NewServer accepts a non-nil configuration and an optional logger, and
// returns a new Server.
// If logger is nil, server logs are disabled.
func NewServer(cfg *Config, logger *log.Logger) (*Server, error) {
	if cfg == nil {
		return nil, errors.New("config cannot be nil")
	}

	if logger == nil {
		logger = log.New(ioutil.Discard, "", 0)
	}

	s := new(Server)
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", s.HandleNewJob)

	s.srv = &http.Server{Handler: mux, Addr: cfg.Addr}
	s.cfg = cfg
	s.Log = logger
	s.jq = NewJobQueue()
	s.pq = NewProjectQueue()
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
	j, err := NewJob(jr.Project, jr.Params, jr.Group, s.cfg)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating new job %v: %s", jr, err),
			http.StatusInternalServerError)
		return
	}

	s.Log.Printf("Building %s...", j)
	buildResult, err := s.Work(context.Background(), j)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error building %s: %s", j, err),
			http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")

	resp, err := json.Marshal(buildResult)
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

	var jobList []Job

	projects, err := ioutil.ReadDir(s.cfg.BuildPath)
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, p := range projects {
		pendingPath := filepath.Join(s.cfg.BuildPath, p.Name(), "pending")
		readyPath := filepath.Join(s.cfg.BuildPath, p.Name(), "ready")
		pendingJobs, err := ioutil.ReadDir(pendingPath)
		if err != nil {
			s.Log.Print(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		readyJobs, err := ioutil.ReadDir(readyPath)
		if err != nil {
			s.Log.Print(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for _, j := range pendingJobs {
			buildLogPath := filepath.Join(pendingPath, j.Name(), BuildLogFname)
			ji := Job{
				ID:        j.Name(),
				Project:   p.Name(),
				StartedAt: j.ModTime(),
				Output:    buildLogPath,
				State:     "pending"}
			jobList = append(jobList, ji)
		}

		for _, j := range readyJobs {
			buildLogPath := filepath.Join(readyPath, j.Name(), BuildLogFname)
			ji := Job{
				ID:        j.Name(),
				Project:   p.Name(),
				StartedAt: j.ModTime(),
				Output:    buildLogPath,
				State:     "ready"}
			jobList = append(jobList, ji)
		}
	}

	sort.Slice(jobList, func(i, j int) bool {
		return jobList[i].StartedAt.Unix() > jobList[j].StartedAt.Unix()
	})

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	resp, err := json.Marshal(jobList)
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = w.Write(resp)
	if err != nil {
		s.Log.Print(err)
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

	path := strings.Split(r.URL.Path, "/")
	project := path[2]
	id := path[3]

	state, err := GetState(s.cfg.BuildPath, project, id)
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	jPath := filepath.Join(s.cfg.BuildPath, project, state, id)
	buildLogPath := filepath.Join(jPath, BuildLogFname)
	buildResultFilePath := filepath.Join(jPath, BuildResultFname)
	var rawLog []byte
	var rawResult []byte

	// Decide whether to tail the log file or print it immediately,
	// based on the job state.
	if state != "pending" {
		rawResult, err = ioutil.ReadFile(buildResultFilePath)
		if err != nil {
			s.Log.Print(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	rawLog, err = ioutil.ReadFile(buildLogPath)
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	jDir, err := ioutil.ReadDir(jPath)
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	ji := Job{
		Output:    string(rawResult),
		Log:       string(rawLog),
		ID:        id,
		Project:   project,
		State:     state,
		StartedAt: jDir[0].ModTime(),
	}

	ct := r.Header.Get("Content-type")
	if ct == "application/json" {
		jiData, err := json.Marshal(ji)
		if err != nil {
			s.Log.Print(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(jiData)
		return
	}

	if state == "pending" {
		// For each job id there is only one tailer responsible for
		// emitting the read bytes to the s.br.Notifier channel.
		hasTail, ok := s.tq.Load(id)
		if !ok || hasTail.(bool) == false {
			// Mark the id to the tailers' queue to identify that a
			// tail reader has been spawned.
			s.tq.Store(id, true)
			// Create a channel to communicate the closure of all connections
			// for the job id to the spawned tailer goroutine.
			if _, ok := s.br.CloseClientC[id]; !ok {
				s.br.CloseClientC[id] = make(chan struct{}, 1)
			}
			// Spawns a tailer which tails the build log file and communicates
			// the read results to the s.br.Notifier channel.
			go func() {
				s.Log.Printf("[Tailer] Starting for: %s", id)
				tl, err := tailer.New(buildLogPath)
				if err != nil {
					s.Log.Print(err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				defer tl.Close()
				scanner := bufio.NewScanner(tl)
				for scanner.Scan() {
					select {
					case <-s.br.CloseClientC[id]:
						s.Log.Printf("[Tailer] Exiting for: %s", id)
						s.tq.Store(id, false)
						return
					default:
						s.br.Notifier <- &broker.Event{Msg: []byte(scanner.Text()), ID: id}
					}
				}
			}()
		}
	}

	t, err := template.ParseFiles("./public/templates/show.html")
	if err != nil {
		s.Log.Print(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	t.Execute(w, ji)
}

// HandleServerPush handles the server push logic.
func (s *Server) HandleServerPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Expected GET, got "+r.Method, http.StatusMethodNotAllowed)
		return
	}

	path := strings.Split(r.URL.Path, "/")
	project := path[2]
	id := path[3]

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

	// Set the headers for browsers that support server sent events.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Each connection registers its own event channel with the
	// broker's client connections registry s.br.Clients.
	client := &broker.Client{ID: id, EventC: make(chan []byte)}

	// Signal the broker that we have a new connection.
	s.br.NewClients <- client

	// Remove this client from the map of connected clients when the
	// handler exits.
	defer func() {
		s.br.ClosingClients <- client
	}()

	// Listen to connection close and un-register the client.
	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		s.br.ClosingClients <- client
	}()

	for {
		// Emit the message from the server.
		fmt.Fprintf(w, "data: %s\n\n", <-client.EventC)
		// Send any buffered content to the client immediately.
		flusher.Flush()
	}
}

// ListenAndServe listens on the TCP network address s.srv.Addr and handle
// requests on incoming connections. ListenAndServe always returns a
// non-nil error.
func (s *Server) ListenAndServe() error {
	s.Log.Printf("Configuration: %#v", s.cfg)
	return s.srv.ListenAndServe()
}
