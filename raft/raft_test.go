package raft

import (
	"log"
	"testing"
)

func TestSend(t *testing.T) {
	Debug = true

	clusterSize := 5
	log.Println("test > setting up cluster")
	cluster, err := NewCluster(clusterSize)
	if err != nil {
		t.Fatal("unable to create new cluster", err)
	}

	log.Println("test > about to send...")
	err = cluster.Send([]byte("command 1"))
	if err != nil {
		t.Fatal("unable to send data to leader", err)
	}

	for _, node := range cluster.Nodes {
		if len(node.Log) != 1 {
			t.Errorf("node %d got %d entry/entries, want %d", node.ID, len(node.Log), 1)
		}
	}
}
