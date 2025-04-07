package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

var mu sync.Mutex // local mutex for sync

// submitJobHandler handles job submission
func submitJobHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	job := PrintJob{
		ID:     fmt.Sprintf("job-%d", len(fsmInst.jobs)+1),
		Status: "queued",
	}

	jobBytes, err := json.Marshal(job)
	if err != nil {
		http.Error(w, "Failed to serialize job", http.StatusInternalServerError)
		return
	}

	applyFuture := raftNode.Apply(jobBytes, 0)
	if err := applyFuture.Error(); err != nil {
		http.Error(w, "Raft apply failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// getJobsHandler returns all print jobs
func getJobsHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	jobs := make([]PrintJob, 0, len(fsmInst.jobs))
	for _, job := range fsmInst.jobs {
		jobs = append(jobs, job)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

// updateJobHandler updates the status of a job
func updateJobHandler(w http.ResponseWriter, r *http.Request) {
	var updatedJob PrintJob
	if err := json.NewDecoder(r.Body).Decode(&updatedJob); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	jobBytes, err := json.Marshal(updatedJob)
	if err != nil {
		http.Error(w, "Failed to serialize update", http.StatusInternalServerError)
		return
	}

	applyFuture := raftNode.Apply(jobBytes, 0)
	if err := applyFuture.Error(); err != nil {
		http.Error(w, "Raft apply failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Job %s updated to %s", updatedJob.ID, updatedJob.Status)
}
