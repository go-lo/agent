package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/rpc"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/jspc/loadtest"
)

type dummyRPCClient struct {
	err bool
}

func (c dummyRPCClient) Call(_ string, _ interface{}, _ interface{}) error {
	if c.err {
		return fmt.Errorf("some error")
	}
	return nil
}

func (c dummyRPCClient) Close() error {
	return nil
}

var (
	dummyServerCalls int
)

type DummyServer struct {
	err bool
}

func (s DummyServer) Run(_ *loadtest.NullArg, _ *loadtest.NullArg) error {
	dummyServerCalls++

	if s.err && dummyServerCalls > 1 {
		return fmt.Errorf("an error")
	}
	return nil
}

var (
	td, _ = ioutil.TempDir("", "loadtest-agent-testing")
)

func TestTryConnect(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic")
		}
	}()

	j := &Job{}
	_ = j.TryConnect()
}

func TestJob_TryRequest(t *testing.T) {
	for _, test := range []struct {
		name        string
		rpcClient   rpcClient
		setup       bool
		complete    bool
		expectError bool
	}{
		{"happy path", dummyRPCClient{}, true, false, false},
		{"during setup", dummyRPCClient{}, false, false, false},
		{"error, but completed", dummyRPCClient{true}, true, true, false},
		{"error", dummyRPCClient{true}, true, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			j := Job{
				service:  test.rpcClient,
				setup:    test.setup,
				complete: test.complete,
			}

			err := j.TryRequest()
			if test.expectError && err == nil {
				t.Errorf("expected error")
			}

			if !test.expectError && err != nil {
				t.Errorf("unexpected error %+v", err)
			}
		})
	}
}

func TestOpenLogFile(t *testing.T) {
	for _, test := range []struct {
		name        string
		logDir      string
		jobName     string
		expectError bool
	}{
		{"happy path", td, "test", false},

		// This test will fail if running as root. Don't run as root.
		{"write to root", "/", "test", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			logDir = &test.logDir
			j := Job{Name: test.name}

			err := j.openLogFile()
			if test.expectError && err == nil {
				t.Errorf("expected error")
			}

			if !test.expectError && err != nil {
				t.Errorf("unexpected error %+v", err)
			}
		})
	}
}

func TestLoggingOut(t *testing.T) {
	l := "hello, world!"
	e := "goodnight, moon!"

	j := Job{
		logfile: new(bytes.Buffer),
		errfile: new(bytes.Buffer),
	}

	j.logline([]byte(l))
	j.logerr([]byte(e))

	if l != j.logfile.(*bytes.Buffer).String() {
		t.Errorf("expected %q, receved %q", l, j.logfile.(*bytes.Buffer).String())
	}

	if e != j.errfile.(*bytes.Buffer).String() {
		t.Errorf("expected %q, receved %q", e, j.errfile.(*bytes.Buffer).String())
	}
}

func TestJob_Initialise(t *testing.T) {
	users := 10

	for _, test := range []struct {
		name        string
		logDir      string
		jobName     string
		users       *int
		expectUsers int
		expectError bool
	}{
		{"happy path, specified users", td, "test", &users, 10, false},
		{"happy path unspecified users", td, "test", nil, DefaultUserCount, false},
		{"bad permissions on dir", "/", "test", &users, 10, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			logDir = &test.logDir

			j := Job{
				Name: test.name,
			}

			if test.users != nil {
				j.Users = *test.users
			}

			err := j.initialiseJob(make(chan loadtest.Output))
			if test.expectError && err == nil {
				t.Errorf("expected error")
			}

			if !test.expectError && err != nil {
				t.Errorf("unexpected error %+v", err)
			}

			if test.expectUsers != j.Users {
				t.Errorf("expected %d users, received %d", test.expectUsers, j.Users)
			}
		})
	}
}

func TestJob_InitialiseRPC(t *testing.T) {
	expoBackoff = backoff.NewExponentialBackOff()
	expoBackoff.MaxElapsedTime = time.Millisecond

	t.Run("no rpc server listening", func(t *testing.T) {
		j := Job{}

		err := j.initialiseRPC()
		if err == nil {
			t.Errorf("expected error")
		}
	})

	t.Run("with running rpc server", func(t *testing.T) {
		l, _ := net.Listen("tcp", loadtest.RPCAddr)
		defer l.Close()

		s := rpc.NewServer()
		s.Register(&DummyServer{})
		go s.Accept(l)

		RPCCommand = "DummyServer.Run"

		j := Job{}

		err := j.initialiseRPC()
		if err != nil {
			t.Errorf("unexpected error %+v", err)
		}
	})
}

func TestJob_Start(t *testing.T) {
	// This is largely a wrapper around the code we're already testing
	// and so really all we can test is that errors bubble up when/ if
	// they need to be.
	//
	// These tests use `testdata/dummy-process`, which is a dirt simple
	// binary created from `script/main.go` - it literally just prints
	// nothing to stdout. The idea is that we don't then need to either
	// hard code a binary into these tests that may not be portable, and
	// that we need a binary that's going to behave in a similar way to
	// loadtest schedules. Originally we used /bin/cat- this was largely
	// portable (sorry windows), but it kept EOF-ing on STDOUT/ STDERR which
	// ended up crashing out too early.
	//
	// Annoyingly, now though, we have to do this awful precompilation step
	// below to ensure the file exists

	err := compileDummyBinary(t)
	if err != nil {
		panic(err)
	}

	for _, test := range []struct {
		name        string
		runner      interface{}
		logDir      string
		job         Job
		doRPC       bool
		expectError bool
	}{
		{"happy path", DummyServer{}, td, Job{Name: "test", Duration: 1, bin: binary{Path: "testdata/dummy-process"}}, true, false},
		{"dodgy log dir", DummyServer{}, "/", Job{Name: "test", Duration: 1, bin: binary{Path: "testdata/dummy-process"}}, true, true},
		{"dodgy binary", DummyServer{}, td, Job{Name: "test", Duration: 5, bin: binary{Path: "/this-binary-hopefully-wont-exist"}}, true, true},
		{"dodgy rpc", DummyServer{}, td, Job{Name: "test", Duration: 1, bin: binary{Path: "testdata/dummy-process"}}, false, true},

		// Erroring requests *shouldn't* chuck an error
		{"erroring request", DummyServer{err: true}, td, Job{Name: "test", Duration: 1, bin: binary{Path: "testdata/dummy-process"}, dropRPCErrors: true}, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			expoBackoff = backoff.NewExponentialBackOff()
			expoBackoff.MaxElapsedTime = time.Millisecond

			logDir = &test.logDir

			RPCCommand = "DummyServer.Run"
			dummyServerCalls = 0
			if test.name == "dodgy rpc" {
				dummyServerCalls++
			}

			if test.doRPC {
				l, _ := net.Listen("tcp", loadtest.RPCAddr)
				defer l.Close()

				s := rpc.NewServer()
				s.Register(test.runner)
				go s.Accept(l)
			}

			err := test.job.Start(make(chan loadtest.Output))
			if test.expectError && err == nil {
				t.Errorf("expected error")
			}

			if !test.expectError && err != nil {
				t.Errorf("unexpected error %+v", err)
			}
		})
	}
}

func TestJob_Tail(t *testing.T) {
	output := `{"sequenceID":"abc123","url":"http://example.com/","method":"GET","status":200,"size":5252,"timestamp":"2018-07-28T14:19:16.343573885+01:00","duration":1000,"error":null}`

	ts, _ := time.Parse("2006-01-02T15:04:05.999999999-07:00", "2018-07-28T14:19:16.343573885+01:00")
	expect := loadtest.Output{
		SequenceID: "abc123",
		URL:        "http://example.com/",
		Method:     "GET",
		Status:     200,
		Size:       5252,
		Timestamp:  ts,
		Duration:   1000,
		Error:      nil,
	}

	for _, test := range []struct {
		name        string
		stdout      string
		stderr      string
		expect      loadtest.Output
		expectError bool
	}{
		{"valid output", output, "", expect, false},
		//		{"invalid output", "{{", "", loadtest.Output{}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			j := Job{
				logfile:    new(bytes.Buffer),
				errfile:    new(bytes.Buffer),
				stdout:     bufio.NewReader(strings.NewReader(test.stdout)),
				stderr:     bufio.NewReader(strings.NewReader(test.stderr)),
				outputChan: make(chan loadtest.Output, 1),
			}

			var output loadtest.Output
			go func() {
				output = <-j.outputChan

				j.complete = true
			}()

			err := j.tail()
			if test.expectError && err == nil {
				t.Errorf("expected error")
			}

			if !test.expectError {
				if err != nil {
					t.Errorf("unexpected error %+v", err)
				}

				if test.stdout != "" {
					if !reflect.DeepEqual(output, test.expect) {
						t.Errorf("expected %+v, received %+v", test.expect, output)
					}
				}
			}

			t.Run("stdout sanity", func(t *testing.T) {
				m := j.logfile.(*bytes.Buffer).String()
				if test.stdout != m {
					t.Errorf("expected %q, received %q", test.stdout, m)
				}
			})

			t.Run("stderr sanity", func(t *testing.T) {
				m := j.errfile.(*bytes.Buffer).String()
				if test.stderr != m {
					t.Errorf("expected %q, received %q", test.stderr, m)
				}
			})
		})
	}
}

func compileDummyBinary(t *testing.T) (err error) {
	cmd := exec.Command("go", "build", "-o", "../dummy-process")

	cmd.Dir = filepath.Join("./testdata/script")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	rd := bufio.NewReader(stdout)

	go func() {
		for {
			l := []byte{}

			l, err = rd.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					return
				}
				panic(err)
			}

			t.Log(string(l))
		}
	}()

	return cmd.Run()
}
