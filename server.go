package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/skroutz/mistry/types"
)

type Server struct {
	Log *log.Logger
	s   *http.Server
}

func NewServer(addr string, logger *log.Logger) *Server {
	s := new(Server)
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", s.handleNewJob)

	s.s = &http.Server{Handler: mux, Addr: addr}
	s.Log = logger
	return s
}

// handleNewJob receives requests for new jobs and schedules their building.
// TODO: also return a JSON report an errors (ideally a BuildResult with Err
// populated)
func (s *Server) handleNewJob(w http.ResponseWriter, r *http.Request) {
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
	j, err := NewJob(jr.Project, jr.Params, jr.Group)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating new job %v: %s", jr, err),
			http.StatusInternalServerError)
		return
	}

	s.Log.Printf("Building %s...", j)
	buildResult, err := Work(context.Background(), j, curfs)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error building %#v: %s", j, err),
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

func (s *Server) ListenAndServe() error {
	return s.s.ListenAndServe()
}
