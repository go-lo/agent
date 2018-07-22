package main

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/jspc/loadtest"
)

type Job struct {
	Users    int    `json:"users"`
	Duration int64  `json:"duration"`
	Binary   string `json:"binary"`

	binaryPath string
	items      int
	process    *os.Process
	connection net.Conn
}

func (j Job) Start() {
	go func() {
		err := j.execute()
		if err != nil {
			panic(err)
		}
	}()

	connect := func() (err error) {
		j.connection, err = net.Dial("tcp", loadtest.RPCAddr)

		return
	}
	err := backoff.Retry(connect, backoff.NewExponentialBackOff())

	service := rpc.NewClient(j.connection)

	go func() {
		for {
			err = service.Call("Server.Run", &loadtest.NullArg{}, &loadtest.NullArg{})
			if err != nil {
				log.Panic(err)
			}
		}
	}()

	time.Sleep(time.Duration(j.Duration) * time.Second)

	j.process.Kill()
}

func (j *Job) execute() (err error) {
	cmd := exec.Command(j.binaryPath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	rd := bufio.NewReader(stdout)

	if err = cmd.Start(); err != nil {
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
