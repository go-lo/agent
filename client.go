package main

import (
	"log"
	"net"
	"net/rpc"
	"time"

	"github.com/jspc/loadtest"
)

type Job struct {
	Users    int    `json:"users"`
	Duration int64  `json:"duration"`
	Binary   string `json:"binary"`
}

func (j Job) Start() {
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

	time.Sleep(time.Duration(j.Duration) * time.Second)
}
