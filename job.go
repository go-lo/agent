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
	"syscall"
	"time"

	"github.com/abiosoft/semaphore"
	"github.com/cenkalti/backoff"
	"github.com/go-lo/go-lo"
)

const (
	// DefaultUserCount is the default number of users to run loadtests
	// to simulate when not specified/ missing
	DefaultUserCount = 25
)

var (
	// RPCCommand is the command to request from our RPC'd up scheduler
	RPCCommand = "Server.Run"

	expoBackoff = backoff.NewExponentialBackOff()
)

type rpcClient interface {
	Call(string, interface{}, interface{}) error
	Close() error
}

// Job contains both the input data from a call to the queue endpoint
// on the API (So: name, binary, etc.) and metadata for the running of
// this job, including process information, service clients, stdout/err
// and flags to determine whether the job is complete
type Job struct {
	Name     string `json:"name"`
	Users    int    `json:"users"`
	Duration int64  `json:"duration"`
	Binary   string `json:"binary"`

	bin           binary
	items         int
	process       *os.Process
	connection    net.Conn
	setup         bool
	complete      bool
	success       bool
	dropRPCErrors bool
	service       rpcClient
	outputChan    chan golo.Output
	sem           *semaphore.Semaphore
	stdout        *bufio.Reader
	stderr        *bufio.Reader
	logfile       io.Writer
	errfile       io.Writer
}

// Start will, given a valid output chan hooked up to a collector client,
// start a loadtest schedule binary, slurp it's stdout/err and then stop it
// once j.Duration seconds pass
func (j *Job) Start(outputChan chan golo.Output) (err error) {
	err = j.initialiseJob(outputChan)
	if err != nil {
		return
	}

	err = j.execute()
	if err != nil {
		return
	}

	go func() {
		err := j.tail()
		if err != nil && err != io.EOF {
			log.Print(err)
		}
	}()

	defer func() {
		if j.process != nil {
			j.process.Kill()

			state, _ := j.process.Wait()
			if !state.Success() {
				status := state.Sys().(syscall.WaitStatus)

				log.Printf("%s exited with status %d, stop signal %s",
					j.Binary,
					status.ExitStatus(),
					status.Signal(),
				)
			}
		}
	}()

	err = j.initialiseRPC()
	if err != nil {
		return
	}

	defer j.connection.Close()
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
				if err != nil && !j.dropRPCErrors {
					log.Print(err)
				}
			}()
		}
	}()

	time.Sleep(time.Duration(j.Duration) * time.Second)
	j.complete = true

	return
}

func (j *Job) initialiseJob(outputChan chan golo.Output) (err error) {
	err = j.openLogFile()
	if err != nil {
		return
	}

	j.outputChan = outputChan

	if j.Users == 0 {
		j.Users = DefaultUserCount
	}
	j.sem = semaphore.New(j.Users)

	return
}

func (j *Job) initialiseRPC() (err error) {
	err = backoff.Retry(j.TryConnect, expoBackoff)
	if err != nil {
		return
	}
	expoBackoff.Reset()

	j.service = rpc.NewClient(j.connection)

	err = backoff.Retry(j.TryRequest, expoBackoff)
	expoBackoff.Reset()

	return
}

// TryConnect tests whether the loadtest schedule binary is
// up and listening on the correct address
func (j *Job) TryConnect() (err error) {
	log.Print("try connect")
	j.connection, err = net.Dial("tcp", golo.RPCAddr)

	return
}

// TryRequest will run an RPC call to the golo RPC server
// listening on https://godoc.org/github.com/go-lo/go-lo#Server.Run
// it's used to test whether a schedule is ready to be run (via
// exponential backoff) and then, once ready, to actuall perform
// tests
func (j *Job) TryRequest() (err error) {
	if !j.setup {
		log.Print("try request")
	}

	err = j.service.Call(RPCCommand, &golo.NullArg{}, &golo.NullArg{})

	if err != nil && !j.complete {
		return
	}

	return nil // Either err is nil, or we don't care because the command is over
}

func (j *Job) execute() (err error) {
	cmd := exec.Command(j.bin.Path)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return
	}

	j.stderr = bufio.NewReader(stderr)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	j.stdout = bufio.NewReader(stdout)

	err = cmd.Start()
	if err != nil {
		return
	}

	j.process = cmd.Process

	return
}

func (j *Job) tail() (err error) {
	defer func() {
		j.complete = true
	}()

	// log stderr straight out
	go func() {
		for {
			if j.stderr == nil {
				continue
			}

			if j.complete {
				return
			}

			scanner := bufio.NewScanner(j.stderr)
			for scanner.Scan() {
				j.logerr(scanner.Bytes())
			}

			if err := scanner.Err(); err != nil {
				continue
			}
		}
	}()

	for {
		if j.complete {
			return
		}

		var line []byte

		scanner := bufio.NewScanner(j.stdout)
		for scanner.Scan() {
			line = scanner.Bytes()
			go j.logline(line)

			// For now we unmarshal output back into a golo.Output
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
			// still going to have a golo.Output- it just either wont
			// contain anything, or what it does contain will be garbage.
			o := new(golo.Output)

			err = json.Unmarshal(line, o)
			if err != nil {
				log.Print(string(line))
				log.Print(err)

				return
			}

			j.items++
			j.outputChan <- *o
		}

		if err := scanner.Err(); err != nil {
			continue
		}

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

func (j Job) logerr(line []byte) {
	j.log(j.errfile, line)
}

func (j Job) logline(line []byte) {
	j.log(j.logfile, line)
}

func (j Job) log(f io.Writer, line []byte) {
	fmt.Fprintf(f, "%s\n", string(line))
}
