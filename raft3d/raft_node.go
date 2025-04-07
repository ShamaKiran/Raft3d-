package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

var (
	nodeID   = flag.String("id", "1", "Node ID")
	httpAddr = flag.String("http", "8080", "HTTP server address")
	raftAddr = flag.String("raft", "9000", "Raft communication address")
	peers    = flag.String("peers", "", "Comma-separated list of peer addresses")
)

func setupRaft(nodeID, raftAddr string, peerList []string) (*raft.Raft, error) {
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

	fsm := NewFSM()
	SetFSMInstance(fsm)

	r, err := raft.NewRaft(config, fsm, store, store, snapshotStore, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create Raft instance: %w", err)
	}

	if len(peerList) == 0 || (len(peerList) == 1 && peerList[0] == "") {
		fmt.Println("Bootstrapping Raft cluster...")
		r.BootstrapCluster(raft.Configuration{
			Servers: []raft.Server{
				{ID: raft.ServerID(nodeID), Address: raft.ServerAddress(raftAddr)},
			},
		})
	}

	return r, nil
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

	r, err := setupRaft(*nodeID, *raftAddr, peerList)
	if err != nil {
		log.Fatalf("Failed to start Raft: %v", err)
	}
	raftNode = r // global reference

	http.HandleFunc("/jobs", getJobsHandler)
	http.HandleFunc("/update", updateJobHandler)

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		state := raftNode.State().String()
		leader := raftNode.Leader()
		fmt.Fprintf(w, "Node %s | State: %s | Leader: %s\n", *nodeID, state, leader)
	})

	log.Fatal(http.ListenAndServe(":"+*httpAddr, nil))
}
