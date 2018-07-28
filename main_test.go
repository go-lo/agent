package main

import (
	"testing"
	"time"

	"github.com/cenkalti/backoff"
)

func TestJobHandler(t *testing.T) {
	err := compileDummyBinary(t)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		err := recover()
		if err != nil {
			t.Errorf("unexpected error %+v", err)
		}
	}()

	expoBackoff = backoff.NewExponentialBackOff()
	expoBackoff.MaxElapsedTime = time.Millisecond

	cooldownSeconds = 1
	silent = true

	l := "/tmp"
	logDir = &l

	c, _ := NewCollector("example.com", "")
	c.client = collectorClient{}

	j := make(chan Job)

	go func() {
		j <- Job{bin: binary{Path: "testdata/dummy-process"}, Duration: 1, Users: 1}
		close(j)
	}()

	JobHandler(c, j)
}

func TestSetup(t *testing.T) {
	a, c, err := Setup(true, "http://localhost/", "/tmp")
	if err != nil {
		t.Errorf("unexpected error %+v", err)
	}

	if a.UploadDir != "/tmp" {
		t.Error("a.UpoadDir is mutated")
	}

	if c.request.URL.String() != "http://localhost/" {
		t.Errorf("c.Host is mutated: %q", c.request.URL.String())
	}

	if c.Database != DeadLetterDatabase {
		t.Errorf("c.Database has been initialised with the wrong database. Should be %q, is %q", DeadLetterDatabase, c.Database)
	}
}
