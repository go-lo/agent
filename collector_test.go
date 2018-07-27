package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/jspc/loadtest"
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
		client      httpClient
		output      loadtest.Output
		expectError bool
	}{
		{"happy path", "example.com", "test", collectorClient{status: 200}, loadtest.Output{}, false},
		{"client non 200", "example.com", "test", collectorClient{status: 500}, loadtest.Output{}, true},
		{"client error", "example.com", "test", collectorClient{err: true}, loadtest.Output{}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, _ := NewCollector(test.host, test.db)
			c.client = test.client

			err := c.Push(test.output)
			if test.expectError && err == nil {
				t.Errorf("expected error")
			}

			if !test.expectError && err != nil {
				t.Errorf("unexpected error %+v", err)
			}
		})
	}
}
