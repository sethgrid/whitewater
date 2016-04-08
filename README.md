# Whitewater

Whitewater is a toy implementation of the Raft algorythm designed as a learning aid for myself.
You can see the implementation in the raft subpackage.
This subpackage demonstrates some key raft concepts such as leadership election, failover, and message replication.
It does not deal with configuration changes like adding a node to the system. 
Many edge cases are not considered as this is not a produciton worthy implementation.

For more information, see the [Raft Paper](https://ramcloud.stanford.edu/raft.pdf).

# Running the Toy Raft Implementation

While you could import the toy implementation and play with it, it is easiest to just run the tests.
```
$ cd raft && go test
```
In a typical test run, you will see output that looks akin to:
```
$ go test
2016/04/08 10:13:14 Node 0 called election, new term: 1
2016/04/08 10:13:15 going to publish to :51091 (id 3)
2016/04/08 10:13:15 going to publish to :51088 (id 0)
2016/04/08 10:13:15 going to publish to :51089 (id 1)
2016/04/08 10:13:15 going to publish to :51092 (id 4)
2016/04/08 10:13:15 going to publish to :51090 (id 2)
2016/04/08 10:13:15 Node 3 (state: 0, leader: false) appending log
2016/04/08 10:13:15 Node 4 (state: 0, leader: false) appending log
2016/04/08 10:13:15 Node 2 (state: 0, leader: false) appending log
2016/04/08 10:13:15 Node 0 (state: 2, leader: true) appending log
2016/04/08 10:13:15 Node 1 (state: 0, leader: false) appending log
2016/04/08 10:13:21 Node 0 called election, new term: 2
2016/04/08 10:13:21 Node 1 called election, new term: 1
2016/04/08 10:13:21 rpc.Serve: accept:accept tcp [::]:51105: use of closed network connection
2016/04/08 10:13:22 Node 1 called election, new term: 1
2016/04/08 10:13:22 Node 3 called election, new term: 1
2016/04/08 10:13:23 Node 2 called election, new term: 1
2016/04/08 10:13:24 Node 4 called election, new term: 1
2016/04/08 10:13:24 Node 4 called election, new term: 1
2016/04/08 10:13:24 Node 3 called election, new term: 1
2016/04/08 10:13:25 Node 2 called election, new term: 1
2016/04/08 10:13:25 Node 0 called election, new term: 1
2016/04/08 10:13:27 Node 2 called election, new term: 1
2016/04/08 10:13:27 going to publish to :51123 (id 4)
2016/04/08 10:13:27 going to publish to :51120 (id 1)
2016/04/08 10:13:27 going to publish to :51121 (id 2)
2016/04/08 10:13:27 going to publish to :51119 (id 0)
2016/04/08 10:13:27 going to publish to :51122 (id 3)
2016/04/08 10:13:27 Node 1 (state: 0, leader: false) appending log
2016/04/08 10:13:27 Node 3 (state: 0, leader: false) appending log
2016/04/08 10:13:27 Node 0 (state: 0, leader: false) appending log
2016/04/08 10:13:27 Node 4 (state: 0, leader: false) appending log
2016/04/08 10:13:27 Node 2 (state: 2, leader: true) appending log
2016/04/08 10:13:27 rpc.Serve: accept:accept tcp [::]:51121: use of closed network connection
2016/04/08 10:13:27 going to publish to :51123 (id 4)
2016/04/08 10:13:27 going to publish to :51120 (id 1)
2016/04/08 10:13:27 going to publish to :51122 (id 3)
2016/04/08 10:13:27 going to publish to :51119 (id 0)
2016/04/08 10:13:27 Node 0 (state: 0, leader: false) appending log
2016/04/08 10:13:27 Node 4 (state: 0, leader: false) appending log
2016/04/08 10:13:27 Node 3 (state: 0, leader: false) appending log
2016/04/08 10:13:27 Node 1 (state: 0, leader: false) appending log
PASS
ok  	github.com/sethgrid/whitewater/raft	19.158s
```
This demonstrates a cluster of 5 nodes starting up, electing a leader, publishing a message to the cluster.
Then the leader node is killed, automatic-reelection occurs and a new message can be published to the cluster.

### Presentation

Intall `present` by running `$ go get code.google.com/p/go.tools/cmd/present`.

Simply run `$ present` from within this directory or the slides directory and go to the url in the output.
For more information on the tool's format, visit https://godoc.org/golang.org/x/tools/present