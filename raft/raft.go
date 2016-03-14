package raft

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/rpc"
	"strconv"
	"strings"
	"sync"
	"time"
)

type state int

// TriggerElectionTimeoutCap is the max time when a node decides to run for office
// var TriggerElectionTimeoutCap = 150 * time.Millisecond
var TriggerElectionTimeoutCap = 7 * time.Second

// TriggerElectionTimeoutMin is the minimum amount of time before a node runs for office
// var TriggerElectionTimeoutMin = 75 * time.Millisecond
var TriggerElectionTimeoutMin = 2 * time.Second

// ElectionTimeout the time that a node will wait for votes on its nomination
// This should be about an order of magnitude greater than the TriggerElectionTimeoutCap
var ElectionTimeout = TriggerElectionTimeoutCap * 10

// ErrElectionTimeout is returned when an election times out
var ErrElectionTimeout = fmt.Errorf("election timed out")

const (
	// NullVote represents when no vote has been cast by a node
	NullVote = -1

	// FollowerState is the default state for any node
	FollowerState = 0

	// CandidateState is when the node is running for office to be leader
	CandidateState = 1

	// LeaderState lets a node know it is a leader
	LeaderState = 2
)

// Debug, when set, will give detailed logs of what is going on
var Debug bool

func init() {
	// TODO: find that article that said not to do this, and let package caller set seed
	rand.Seed(time.Now().UnixNano())
}

// LogEntry represents a raft log entry
type LogEntry struct {
	LeaderID int
	Term     int
	Index    int
	Data     []byte
}

// Reply states if the previous message was applied
type Reply struct {
	Applied bool
}

// Cluster is our cluster
type Cluster struct {
	sync.Mutex
	// curLeader is an index in Nodes
	curLeader int
	Nodes     []*Node
}

// CurLeader returns the leader node
func (c *Cluster) CurLeader() *Node {
	// TODO: should use atomic for leaderPoll
	leaderAvailable := false
	for !leaderAvailable {
		if c.curLeader == NullVote {
			debug("no leader, will poll")
		} else if !canPingPort(c.Nodes[c.curLeader].Port) {
			debug("no leader, ID %d is dead", c.curLeader)
		} else {
			leaderAvailable = true
		}

		if !leaderAvailable {
			// if there is still no leader, wait a bit
			<-time.After(500 * time.Millisecond)
		}
	}

	return c.Nodes[c.curLeader]
}

// NewCluster returns a cluster of a given size
func NewCluster(count int) (*Cluster, error) {
	nodes := make([]*Node, count)
	var err error
	for i := 0; i < count; i++ {
		debug("creating node...")
		nodes[i], err = NewNode(i)
		if err != nil {
			log.Println("unable to create node", err)
			return nil, err
		}
		debug("node created!")
	}

	cluster := &Cluster{Nodes: nodes, curLeader: NullVote}
	for _, node := range cluster.Nodes {
		node.Cluster = cluster
	}

	return cluster, err
}

// Send publishes a message to the cluster
func (c *Cluster) Send(msg []byte) error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
	}()

	debug("sending...")
	leader := c.CurLeader()
	err := leader.publish(msg)
	if err != nil {
		log.Println("unable to publish to leader ", err)
		return err
	}

	return nil
}

// StopNode turns off a node
func (c *Cluster) StopNode(id int) error {
	for _, node := range c.Nodes {
		if node.ID == id {
			node.stop()
			return nil
		}
	}
	return nil
}

// Node is a single running instance
type Node struct {
	mtx sync.Mutex

	State       state
	ID          int
	Port        int
	Log         []LogEntry
	CurrentTerm int
	VotedFor    int
	CommitIndex int
	LastApplied int
	NextIndex   int
	MatchIndex  int

	Cluster *Cluster

	close          func() error
	heartbeat      chan struct{}
	closeElection  chan struct{}
	electionCalled bool
}

func (n *Node) stop() error {
	return n.close()
}

// publish should be called on the leader, and the leader then sends to the other nodes
func (n *Node) publish(msg []byte) error {
	// TODO - make sure this is the leader
	wg := sync.WaitGroup{}
	for _, node := range n.Cluster.Nodes {
		wg.Add(1)
		go func(node *Node) {
			defer wg.Done()
			log.Printf("going to publish to :%d (id %d)", node.Port, node.ID)
			// TODO - pass a `message` including term and such

			client, err := rpc.Dial("tcp", fmt.Sprintf("0.0.0.0:%d", node.Port))
			if err != nil {
				log.Println("unable to dial client ", err)
				return
			}

			reply := Reply{}
			data := LogEntry{LeaderID: n.Cluster.curLeader, Term: node.CurrentTerm, Index: len(node.Log) + 1, Data: msg}
			debug("calling AppendEntry...")
			err = client.Call("Node.AppendEntry", data, &reply)
			// err = client.Call(fmt.Sprintf("AppendEntry_%d.", node.ID), data, &reply)
			if err != nil {
				log.Println("unable to call client ", err)
			}
			debug("Log Applied?: %v", reply.Applied)
		}(node)
	}
	wg.Wait()
	return nil
}

// AppendEntry is the RPC call the leader will make
func (n *Node) AppendEntry(msg LogEntry, reply *Reply) error {
	debug("node %d AppendEntry %q", n.ID, string(msg.Data))

	// don't send a heartbeat to yourself if you're the leader, it confuses elections
	// TODO: isolate this odd behavior in the election
	if msg.LeaderID != n.ID {
		n.heartbeat <- struct{}{}
	}

	// n.mtx.Lock()
	// defer n.mtx.Unlock()

	if msg.Term > n.CurrentTerm {
		n.CurrentTerm = msg.Term
		// need lock
		log.Printf("node %d reverting to follower state", n.ID)
		n.State = FollowerState
	}

	if msg.Term < n.CurrentTerm {
		// ignore request
		log.Printf("append entry term %d ignored, current term: %d", msg.Term, n.CurrentTerm)
		return nil
	}

	// if data in nil, then it is only a heartbeat
	if msg.Data != nil {
		log.Printf("Node %d (state: %d, leader: %v) appending log", n.ID, n.State, n.State == LeaderState)
		n.Log = append(n.Log, msg)
	}
	reply.Applied = true
	return nil
}

// Vote is supplied by each node during an election
type Vote struct {
	Term         int
	CandidateID  int
	LastLogIndex int
	LastLogTerm  int
}

// VoteReply is returned to candidates
type VoteReply struct {
	Term        int
	VoteGranted bool
}

// RequestVote is the rpc call for requesting a vote
func (n *Node) RequestVote(vote Vote, reply *VoteReply) error {
	n.mtx.Lock()
	defer n.mtx.Unlock()

	reply.Term = n.CurrentTerm
	if vote.Term > n.CurrentTerm {
		debug("node %d (term %d) got request for term %d, resetting vote", n.ID, n.CurrentTerm, vote.Term)
		n.VotedFor = NullVote
	}
	if vote.Term < n.CurrentTerm || vote.LastLogIndex < n.CommitIndex || n.VotedFor != NullVote {
		debug("node %d (term %d) regected vote for node %d term %d (current vote: node %d)", n.ID, n.CurrentTerm, vote.CandidateID, vote.Term, n.VotedFor)
		reply.VoteGranted = false
	} else if n.VotedFor == NullVote {
		n.VotedFor = vote.CandidateID
		debug("node %d granted vote for node %d in term %d", n.ID, vote.CandidateID, vote.Term)
		reply.VoteGranted = true
	} else {
		log.Printf("node %d - hanging chad - how did this happen? %#v %#v", n.ID, n, vote)
	}

	debug("node %d got request vote from %d, granted %v", n.ID, vote.CandidateID, reply.VoteGranted)
	return nil
}

func (n *Node) callElection() error {
	n.mtx.Lock()
	defer n.mtx.Unlock()

	n.State = CandidateState
	n.electionCalled = true
	tally := make(chan struct{})
	votes := 1 // votes for itself, start at 1
	majority := determineMajority(len(n.Cluster.Nodes))
	n.closeElection = make(chan struct{})
	n.CurrentTerm++
	log.Printf("Node %d called election, new term: %d", n.ID, n.CurrentTerm)
	for _, node := range n.Cluster.Nodes {
		if node.ID == n.ID {
			// node already voted for itself
			continue
		}
		go func(node *Node) {
			debug("node %d calling an election to %d ...", n.ID, node.ID)
			client, err := rpc.Dial("tcp", fmt.Sprintf("0.0.0.0:%d", node.Port))
			if err != nil {
				log.Println("unable to dial client ", err)
				return
			}

			vote := Vote{
				Term:         n.CurrentTerm,
				CandidateID:  n.ID,
				LastLogIndex: n.CommitIndex,
				LastLogTerm:  n.CurrentTerm,
			}
			reply := VoteReply{}

			err = client.Call("Node.RequestVote", vote, &reply)
			if err != nil {
				log.Println("unable to call client ", err)
			}

			if reply.VoteGranted {
				tally <- struct{}{}
			}
		}(node)
	}

	timeout := time.After(ElectionTimeout)

	for {
		select {
		case <-tally:
			votes++
			if votes >= majority {
				debug("node %d is now leader", n.ID)
				n.State = LeaderState
				// n.CurrentTerm++
				n.CommitIndex = 0
				n.LastApplied = 0
				n.MatchIndex = 0
				n.NextIndex = 1
				// lock
				n.Cluster.curLeader = n.ID

				// TODO: not sure what to do with the error
				err := n.sendHeartbeat()
				if err != nil {
					log.Println("** ERROR ** heartbeat ", err)
				}
				return nil
			}
		case <-n.closeElection:
			debug("node %d election closed", n.ID)
			return nil
		case <-timeout:
			debug("node %d election timed out", n.ID)
			return ErrElectionTimeout
		}
	}
}

func (n *Node) sendHeartbeat() error {
	debug("node %d is sending a heartbeat", n.ID)
	data := LogEntry{}
	data.LeaderID = n.Cluster.curLeader
	data.Index = n.NextIndex
	data.Term = n.CurrentTerm
	data.Data = nil

	reply := Reply{}
	err := n.AppendEntry(data, &reply)
	if err != nil {
		log.Println("unable to send heartbeat ", err)
		return err
	}

	if !reply.Applied {
		return fmt.Errorf("hearbeat rejected")
	}

	return nil
}

// NewNode creates a node running on a random port
func NewNode(ID int) (*Node, error) {
	l, err := net.Listen("tcp", ":0")

	if err != nil {
		log.Println("unable to create listener", err)
		return nil, err
	}

	port := addrPort(l.Addr().String())
	heartbeat := make(chan struct{})

	n := &Node{ID: ID, Port: port, Log: make([]LogEntry, 0), VotedFor: NullVote, heartbeat: heartbeat}

	s := rpc.NewServer()
	s.Register(n)
	go s.Accept(l)

	// make sure periodic heartbeats go out from the leader to ensure they don't get deposed for no reason
	go func() {
		for {
			select {
			case <-time.After(TriggerElectionTimeoutMin / 2):
				if n.State == LeaderState {
					n.sendHeartbeat()
				}
			}
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovered in f", r)
			}
		}()
		for {
			max := TriggerElectionTimeoutCap - TriggerElectionTimeoutMin
			triggerTimeout := TriggerElectionTimeoutMin + time.Duration(rand.Intn(int(max/1e6)))*time.Millisecond
			debug("node %d will call an election in %s if nothing comes in", ID, triggerTimeout.String())
			select {
			case <-heartbeat:
				debug("node %d got a heartbeat", n.ID)
				n.State = FollowerState // TODO: lock / atomic
				if n.electionCalled {
					debug("node %d got heartbeat during election, closing election", n.ID)
					close(n.closeElection)
				}
				// TODO: elections are not working
			case <-time.After(triggerTimeout + 5*time.Second):
				go func() {
					debug("node %d calling an election", n.ID)
					err := n.callElection()
					if err == ErrElectionTimeout {
						n.CurrentTerm++
					}
				}()
			}
		}
	}()

	n.close = l.Close

	return n, nil
}

// addrPort expects a net.Addr.String() format of `[::]:9000` and will return 9000
func addrPort(addr string) int {
	s := strings.Replace(addr, "[::]:", "", 1)
	p, err := strconv.Atoi(s)
	if err != nil {
		log.Println("error obtaining port ", err)
	}
	return p
}

func debug(format string, v ...interface{}) {
	if Debug {
		log.Printf(format, v...)
	}
}

func determineMajority(count int) int {
	return count/2 + 1
}

func canPingPort(port int) bool {
	// heh. Sounds like "con-air". Nick Cage ftw.
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("[0.0.0.0]:%d", port), 1*time.Second)
	if err != nil {
		log.Printf("unable to ping leader on port %d: %v", port, err)
		return false
	}
	conn.Close()
	return true
}
