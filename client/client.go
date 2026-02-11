package main

import (
	"fmt"
	"math/rand/v2"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"

	"gossip/shared"
)

const (
	MIN_NODE  = 1
	MAX_NODES = 8

	// times in seconds
	MSG_TIME  = 2
	KILL_TIME = 60
	DEAD_TIME = 6

	// times in milliseconds
	HEARTBEAT_TIME = 50
	CANDIDATE_TIME = 150
	ELECTION_TIME  = 50
)

var self_node shared.Node
var start_time = time.Now()
var leader shared.Leader
var candidate bool = false
var curr_term = 0
var voted_for = 0

// Send the current membership table to a neighboring node with the provided ID
func sendMessage(server *rpc.Client, id int, membership shared.Membership) {
	var req = shared.Request{
		ID:    id,
		Table: membership,
	}
	res := false
	server.Call("Requests.Add", req, &res)
	if !res {
		fmt.Printf("Error sending message to %d\n", id)
	}
}

// Read incoming messages from other nodes
func readMessages(server *rpc.Client, id int, membership *shared.Membership) *shared.Membership {
	var table shared.Membership
	if err := server.Call("Requests.Listen", id, &table); err != nil {
		return membership
	}

	result := shared.CombineTables(membership, &table)

	return result
}

func calcTime() float64 {
	return float64(time.Since(start_time)) / 1000000000
}

var wg = &sync.WaitGroup{}

func main() {
	// Connect to RPC server
	server, _ := rpc.DialHTTP("tcp", "localhost:9005")

	args := os.Args[1:]

	// Get ID from command line argument
	if len(args) == 0 {
		fmt.Println("No args given")
		return
	}
	id, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Println("Found Error", err)
	}

	if id < MIN_NODE || id > MAX_NODES {
		fmt.Println("Invalid ID entered. Please choose a number from 1 - 8.")
		return
	}

	currTime := calcTime()
	// Construct self
	self_node = shared.Node{ID: id, Hbcounter: 0, Time: currTime, Alive: true}
	var self_node_response shared.Node // Allocate space for a response to overwrite this

	// Add node with input ID
	if err := server.Call("Membership.Add", self_node, &self_node_response); err != nil {
		fmt.Println("Error:2 Membership.Add()", err)
		return
	} else {
		fmt.Printf("Success: Node created with id= %d\n", id)
	}

	if err := server.Call("Leader.Get", leader, &leader); err != nil {
		fmt.Println("No leader found")
	} else {
		fmt.Printf("Leader %d found. Waiting for heartbeats\n", leader.NodeID)
	}

	neighbors := self_node.InitializeNeighbors(id)

	membership := shared.NewMembership()
	membership.Add(self_node, &self_node)

	time.AfterFunc(time.Millisecond*HEARTBEAT_TIME, func() { heartbeat(server, &self_node, membership, id) })
	time.AfterFunc(time.Second*MSG_TIME, func() { shareTables(server, neighbors, membership, id) })
	time.AfterFunc(time.Second*time.Duration(KILL_TIME), func() { killNode(id) })

	wg.Add(1)
	wg.Wait()
}

func heartbeat(server *rpc.Client, node *shared.Node, membership *shared.Membership, id int) {
	var self_node_response shared.Node
	node.Hbcounter++
	node.Time = calcTime()

	payload := *node
	membership.Update(payload, &self_node_response)

	server.Call("Membership.Update", payload, &self_node_response)

	if !membership.Members[leader.NodeID].Alive {
		var election = shared.Election{}
		if err := server.Call("Election.Get", leader, &election); err == nil {
			if election.Term > curr_term {
				curr_term = election.Term
				voted_for = 0
			}
			if len(election.Results) > 0 && curr_term == election.Term && candidate == false && voted_for == 0 {
				vote := 0
				reply := false
				for key := range election.Results {
					if membership.Members[key].Alive {
						vote = key
						break
					}
				}
				accepted_leader := shared.Leader{
					NodeID: vote,
					Term:   election.Term,
				}
				server.Call("Election.SendVote", accepted_leader, &reply)
				if reply {
					voted_for = vote
				}
			}
		}
	}

	time.AfterFunc(time.Millisecond*HEARTBEAT_TIME, func() { heartbeat(server, node, membership, id) })
}

func shareTables(server *rpc.Client, neighbors [2]int, membership *shared.Membership, id int) {
	for _, n := range neighbors {
		sendMessage(server, n, *membership)
	}
	new_membership := readMessages(server, self_node.ID, membership)

	for k, v := range new_membership.Members {
		if data, ok := membership.Members[k]; ok {
			if data.Hbcounter == v.Hbcounter {
				if v.Time < calcTime()-DEAD_TIME {
					v.Alive = false
					var reply shared.Node
					server.Call("Membership.Update", v, &reply)
				}
			} else {
				v.Time = calcTime()
			}
		}
		new_membership.Members[k] = v
	}

	*membership = *new_membership
	//fmt.Println("-----   Membership Table   -----")
	//printMembership(*membership)

	server.Call("Leader.Get", leader, &leader)
	if !candidate && leader.NodeID == 0 || (leader.NodeID != 0 && !membership.Members[leader.NodeID].Alive) {
		// start election
		timeout := time.Duration(rand.IntN(CANDIDATE_TIME) + CANDIDATE_TIME)
		time.AfterFunc(time.Millisecond*timeout, func() { tryCandidate(server, membership) })
	}

	time.AfterFunc(time.Second*MSG_TIME, func() { shareTables(server, neighbors, membership, id) })
}

func tryCandidate(server *rpc.Client, membership *shared.Membership) {

	server.Call("Leader.Get", leader, &leader)
	if leader.NodeID == 0 || (leader.NodeID != 0 && !membership.Members[leader.NodeID].Alive) {

		candidate = true
		// do candidate stuff
		proposal := shared.Leader{
			NodeID: self_node.ID,
			Term:   leader.Term + 1,
		}
		reply := 0
		curr_term = leader.Term + 1
		voted_for = self_node.ID
		server.Call("Election.RequestVote", proposal, &reply)
		// set timer, on timeout check election
		timeout := time.Duration(rand.IntN(ELECTION_TIME))
		time.AfterFunc(time.Millisecond*timeout, func() { countVotes(server) })
	} else {
		candidate = false
		return
	}
}

func countVotes(server *rpc.Client) {
	var election = shared.Election{}
	reply := false
	server.Call("Election.Get", leader, &election)
	fmt.Printf("----   Election Results For Term %d  ----\n", election.Term)
	for k, v := range election.Results {
		fmt.Printf("Votes for Node %d: %d\n", k, v)
	}
	fmt.Println()

	if election.Results[self_node.ID] >= (MAX_NODES/2 + 1) {
		// clear election
		new_leader := shared.Leader{
			NodeID: self_node.ID,
			Term:   election.Term,
		}
		server.Call("Leader.Update", &new_leader, &reply)
		server.Call("Election.Clear", election.Term, &reply)
	} else {
		candidate = false
		server.Call("Election.Drop", self_node.ID, &reply)
		voted_for = 0
	}
}

func killNode(id int) {
	if id == 4 {
		fmt.Println("Node", id, "FAILED")
		os.Exit(1)
	}
}

func printMembership(m shared.Membership) {
	for i := MIN_NODE; i <= MAX_NODES; i++ {
		if n, ok := m.Members[i]; ok {
			status := "is Alive"
			if !n.Alive {
				status = "is Dead"
			}
			fmt.Printf("Node %d has hb %d, time %.3f and %s\n", n.ID, n.Hbcounter, n.Time, status)
		}
	}
	fmt.Println("")
}
