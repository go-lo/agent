package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"

	"github.com/go-lo/go-lo"
)

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

// Collector is an API client for the collector service
type Collector struct {
	Database string

	client  httpClient
	request *http.Request

	queueLen   int
	queue      []golo.Output
	queueMutex *sync.Mutex
}

// NewCollector will return a collector client, and pre-generate
// a request to be used each time we call- this should reduce the
// work needed to make each call
func NewCollector(host, db string) (c Collector, err error) {
	if host == "" {
		err = fmt.Errorf("Host cannot be empty")

		return
	}

	u, err := url.Parse(host)
	if err != nil {
		return
	}

	c.Database = db

	c.request, err = http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return
	}

	c.client = &http.Client{}

	c.queueLen = 256
	c.queue = make([]golo.Output, 0)
	c.queueMutex = new(sync.Mutex)

	return
}

// Push will take a golo.Output and send it to a collector
// to be handled there
func (c *Collector) Push(o golo.Output) (err error) {
	c.queueMutex.Lock()
	defer c.queueMutex.Unlock()

	c.queue = append(c.queue, o)
	if len(c.queue) >= c.queueLen {
		var queueBytes []byte
		queueBytes, err := json.Marshal(c.queue)
		if err != nil {
			return err
		}

		r := bytes.NewBufferString(string(queueBytes))
		c.request.Body = ioutil.NopCloser(r)

		c.request.URL.Path = fmt.Sprintf("/push/%s", c.Database)

		var resp *http.Response
		resp, err = c.client.Do(c.request)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			b, _ := ioutil.ReadAll(resp.Body)

			return fmt.Errorf("%s on %s returned %s - %s",
				c.request.Method,
				c.request.URL.String(),
				resp.Status,
				string(b),
			)
		}

		c.queue = nil
	}

	return
}
