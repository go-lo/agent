package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"
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

func TestTryConnect(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic")
		}
	}()

	j := &Job{}
	_ = j.TryConnect()
}

func TestTryRequest(t *testing.T) {
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
	td, _ := ioutil.TempDir("", "loadtest-agent-testing")

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
