package main

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/jspc/loadtest"
)

const (
	// RPCommand is the command to request from our RPC'd up scheduler
	RPCCommand = "Server.Run"
)

type Job struct {
	Users    int    `json:"users"`
	Duration int64  `json:"duration"`
	Binary   string `json:"binary"`

	bin        binary
	items      int
	process    *os.Process
	connection net.Conn
	setup      bool
	complete   bool
	service    *rpc.Client
}

func (j *Job) Start() {
	go func() {
		err := j.execute()
		if err != nil && err != io.EOF {
			panic(err)
		}
	}()

	defer func() {
		if j.process != nil {
			j.process.Kill()
			j.process.Wait()
		}
	}()

	err := backoff.Retry(j.TryConnect, backoff.NewExponentialBackOff())
	if err != nil {
		panic(err)
	}
	defer j.connection.Close()

	j.service = rpc.NewClient(j.connection)

	err = backoff.Retry(j.TryRequest, backoff.NewExponentialBackOff())
	if err != nil {
		panic(err)
	}
	defer j.service.Close()

	j.setup = true

	go func() {
		for {
			if j.complete {
				return
			}

			err = j.TryRequest()
			if err != nil {
				panic(err)
			}
		}
	}()

	time.Sleep(time.Duration(j.Duration) * time.Second)

	j.complete = true
}

func (j *Job) TryConnect() (err error) {
	log.Print("try connect")
	j.connection, err = net.Dial("tcp", loadtest.RPCAddr)

	return
}

func (j *Job) TryRequest() (err error) {
	if !j.setup {
		log.Print("try request")
	}

	err = j.service.Call(RPCCommand, &loadtest.NullArg{}, &loadtest.NullArg{})

	if err != nil && !j.complete {
		return
	}

	return nil // Either err is nil, or we don't care because the command is over
}

func (j *Job) execute() (err error) {
	cmd := exec.Command(j.bin.Path)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	rd := bufio.NewReader(stdout)

	err = cmd.Start()
	if err != nil {
		return
	}

	j.process = cmd.Process

	for {
		var line []byte

		line, err = rd.ReadBytes('\n')
		if err != nil {
			return
		}

		o := new(loadtest.Output)

		err = json.Unmarshal(line, o)
		if err != nil {
			return
		}

		j.items++

		// do something here with o
	}
}
