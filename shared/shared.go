package shared

import (
	"errors"
	"fmt"
	"maps"
	"math/rand"
	"sync"
)

const (
	MAX_NODES = 8
)

type Leader struct {
	NodeID int
	Term   int
}

// Node struct represents a computing node.
type Node struct {
	ID        int
	Hbcounter int
	Time      float64
	Alive     bool
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
	mu 		sync.Mutex
}

// Returns a new instance of a Membership (pointer).
func NewMembership() *Membership {
	return &Membership{
		Members: make(map[int]Node),
	}
}

// Adds a node to the membership list.
func (m *Membership) Add(payload Node, reply *Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Members[payload.ID]; ok {
		return errors.New("ID already exists")
	}
	m.Members[payload.ID] = payload
	fmt.Printf("Node %d added!\n", payload.ID)
	return nil
}

// Updates a node in the membership list.
func (m *Membership) Update(payload Node, reply *Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.Members[payload.ID]; !ok {
		return errors.New("ID does not exist")
	}
	m.Members[payload.ID] = payload
	*reply = payload

	return nil
}

func (m *Membership) GetNumNodes(id int, reply *int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	num := 0
	for _, node := range m.Members{
		if node.Alive {
			num++
		}
	}
	*reply = num
	return nil
}

// Returns a node with specific ID.
func (l *Leader) Get(payload Leader, reply *Leader) error {
	fmt.Printf("leader id: %d\ton term: %d\n", l.NodeID, l.Term)
	if l.NodeID != 0 {
		*reply = *l
		return nil
	}
	return errors.New("No Leader was found")
}

func (l *Leader) Update(newLeader *Leader, reply *bool) error {
	*l = *newLeader
	*reply = true
	fmt.Printf("New leader id: %d\ton term: %d\n", l.NodeID, l.Term)
	return nil
}

type Election struct {
	Results map[int]int
	Term    int
	mu 		sync.Mutex
}

func (e *Election) RequestVote(proposedLeader Leader, reply *int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if proposedLeader.Term < e.Term {
		return errors.New("Invalid term")
	}
	e.Results[proposedLeader.NodeID] = 1
	e.Term = proposedLeader.Term
	return nil
}

func (e *Election) SendVote(vote Leader, reply *bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if vote.Term == e.Term {
		_, ok := e.Results[vote.NodeID]
		if vote.NodeID == 0 || ok {
			e.Results[vote.NodeID] += 1
			*reply = true
		}
		return nil
	} else {
		return errors.New("Invalid term for client vote")
	}
}

func (e *Election) Get(currentLeader Leader, reply *Election) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.Term > currentLeader.Term {
		*reply = *e
	} else {
		return errors.New("No new election found")
	}
	return nil
}

func (e *Election) Clear(currTerm int, reply *bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Results = make(map[int]int)
	e.Term = currTerm + 1
	*reply = true
	return nil
}

func (e *Election) Drop(nodeID int, reply *bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.Results, nodeID)
	*reply = true
	return nil
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
	mu 		sync.Mutex
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
	req.mu.Lock()
	defer req.mu.Unlock()
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
	req.mu.Lock()
	defer req.mu.Unlock()
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
