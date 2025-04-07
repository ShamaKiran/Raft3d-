package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/hashicorp/raft"
)

var (
	raftNode *raft.Raft
	fsmInst  *FSM
	mu       sync.Mutex
)

// submitJobHandler handles submission of new print jobs
func submitJobHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()

	job := PrintJob{
		ID:     fmt.Sprintf("job-%d", len(fsmInst.jobs)+1),
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
