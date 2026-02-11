package main

import (
	"gossip/shared"
	"io"
	"net/http"
	"net/rpc"
)

func main() {
	// create a Membership list
	nodes := shared.NewMembership()
	requests := shared.NewRequests()
	leader := shared.Leader{
		NodeID: 0,
		Term:   0,
	}
	election := shared.Election{
		Results: make(map[int]int),
		Term:    0,
	}

	// register nodes with `rpc.DefaultServer`
	rpc.Register(nodes)
	rpc.Register(requests)
	rpc.Register(&leader)
	rpc.Register(&election)

	// register an HTTP handler for RPC communication
	rpc.HandleHTTP()

	// sample test endpoint
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		io.WriteString(res, "RPC SERVER LIVE!")
	})

	// listen and serve default HTTP server
	http.ListenAndServe("localhost:9005", nil)
}
