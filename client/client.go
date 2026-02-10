package main

import (
	"fmt"
	"net/rpc"
	"os"
	"strconv"
	"sync"
	"time"

	"gossip/shared"
)

const (
	MIN_NODE   = 1
	MAX_NODES  = 8
	X_TIME     = 1
	Y_TIME     = 2
	Z_TIME_MAX = 100
	Z_TIME_MIN = 10
	DEAD_TIME  = 6
)

var self_node shared.Node
var start_time = time.Now()

// Send the current membership table to a neighboring node with the provided ID
func sendMessage(server *rpc.Client, id int, membership shared.Membership) {
	var req = shared.Request{
		ID:    id,
		Table: membership,
	}
	res := false
	fmt.Printf("Sending msg to %d\n", id)
	server.Call("Requests.Add", req, &res)
	if !res {
		fmt.Printf("Error sending message to %d\n", id)
	}
}

// Read incoming messages from other nodes
func readMessages(server *rpc.Client, id int, membership *shared.Membership) *shared.Membership {
	var table shared.Membership
	fmt.Printf("Listening for msg from %d\n", id)
	if err := server.Call("Requests.Listen", id, &table); err != nil {
		fmt.Println("Oops")
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
	//rand.Seed(time.Now().UnixNano())
	Z_TIME := 60

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

	neighbors := self_node.InitializeNeighbors(id)
	fmt.Println("Neighbors:", neighbors)

	membership := shared.NewMembership()
	membership.Add(self_node, &self_node)

	//sendMessage(*server, neighbors[rand.Intn(2)], *membership)

	// crashTime := self_node.CrashTime()

	time.AfterFunc(time.Second*X_TIME, func() { runAfterX(server, &self_node, membership, id) })
	time.AfterFunc(time.Second*Y_TIME, func() { runAfterY(server, neighbors, membership, id) })
	time.AfterFunc(time.Second*time.Duration(Z_TIME), func() { runAfterZ(id) })

	wg.Add(1)
	wg.Wait()
}

func runAfterX(server *rpc.Client, node *shared.Node, membership *shared.Membership, id int) {
	//TODO : increment heartbeat, listen
	var self_node_response shared.Node
	node.Hbcounter++
	node.Time = calcTime()

	payload := *node
	membership.Update(payload, &self_node_response)

	server.Call("Membership.Update", payload, &self_node_response)
	fmt.Printf("Node %d has hb %d, time %.3f\n", node.ID, node.Hbcounter, node.Time)
	time.AfterFunc(time.Second*X_TIME, func() { runAfterX(server, node, membership, id) })
}

func runAfterY(server *rpc.Client, neighbors [2]int, membership *shared.Membership, id int) {
	for _, n := range neighbors {
		sendMessage(server, n, *membership)
	}
	new_membership := readMessages(server, self_node.ID, membership)

	for k, v := range new_membership.Members {
		if data, ok := membership.Members[k]; ok {
			if data.Hbcounter == v.Hbcounter {
				if v.Time < calcTime()-DEAD_TIME {
					v.Alive = false
				}
			} else {
				v.Time = calcTime()
			}
		}
		new_membership.Members[k] = v
	}

	*membership = *new_membership
	fmt.Println("-----   Membership Table   -----")
	printMembership(*membership)
	time.AfterFunc(time.Second*Y_TIME, func() { runAfterY(server, neighbors, membership, id) })
}

func runAfterZ(id int) {
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
