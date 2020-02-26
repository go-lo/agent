[![Go Report Card](https://goreportcard.com/badge/github.com/go-lo/agent)](https://goreportcard.com/report/github.com/go-lo/agent)
[![Build Status](https://travis-ci.com/go-lo/agent.svg?branch=master)](https://travis-ci.com/go-lo/agent)
[![GoDoc](https://godoc.org/github.com/go-lo/agent?status.svg)](https://godoc.org/github.com/go-lo/agent)


# Agent

The go-lo Agent exposes a set of gRPC endpoints designed to run go-lo Loadtests, as created with [go-lo/go-lo](https://github.com/go-lo/go-lo), which are packed into containers.

The agent takes a schedule which looks like:

```yaml
version: job:latest
schema:
    name: "my loadtest"
    users: 1024
    duration: 900
    container: somecontainer:latest
```

It will then enqueue this job- only one job is run at a time; multiple running jobs can affect results by creating agent network contention.

When running a job, an agent will:

1. Run the container, exposing the job's gRPC server
1. Try to connect to the job container, following an exponential backoff strategy
1. Create a pool of 1024 users, each of which will trigger a job run every second
1. After 900 seconds have elapsed, close the user pool, and kill the container
1. Wait for another job to schedule


## Interacting with the Agent

This project comes with a cli
