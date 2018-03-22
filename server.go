package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

type RequestJob struct {
	project string
	params  map[string]string
	group   string
}

type Server struct {
	*http.Server
	Log *log.Logger
}

func NewServer(addr string, logger *log.Logger) *Server {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", s.handleNewJob)

	s.Server = &http.Server{Handler: mux, Addr: addr}
	s.Log = logger
	return s
}

// handleNewJob receives requests for new jobs and schedules their building.
func (s *Server) handleNewJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	r.Body.Close()

	rj := &RequestJob{}
	err = json.Unmarshal(body, rj)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error unmarshalling body '%s' to Job: %s", body, err),
			http.StatusBadRequest)
		return
	}
	j, err := NewJob(rj.project, rj.params, rj.group)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating new job %v: %s", rj, err),
			http.StatusInternalServerError)
		return
	}

	// TODO call Work properly
	ctx, _ := context.WithCancel(context.TODO())
	path, err := Work(ctx, j, curfs)
	err = nil
	if err != nil {
		http.Error(w, fmt.Sprintf("Error building %s: %s", j, err),
			http.StatusInternalServerError)
		return
	}
	s.Log.Println("Enqueued", j.ID)

	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")

	resp, err := json.Marshal(fmt.Sprintf(`{"path":"%s"}`, path))
	if err != nil {
		s.Log.Print(err)
	}
	_, err = w.Write([]byte(resp))
	if err != nil {
		s.Log.Printf("Error writing response for %s: %s", j, err)
	}
}
