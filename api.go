package main

import (
	"encoding/json"
	"io"
	"log"

	"github.com/hashicorp/raft"
)

// PrintJob represents a print job stored in Raft logs
type PrintJob struct {
	ID             string  `json:"id"`
	Status         string  `json:"status"` // "queued", "printing", "completed", "failed"
	PrinterID      string  `json:"printer_id"`
	FilamentWeight float64 `json:"filament_weight"`
}

// FSM implements the Raft state machine
type FSM struct {
	jobs     map[string]PrintJob
	printers map[string]Printer
}

// Apply applies a Raft log entry
func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	var command map[string]interface{}
	if err := json.Unmarshal(logEntry.Data, &command); err != nil {
		log.Printf("Failed to decode command: %v", err)
		return nil
	}

	cmdType, ok := command["type"].(string)
	if !ok {
		log.Printf("Missing command type")
		return nil
	}

	switch cmdType {
	case "create_printer":
		return f.applyCreatePrinter(command)
	case "submit_job":
		return f.applySubmitJob(command)
	case "update_job_status":
		return f.applyUpdateJobStatus(command)
	case "update_printer_status":
		return f.applyUpdatePrinterStatus(command)
	}

	return nil
}

func (f *FSM) applyCreatePrinter(cmd map[string]interface{}) interface{} {
	printerData, err := json.Marshal(cmd["printer"])
	if err != nil {
		log.Printf("Failed to marshal printer data: %v", err)
		return nil
	}

	var printer Printer
	if err := json.Unmarshal(printerData, &printer); err != nil {
		log.Printf("Failed to unmarshal printer: %v", err)
		return nil
	}

	f.printers[printer.ID] = printer
	return nil
}

func (f *FSM) applySubmitJob(cmd map[string]interface{}) interface{} {
	jobData, err := json.Marshal(cmd["job"])
	if err != nil {
		log.Printf("Failed to marshal job data: %v", err)
		return nil
	}

	var job PrintJob
	if err := json.Unmarshal(jobData, &job); err != nil {
		log.Printf("Failed to unmarshal job: %v", err)
		return nil
	}

	f.jobs[job.ID] = job
	return nil
}

func (f *FSM) applyUpdateJobStatus(cmd map[string]interface{}) interface{} {
	jobID, ok := cmd["job_id"].(string)
	if !ok {
		log.Printf("Missing job ID")
		return nil
	}

	status, ok := cmd["status"].(string)
	if !ok {
		log.Printf("Missing status")
		return nil
	}

	job, exists := f.jobs[jobID]
	if !exists {
		log.Printf("Job not found: %s", jobID)
		return nil
	}

	job.Status = status
	f.jobs[jobID] = job

	// If status is "completed", update printer filament
	if status == "completed" {
		if job.PrinterID != "" {
			printer, exists := f.printers[job.PrinterID]
			if exists {
				// Reduce filament weight
				printer.FilamentWeight -= job.FilamentWeight
				printer.Status = "idle"
				printer.CurrentJobID = ""
				f.printers[job.PrinterID] = printer
			}
		}
	}

	return nil
}

func (f *FSM) applyUpdatePrinterStatus(cmd map[string]interface{}) interface{} {
	printerID, ok := cmd["printer_id"].(string)
	if !ok {
		log.Printf("Missing printer ID")
		return nil
	}

	status, ok := cmd["status"].(string)
	if !ok {
		log.Printf("Missing status")
		return nil
	}

	printer, exists := f.printers[printerID]
	if !exists {
		log.Printf("Printer not found: %s", printerID)
		return nil
	}

	printer.Status = status

	// If a job ID is provided, update it
	if jobID, ok := cmd["job_id"].(string); ok {
		printer.CurrentJobID = jobID
	}

	f.printers[printerID] = printer
	return nil
}

// Snapshot returns a snapshot of the current state
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &Snapshot{jobs: f.jobs, printers: f.printers}, nil
}

// Restore restores the state from a snapshot
func (f *FSM) Restore(reader io.ReadCloser) error {
	var data map[string]interface{}
	if err := json.NewDecoder(reader).Decode(&data); err != nil {
		return err
	}

	// Restore jobs
	jobsData, err := json.Marshal(data["jobs"])
	if err != nil {
		return err
	}
	if err := json.Unmarshal(jobsData, &f.jobs); err != nil {
		return err
	}

	// Restore printers
	printersData, err := json.Marshal(data["printers"])
	if err != nil {
		return err
	}
	if err := json.Unmarshal(printersData, &f.printers); err != nil {
		return err
	}

	return nil
}
