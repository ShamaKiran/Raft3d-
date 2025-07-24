package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// JobRequest represents the input to create a print job
type JobRequest struct {
	PrinterID      string  `json:"printer_id"`
	FilamentWeight float64 `json:"filament_weight"`
}

func submitJobHandler(w http.ResponseWriter, r *http.Request) {
	var jobReq JobRequest

	if err := json.NewDecoder(r.Body).Decode(&jobReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if printer exists
	printer, exists := fsm.printers[jobReq.PrinterID]
	if !exists {
		http.Error(w, "Printer not found", http.StatusNotFound)
		return
	}

	// Check if printer has enough filament
	if printer.FilamentWeight < jobReq.FilamentWeight {
		http.Error(w, "Not enough filament", http.StatusBadRequest)
		return
	}

	// Check if printer is idle
	if printer.Status != "idle" {
		http.Error(w, "Printer is busy", http.StatusBadRequest)
		return
	}

	job := PrintJob{
		ID:             fmt.Sprintf("job-%d", len(fsm.jobs)+1),
		Status:         "queued",
		PrinterID:      jobReq.PrinterID,
		FilamentWeight: jobReq.FilamentWeight,
	}

	// Create command
	command := map[string]interface{}{
		"type": "submit_job",
		"job":  job,
	}

	commandBytes, err := json.Marshal(command)
	if err != nil {
		http.Error(w, "Failed to serialize command", http.StatusInternalServerError)
		return
	}

	// Apply command to Raft log
	applyFuture := raftNode.Apply(commandBytes, 0)
	if err := applyFuture.Error(); err != nil {
		http.Error(w, "Raft apply failed", http.StatusInternalServerError)
		return
	}

	// Update printer status
	updatePrinterForJob(jobReq.PrinterID, job.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func updatePrinterForJob(printerID, jobID string) {
	// Create command to update printer status
	command := map[string]interface{}{
		"type":       "update_printer_status",
		"printer_id": printerID,
		"status":     "printing",
		"job_id":     jobID,
	}

	commandBytes, _ := json.Marshal(command)
	raftNode.Apply(commandBytes, 0)
}

func getJobsHandler(w http.ResponseWriter, r *http.Request) {
	// Return all jobs
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fsm.jobs)
}

func getJobHandler(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from URL path
	jobID := r.URL.Path[len("/jobs/"):]

	job, exists := fsm.jobs[jobID]
	if !exists {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

func updateJobStatusHandler(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from URL path
	jobID := r.URL.Path[len("/jobs/"):]

	var statusUpdate struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&statusUpdate); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create a command to update job status
	command := map[string]interface{}{
		"type":   "update_job_status",
		"job_id": jobID,
		"status": statusUpdate.Status,
	}

	commandBytes, err := json.Marshal(command)
	if err != nil {
		http.Error(w, "Failed to serialize command", http.StatusInternalServerError)
		return
	}

	// Apply command to Raft log
	applyFuture := raftNode.Apply(commandBytes, 0)
	if err := applyFuture.Error(); err != nil {
		http.Error(w, "Raft apply failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fsm.jobs[jobID])
}
