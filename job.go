package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"
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
	success    bool
	service    *rpc.Client
	outputChan chan loadtest.Output
	sem        *semaphore.Semaphore
	logfile    *os.File
	errfile    *os.File
}

func (j *Job) Start(outputChan chan loadtest.Output) (err error) {
	err = j.openLogFile()
	if err != nil {
		return
	}

	j.outputChan = outputChan

	if j.Users == 0 {
		j.Users = DefaultUserCount
	}
	j.sem = semaphore.New(j.Users)

	go func() {
		err := j.execute()
		if err != nil && err != io.EOF {
			return
		}
	}()

	defer func() {
		if j.process != nil {
			j.process.Kill()
			j.process.Wait()
		}
	}()

	// // Ensure process is still up
	// go func() {

	// }()

	err = backoff.Retry(j.TryConnect, backoff.NewExponentialBackOff())
	if err != nil {
		return
	}
	defer j.connection.Close()

	j.service = rpc.NewClient(j.connection)

	err = backoff.Retry(j.TryRequest, backoff.NewExponentialBackOff())
	if err != nil {
		return
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
					log.Print(err)
				}
			}()
		}
	}()

	time.Sleep(time.Duration(j.Duration) * time.Second)

	j.complete = true

	return
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
	defer func() {
		j.complete = true
	}()
	cmd := exec.Command(j.bin.Path)

	// log stderr straight out
	go func() {
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return
		}

		stderrReader := bufio.NewReader(stderr)
		for {
			if j.complete {
				return
			}

			// silently drop read errors on STDERR
			errorLine, err := stderrReader.ReadBytes('\n')
			if err == nil {
				j.logerr(errorLine)
			}
		}

	}()

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

		go j.logline(line)

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

func (j *Job) openLogFile() (err error) {
	os.MkdirAll(filepath.Join(*logDir, j.Name), os.ModePerm)
	logPath := filepath.Join(*logDir, j.Name, "out.log")
	errPath := filepath.Join(*logDir, j.Name, "err.log")

	j.logfile, err = os.OpenFile(logPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return
	}

	j.errfile, err = os.OpenFile(errPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return
	}

	return
}

func (j Job) closeLogFile() (err error) {
	return j.logfile.Close()
}

func (j Job) logerr(line []byte) {
	j.log(j.errfile, line)
}

func (j Job) logline(line []byte) {
	j.log(j.logfile, line)
}

func (j Job) log(f *os.File, line []byte) {
	fmt.Fprint(f, string(line))
}
