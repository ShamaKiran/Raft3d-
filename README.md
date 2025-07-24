# Raft3D - Distributed 3D Prints Management using Raft Consensus Algorithm

Raft3D is a distributed backend system for managing 3D print jobs across a cluster of nodes using the Raft consensus algorithm for state replication, fault tolerance, and consistency.

## Overview

- Replaces centralized databases with Raft-based distributed state management.
- Implements leader election, log replication, snapshotting, and failover handling.
- Designed to manage and persist printer, filament, and print job data across nodes.

## Technologies

- Language: Go
- Raft Library: [hashicorp/raft](https://github.com/hashicorp/raft)
- Architecture: Distributed, Leader-based replication
- API Style: RESTful

## Raft Features

- Leader election and re-election on failure
- Persistent log and snapshot support
- Custom Raft FSM for domain objects
- Fault-tolerant across multiple nodes

## Domain Objects (FSM State)

- **Printers** – Unique ID, company, and model
- **Filaments** – Type, color, weight tracking
- **PrintJobs** – References printer & filament, tracks file, weight, and status

## API Endpoints

- `POST /api/v1/printers` – Create printer
- `GET /api/v1/printers` – List printers
- `POST /api/v1/filaments` – Create filament
- `GET /api/v1/filaments` – List filaments
- `POST /api/v1/print_jobs` – Create print job
- `GET /api/v1/print_jobs` – List print jobs
- `POST /api/v1/print_jobs/<id>/status` – Update job status

## Business Logic Rules

- Filament weight reduced only when print job is marked `Done`
- Print job status transitions are strictly validated
- All state updates pass through the Raft log for consistency


