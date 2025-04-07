package main

import (
	"encoding/json"
	"io"
	"log" // 👈 for log.Fatal
	"sync"

	"github.com/hashicorp/raft"
)

// PrintJob represents a print job stored in Raft logs
type PrintJob struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// FSM implements the Raft state machine
type FSM struct {
	mu   sync.Mutex
	jobs map[string]PrintJob
}

// NewFSM returns an initialized FSM
func NewFSM() *FSM {
	return &FSM{
		jobs: make(map[string]PrintJob),
	}
}

// Apply applies a Raft log entry
func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	var job PrintJob
	if err := json.Unmarshal(logEntry.Data, &job); err != nil {
		log.Fatal("Failed to decode job:", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobs[job.ID] = job
	return nil
}

// Snapshot returns a snapshot of the current state
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Create a deep copy to freeze state
	jobsCopy := make(map[string]PrintJob)
	for k, v := range f.jobs {
		jobsCopy[k] = v
	}
	return &fsmSnapshot{jobs: jobsCopy}, nil
}

// Restore restores the state from a snapshot
func (f *FSM) Restore(reader io.ReadCloser) error {
	defer reader.Close()
	decoder := json.NewDecoder(reader)
	var jobs map[string]PrintJob
	if err := decoder.Decode(&jobs); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobs = jobs
	return nil
}

// fsmSnapshot implements raft.FSMSnapshot
type fsmSnapshot struct {
	jobs map[string]PrintJob
}

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	data, err := json.Marshal(s.jobs)
	if err != nil {
		sink.Cancel()
		return err
	}
	if _, err := sink.Write(data); err != nil {
		sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *fsmSnapshot) Release() {}
