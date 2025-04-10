package main

import (
	"encoding/json"
	"io"

	"github.com/hashicorp/raft"
)

// PrintJob represents a print job stored in Raft logs
type PrintJob struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// FSM implements the Raft state machine
type FSM struct {
	jobs map[string]PrintJob
}

// Apply applies a Raft log entry
func (f *FSM) Apply(log *raft.Log) interface{} {
	var job PrintJob
	if err := json.Unmarshal(log.Data, &job); err != nil {
		log.Fatal("Failed to decode job:", err)
	}
	f.jobs[job.ID] = job
	return nil
}

// Snapshot returns a snapshot of the current state
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &Snapshot{jobs: f.jobs}, nil
}

// Restore restores the state from a snapshot
func (f *FSM) Restore(reader io.ReadCloser) error {
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&f.jobs); err != nil {
		return err
	}
	return nil
}
