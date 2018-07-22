package main

import (
	"log"
	"net"
	"net/rpc"
	"time"

	"github.com/jspc/loadtest"
)

type Client struct {
	Users    int
	Duration time.Duration
	Binary   string
}

func (c Client) Start() {
	// copy c.Bunary to /tmp/whatever
	// make /tmp/whatever executable
	// start /tmp/whatever and slurp it's STDOUT
	// send stdout somewhere

	conn, err := net.Dial("tcp", loadtest.RPCAddr)
	if err != nil {
		panic(err)
	}

	service := rpc.NewClient(conn)

	log.Printf("%+v\n\n", service)

	go func() {
		for {
			err = service.Call("Server.Run", &loadtest.NullArg{}, &loadtest.NullArg{})
			if err != nil {
				log.Panic(err)
			}
		}
	}()

	time.Sleep(c.Duration)
}
