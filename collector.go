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
	Host string

	client  httpClient
	request *http.Request
}

func NewCollector(host string) (c Collector, err error) {
	u, err := url.Parse(host)
	if err != nil {
		return
	}

	u.Path = "/push"

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
