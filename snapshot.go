package main

import (
	"encoding/json"

	"github.com/hashicorp/raft"
)

// Snapshot implements the raft.FSMSnapshot interface
type Snapshot struct {
	jobs     map[string]PrintJob
	printers map[string]Printer
}

// Persist writes the snapshot to the given sink
func (s *Snapshot) Persist(sink raft.SnapshotSink) error {
	data, err := json.Marshal(map[string]interface{}{
		"jobs":     s.jobs,
		"printers": s.printers,
	})
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

// Release is called when we're done with the snapshot
func (s *Snapshot) Release() {}
