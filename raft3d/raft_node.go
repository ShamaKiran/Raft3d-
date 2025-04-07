package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

// Command-line flags
var (
	nodeID   = flag.String("id", "1", "Node ID")
	httpAddr = flag.String("http", "8080", "HTTP server address")
	raftAddr = flag.String("raft", "9000", "Raft communication address")
	peers    = flag.String("peers", "", "Comma-separated list of peer addresses")
)

// State Machine for Raft
type RaftNode struct {
	mu    sync.Mutex
	raft  *raft.Raft
	store *raftboltdb.BoltStore
	data  map[string]string // Simple key-value store
}

// Raft FSM (Finite State Machine) Implementation
func (rn *RaftNode) Apply(log *raft.Log) interface{} {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	var cmd map[string]string
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal log data: %v", err)
	}

	for key, value := range cmd {
		rn.data[key] = value
	}
	return nil
}

func (rn *RaftNode) Snapshot() (raft.FSMSnapshot, error) {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	snapshot := make(map[string]string)
	for key, value := range rn.data {
		snapshot[key] = value
	}

	return &snapshotStore{data: snapshot}, nil
}

func (rn *RaftNode) Restore(rc io.ReadCloser) error {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	data := make(map[string]string)
	if err := json.NewDecoder(rc).Decode(&data); err != nil {
		return err
	}
	rn.data = data
	return nil
}

// Snapshot store for Raft FSM
type snapshotStore struct {
	data map[string]string
}

func (s *snapshotStore) Persist(sink raft.SnapshotSink) error {
	data, err := json.Marshal(s.data)
	if err != nil {
		return err
	}

	if _, err := sink.Write(data); err != nil {
		_ = sink.Cancel()
		return err
	}

	return sink.Close()
}

func (s *snapshotStore) Release() {}

// Initialize Raft
func setupRaft(nodeID, raftAddr string, peerList []string) (*RaftNode, error) {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)

	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:"+raftAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Raft address: %w", err)
	}

	store, err := raftboltdb.NewBoltStore(fmt.Sprintf("raft_node_%s.db", nodeID))
	if err != nil {
		return nil, fmt.Errorf("failed to create BoltDB store: %w", err)
	}

	snapshotStore := raft.NewInmemSnapshotStore()
	transport, err := raft.NewTCPTransport(addr.String(), addr, 3, time.Second, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	node := &RaftNode{store: store, data: make(map[string]string)}

	raftInstance, err := raft.NewRaft(config, node, store, store, snapshotStore, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create Raft instance: %w", err)
	}
	node.raft = raftInstance

	// Bootstrap the cluster if this is the first node
	if len(peerList) == 0 || peerList[0] == "" {
		fmt.Println("Bootstrapping Raft cluster...")
		raftInstance.BootstrapCluster(raft.Configuration{
			Servers: []raft.Server{
				{ID: raft.ServerID(nodeID), Address: raft.ServerAddress(raftAddr)},
			},
		})
	}

	return node, nil
}

func main() {
	flag.Parse()

	fmt.Printf("Starting Raft node %s...\n", *nodeID)
	fmt.Printf("HTTP API running at :%s\n", *httpAddr)
	fmt.Printf("Raft communication on port %s\n", *raftAddr)

	peerList := strings.Split(*peers, ",")
	if len(peerList) > 0 && peerList[0] != "" {
		fmt.Println("Known peers:", peerList)
	} else {
		fmt.Println("No known peers provided.")
	}

	node, err := setupRaft(*nodeID, *raftAddr, peerList)
	if err != nil {
		log.Fatalf("Failed to start Raft: %v", err)
	}

	// HTTP Handlers
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		state := node.raft.State().String()
		leader := node.raft.Leader()
		fmt.Fprintf(w, "Node %s | State: %s | Leader: %s\n", *nodeID, state, leader)
	})

	http.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		if node.raft.State() != raft.Leader {
			http.Error(w, "Only the leader can handle writes", http.StatusForbidden)
			return
		}

		key := r.URL.Query().Get("key")
		value := r.URL.Query().Get("value")
		if key == "" || value == "" {
			http.Error(w, "Missing key or value", http.StatusBadRequest)
			return
		}

		cmd := map[string]string{key: value}
		data, _ := json.Marshal(cmd)

		future := node.raft.Apply(data, time.Second)
		if err := future.Error(); err != nil {
			http.Error(w, fmt.Sprintf("Raft apply failed: %v", err), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "Key '%s' set to '%s'\n", key, value)
	})

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		node.mu.Lock()
		value, exists := node.data[key]
		node.mu.Unlock()

		if !exists {
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}
		fmt.Fprintf(w, "Key '%s' = '%s'\n", key, value)
	})

	log.Fatal(http.ListenAndServe(":"+*httpAddr, nil))
}
