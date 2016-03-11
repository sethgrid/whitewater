package raft

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"strconv"
	"strings"
	"sync"
)

// Debug, when set, will give detailed logs of what is going on
var Debug bool

// Reply states if the previous message was applied
type Reply struct {
	Applied bool
}

// Cluster is our cluster
type Cluster struct {
	// curLeader is an index in Nodes
	curLeader int
	Nodes     []*Node
}

// CurLeader returns the leader node
// TODO: handle when there is no leader
func (c Cluster) CurLeader() Node {
	return Node{
		Port:    c.Nodes[c.curLeader].Port,
		Cluster: c.Nodes[c.curLeader].Cluster,
		ID:      c.Nodes[c.curLeader].ID,
		Log:     c.Nodes[c.curLeader].Log,
	}
}

// NewCluster returns a cluster of a given size
func NewCluster(count int) (Cluster, error) {
	nodes := make([]*Node, count)
	var err error
	for i := 0; i < count; i++ {
		debug("creating node...")
		nodes[i], err = NewNode(i)
		if err != nil {
			log.Println("unable to create node", err)
			return Cluster{}, err
		}
		debug("node created!")
	}

	cluster := Cluster{Nodes: nodes, curLeader: 0}
	for _, node := range cluster.Nodes {
		node.Cluster = cluster
	}

	return cluster, err
}

// Node is a single running instance
type Node struct {
	ID   int
	Port int
	Log  []LogEntry

	Cluster Cluster
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

// publish should be called on the leader, and the leader then sends to the other nodes
func (n *Node) publish(msg []byte) error {
	// TODO - make sure this is the leader
	wg := sync.WaitGroup{}
	for _, node := range n.Cluster.Nodes {
		wg.Add(1)
		go func(node *Node) {
			defer wg.Done()
			debug("going to publish to :%d (id %d)", node.Port, node.ID)
			// TODO - pass a `message` including term and such

			client, err := rpc.Dial("tcp", fmt.Sprintf("0.0.0.0:%d", node.Port, node.ID))
			if err != nil {
				log.Println("unable to dial client ", err)
				return
			}

			reply := Reply{}
			data := LogEntry{Term: 0, Index: 0, Data: msg}
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
	// TODO: lock

	n.Log = append(n.Log, msg)
	reply.Applied = true
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

	n := &Node{ID: ID, Port: port, Log: make([]LogEntry, 0)}

	s := rpc.NewServer()
	s.Register(n)
	go s.Accept(l)
	// rpc.Register(n)
	// // rpc.RegisterName(fmt.Sprintf("AppendEntry_%d", ID), n)
	// go rpc.Accept(l)

	return n, nil
}

// LogEntry represents a raft log entry
type LogEntry struct {
	Term  int
	Index int
	Data  []byte
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
