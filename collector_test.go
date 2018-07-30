package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/go-lo/go-lo"
)

type collectorClient struct {
	status int
	body   string
	err    bool
}

func (c collectorClient) Do(_ *http.Request) (r *http.Response, err error) {
	r = &http.Response{StatusCode: c.status, Body: ioutil.NopCloser(bytes.NewBufferString(c.body))}

	if c.err {
		err = fmt.Errorf("an error")
	}

	return
}

func TestNewCollector(t *testing.T) {
	for _, test := range []struct {
		name        string
		host        string
		db          string
		expectError bool
	}{
		{"happy path", "http://example.com", "test", false},
		{"empty host", "", "test", true},
		{"malformed host", "\t1x:example.com", "test", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewCollector(test.host, test.db)

			if test.expectError && err == nil {
				t.Errorf("expected error")
			}

			if !test.expectError && err != nil {
				t.Errorf("unexpected error %+v", err)
			}
		})
	}
}

func TestCollector_Push(t *testing.T) {
	for _, test := range []struct {
		name        string
		host        string
		db          string
		queueLen    int
		client      httpClient
		output      golo.Output
		expectError bool
	}{
		{"happy path", "example.com", "test", 1, collectorClient{status: 200}, golo.Output{}, false},
		{"client non 200", "example.com", "test", 1, collectorClient{status: 500}, golo.Output{}, true},
		{"client error", "example.com", "test", 1, collectorClient{err: true}, golo.Output{}, true},
		{"multiple requests queue", "example.com", "test", 10, collectorClient{}, golo.Output{}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, _ := NewCollector(test.host, test.db)
			c.client = test.client
			c.queueLen = test.queueLen

			err := c.Push(test.output)
			if test.expectError && err == nil {
				t.Errorf("expected error")
			}

			if !test.expectError && err != nil {
				t.Errorf("unexpected error %+v", err)
			}

			if test.queueLen > 1 {
				if len(c.queue) != 1 {
					t.Errorf("output did not queue")
				}
			}
		})
	}
}
