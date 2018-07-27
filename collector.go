package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/jspc/loadtest"
)

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Collector struct {
	Host     string
	Database string

	client  httpClient
	request *http.Request
}

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

	return
}

func (c Collector) Push(o loadtest.Output) (err error) {
	r := bytes.NewBufferString(o.String())
	c.request.Body = ioutil.NopCloser(r)

	c.request.URL.Path = fmt.Sprintf("/push/%s", c.Database)

	resp, err := c.client.Do(c.request)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)

		err = fmt.Errorf("%s on %s returned %s - %s",
			c.request.Method,
			c.request.URL.String(),
			resp.Status,
			string(b),
		)
	}

	return
}
