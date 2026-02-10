package shared

import (
	"errors"
	"fmt"
	"maps"
	"math/rand"
)

const (
	MAX_NODES = 8
)

// Node struct represents a computing node.
type Node struct {
	ID        int
	Hbcounter int
	Time      float64
	Alive     bool
}

// Generate random crash time from 10-60 seconds
func (n Node) CrashTime() int {
	//rand.Seed(time.Now().UnixNano())
	//max := 60
	//min := 10
	//return rand.Intn(max-min) + min  // use for random crash timer
	return 60 // just use 60 for crash timer
}

func (n Node) InitializeNeighbors(id int) [2]int {
	neighbor1 := id - 1
	if neighbor1 < 1 {
		neighbor1 = MAX_NODES
	}
	neighbor2 := (id + 1) % MAX_NODES
	return [2]int{neighbor1, neighbor2}
}

func RandInt() int {
	//rand.Seed(time.Now().UnixNano())
	return rand.Intn(MAX_NODES-1+1) + 1
}

/*---------------*/

// Membership struct represents participanting nodes
type Membership struct {
	Members map[int]Node
}

// Returns a new instance of a Membership (pointer).
func NewMembership() *Membership {
	return &Membership{
		Members: make(map[int]Node),
	}
}

// Adds a node to the membership list.
func (m *Membership) Add(payload Node, reply *Node) error {
	if _, ok := m.Members[payload.ID]; ok {
		return errors.New("ID already exists")
	}
	m.Members[payload.ID] = payload
	fmt.Printf("Node %d added!\n", payload.ID)
	return nil
}

// Updates a node in the membership list.
func (m *Membership) Update(payload Node, reply *Node) error {
	if _, ok := m.Members[payload.ID]; !ok {
		return errors.New("ID does not exist")
	}
	m.Members[payload.ID] = payload
	*reply = payload

	return nil
}

// Returns a node with specific ID.
func (m *Membership) Get(payload int, reply *Node) error {
	if _, ok := m.Members[payload]; ok {
		temp := m.Members[payload]
		reply = &temp
		return nil
	}
	return errors.New("ID not found in Membership Table")
}

/*---------------*/

// Request struct represents a new message request to a client
type Request struct {
	ID    int
	Table Membership
}

// Requests struct represents pending message requests
type Requests struct {
	Pending map[int]Membership
}

// Returns a new instance of a Membership (pointer).
func NewRequests() *Requests {
	//TODO
	return &Requests{
		Pending: make(map[int]Membership),
	}
}

// Adds a new message request to the pending list
func (req *Requests) Add(payload Request, reply *bool) error {
	if table, ok := req.Pending[payload.ID]; ok {
		combined := CombineTables(&table, &payload.Table)
		req.Pending[payload.ID] = *combined
	} else {
		req.Pending[payload.ID] = payload.Table
	}
	*reply = true
	return nil
}

// Listens to communication from neighboring nodes.
func (req *Requests) Listen(ID int, reply *Membership) error {
	if table, ok := req.Pending[ID]; ok {
		*reply = table
		delete(req.Pending, ID)
		return nil
	}
	return errors.New("Nothing to listen to")
}

func CombineTables(table1 *Membership, table2 *Membership) *Membership {
	//higher heartbeat wins-> gossip heartbeat protocol

	merged := NewMembership()

	//copy table1
	maps.Copy(merged.Members, table1.Members)

	//merge table2
	for id, node2 := range table2.Members {
		node1, exists := merged.Members[id]

		if !exists || node2.Hbcounter > node1.Hbcounter {
			merged.Members[id] = node2
		}
	}

	return merged
}
