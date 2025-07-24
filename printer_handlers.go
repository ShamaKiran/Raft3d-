package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Printer represents a 3D printer in the system
type Printer struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Status         string  `json:"status"` // "idle", "printing", "error"
	FilamentWeight float64 `json:"filament_weight"`
	CurrentJobID   string  `json:"current_job_id,omitempty"`
}

// PrinterRequest represents the input to create a printer
type PrinterRequest struct {
	Name           string  `json:"name"`
	FilamentWeight float64 `json:"filament_weight"`
}

func createPrinterHandler(w http.ResponseWriter, r *http.Request) {
	var printerReq PrinterRequest

	if err := json.NewDecoder(r.Body).Decode(&printerReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	printer := Printer{
		ID:             fmt.Sprintf("printer-%d", len(fsm.printers)+1),
		Name:           printerReq.Name,
		Status:         "idle",
		FilamentWeight: printerReq.FilamentWeight,
	}

	// Create command
	command := map[string]interface{}{
		"type":    "create_printer",
		"printer": printer,
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
	json.NewEncoder(w).Encode(printer)
}

func getPrintersHandler(w http.ResponseWriter, r *http.Request) {
	// Return all printers
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fsm.printers)
}

func getPrinterHandler(w http.ResponseWriter, r *http.Request) {
	// Extract printer ID from URL path
	printerID := r.URL.Path[len("/printers/"):]

	printer, exists := fsm.printers[printerID]
	if !exists {
		http.Error(w, "Printer not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(printer)
}
