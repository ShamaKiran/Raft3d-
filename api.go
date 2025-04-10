package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/raft"
)

var raftNode *raft.Raft

func submitJobHandler(w http.ResponseWriter, r *http.Request) {
	job := PrintJob{
		ID:     fmt.Sprintf("job-%d", len(fsm.jobs)+1),
		Status: "queued",
	}

	// Serialize job to JSON
	jobBytes, err := json.Marshal(job)
	if err != nil {
		http.Error(w, "Failed to serialize job", http.StatusInternalServerError)
		return
	}

	// Apply job to Raft log
	applyFuture := raftNode.Apply(jobBytes, 0)
	if err := applyFuture.Error(); err != nil {
		http.Error(w, "Raft apply failed", http.StatusInternalServerError)
		return
	}

	// Respond with job details
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}
