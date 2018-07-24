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

	"github.com/abiosoft/semaphore"
	"github.com/cenkalti/backoff"
	"github.com/jspc/loadtest"
)

const (
	// RPCommand is the command to request from our RPC'd up scheduler
	RPCCommand = "Server.Run"

	// DefaultUserCount is the default number of users to run loadtests
	// to simulate when not specified/ missing
	DefaultUserCount = 25
)

type Job struct {
	Name     string `json:"name"`
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
	outputChan chan loadtest.Output
	sem        *semaphore.Semaphore
}

func (j *Job) Start(outputChan chan loadtest.Output) {
	j.outputChan = outputChan

	if j.Users == 0 {
		j.Users = DefaultUserCount
	}
	j.sem = semaphore.New(j.Users)

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

			j.sem.Acquire()
			go func() {
				defer j.sem.Release()

				err = j.TryRequest()
				if err != nil {
					panic(err)
				}
			}()
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

		// For now we unmarshal output back into a loadtest.Output
		// as a way of ensuring the content read from the binary is
		// valid to be sent to the collector endpoint. This is to ensure
		// that the collector has largely decent data to work with, and
		// that if there any errors we can get that data from an agent
		// running the test, rather than picking it out of the collector
		// logs and trying to traceback to where the data came from.
		//
		// The downside to all of this is the latency and complexity
		// of all of this unmarshalling/ marshalling back and forth.
		// It's also a bit of a false assumption- if the body of the
		// line from the scheduler is a valid json object then we're
		// still going to have a loadtest.Output- it just either wont
		// contain anything, or what it does contain will be garbage.
		o := new(loadtest.Output)

		err = json.Unmarshal(line, o)
		if err != nil {
			return
		}

		j.items++
		j.outputChan <- *o
	}
}
