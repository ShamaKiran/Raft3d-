package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hashicorp/raft"
)

// PrintJob represents a print job in the system
type PrintJob struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// FSM implements raft.FSM and manages PrintJobs
type FSM struct {
	jobs map[string]PrintJob
}

func NewFSM() *FSM {
	return &FSM{jobs: make(map[string]PrintJob)}
}

func (f *FSM) Apply(log *raft.Log) interface{} {
	var job PrintJob
	if err := json.Unmarshal(log.Data, &job); err != nil {
		fmt.Println("Failed to unmarshal log:", err)
		return nil
	}

	f.jobs[job.ID] = job
	return nil
}

func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{jobs: f.jobs}, nil
}

func (f *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	return json.NewDecoder(rc).Decode(&f.jobs)
}

// Snapshot structure
type snapshot struct {
	jobs map[string]PrintJob
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	b, err := json.Marshal(s.jobs)
	if err != nil {
		sink.Cancel()
		return err
	}

	if _, err := sink.Write(b); err != nil {
		sink.Cancel()
		return err
	}

	return sink.Close()
}

func (s *snapshot) Release() {}

// --- ✅ Globals and Setters moved here ---
var (
	raftNode *raft.Raft
	fsmInst  *FSM
)

func SetFSMInstance(f *FSM) {
	fsmInst = f
}

func SetRaftInstance(r *raft.Raft) {
	raftNode = r
}
