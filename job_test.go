package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/rpc"
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

type DummyServer struct{}

func (s DummyServer) Run(_ *loadtest.NullArg, _ *loadtest.NullArg) error {
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
