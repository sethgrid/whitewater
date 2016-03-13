package raft

import "testing"

func TestSend(t *testing.T) {
	t.Skip()
	Debug = false

	clusterSize := 5
	debug("test > setting up cluster")
	cluster, err := NewCluster(clusterSize)
	if err != nil {
		t.Fatal("unable to create new cluster ", err)
	}

	debug("test > about to send...")
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

func TestLeaderElection(t *testing.T) {
	Debug = true

	clusterSize := 5
	debug("test > setting up cluster")
	cluster, err := NewCluster(clusterSize)
	if err != nil {
		t.Fatal("unable to create new cluster ", err)
	}

	debug("test > getting leader...")
	leader := cluster.CurLeader()
	originalLeaderID := leader.ID

	debug("test > stopping leader...")
	err = cluster.StopNode(originalLeaderID)
	if err != nil {
		t.Fatal("unable to stop node ", err)
	}

	debug("test > getting new leader...")
	newLeader := cluster.CurLeader()
	newLeaderID := newLeader.ID

	if newLeaderID == originalLeaderID {
		t.Errorf("Got id %d, want a different than id %d for new leader", newLeaderID, originalLeaderID)
	}
}

func TestDetermineMajority(t *testing.T) {
	cases := []struct {
		count    int
		majority int
	}{
		{count: 10, majority: 6},
		{count: 11, majority: 6},
	}

	for _, test := range cases {
		if got, want := determineMajority(test.count), test.majority; got != want {
			t.Errorf("got %d, want %d for majority of %d", got, want, test.count)
		}
	}
}
