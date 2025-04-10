package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

var raftNode *raft.Raft

func main() {
	// CLI flags
	id := flag.String("id", "", "Node ID")
	httpAddr := flag.String("http", ":8080", "HTTP server bind address")
	raftBind := flag.String("raft", "127.0.0.1:9000", "Raft bind address")
	joinAddr := flag.String("join", "", "Address of leader to join (host:port)")
	flag.Parse()

	// Raft config
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(*id)
	config.Logger = hclog.New(&hclog.LoggerOptions{
		Name:  "raft",
		Level: hclog.LevelFromString("INFO"),
	})

	// Raft transport
	addr, err := net.ResolveTCPAddr("tcp", *raftBind)
	if err != nil {
		log.Fatalf("Failed to resolve TCP address: %v", err)
	}

	transport, err := raft.NewTCPTransport(*raftBind, addr, 3, 10*time.Second, os.Stdout)
	if err != nil {
		log.Fatalf("Failed to create transport: %v", err)
	}

	// Raft stores
	store, err := raftboltdb.NewBoltStore(fmt.Sprintf("raft-%s.db", *id))
	if err != nil {
		log.Fatalf("Failed to create bolt store: %v", err)
	}
	snapshots := raft.NewInmemSnapshotStore()
	logStore := store

	// FSM
	raftNode, err = raft.NewRaft(config, &FSM{}, store, logStore, snapshots, transport)
	if err != nil {
		log.Fatalf("Failed to create raft node: %v", err)
	}

	// Bootstrap or Join
	if *joinAddr == "" {
		config := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raft.ServerID(*id),
					Address: transport.LocalAddr(),
				},
			},
		}
		raftNode.BootstrapCluster(config)
		log.Println("Bootstrapped self as leader")
	} else {
		// Join another node
		url := fmt.Sprintf("http://%s/join?id=%s&addr=%s", *joinAddr, *id, *raftBind)
		resp, err := http.Post(url, "", nil)
		if err != nil {
			log.Fatalf("Failed to join cluster: %v", err)
		}
		defer resp.Body.Close()
		log.Printf("Sent join request to leader at %s", *joinAddr)
	}

	// HTTP Handlers
	http.HandleFunc("/join", handleJoin)
	http.HandleFunc("/status", handleStatus)

	log.Printf("HTTP server listening on %s", *httpAddr)
	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}

// FSM implementation
type FSM struct{}

func (f *FSM) Apply(*raft.Log) interface{}         { return nil }
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) { return &fsmSnapshot{}, nil }
func (f *FSM) Restore(io.ReadCloser) error         { return nil }

type fsmSnapshot struct{}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	sink.Cancel()
	return nil
}
func (f *fsmSnapshot) Release() {}

// /join handler
func handleJoin(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	addr := r.URL.Query().Get("addr")

	if id == "" || addr == "" {
		http.Error(w, "Missing id or addr", http.StatusBadRequest)
		return
	}

	f := raftNode.AddVoter(raft.ServerID(id), raft.ServerAddress(addr), 0, 0)
	if err := f.Error(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "Node %s at %s joined successfully\n", id, addr)
}

// /status handler
func handleStatus(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "State: %s\n", raftNode.State())
	fmt.Fprintf(w, "Leader: %s\n", raftNode.Leader())
}
