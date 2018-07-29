[![Go Report Card](https://goreportcard.com/badge/github.com/go-lo/agent)](https://goreportcard.com/report/github.com/go-lo/agent)
[![Build Status](https://travis-ci.com/go-lo/agent.svg?branch=master)](https://travis-ci.com/go-lo/agent)
[![GoDoc](https://godoc.org/github.com/go-lo/agent?status.svg)](https://godoc.org/github.com/go-lo/agent)


# Agent

Agent provides an API which exposes a series of endpoints, documented below, which allow a user to upload and queue a loadtest and schedule to loadtest endpoints.

The best way to interact with this tool is with [golo-cli](github.com/go-lo/golo-cli).

Loadtest binaries are run as self contained executables- they're expected to expose an RPC server and print results to STDOUT in a specific line format. The easiest way of doing this is to implement the loadtest binary using [github.com/go-lo/go-lo](https://github.com/go-lo/go-lo), but any binary which implements, at a minimum [server.Run()](https://godoc.org/github.com/go-lo/go-lo#Server.Run) and prints json fulfilling [Output](https://godoc.org/github.com/go-lo/go-lo#Output) can be passed to an agent to be run.

## Usage

The best way to use this application is via docker:

```bash
$ docker run -p8081:8081 -v $(pwd)/logs:/var/log/loadtest-agent goload/agent
```

This will volume mount stdout/err logs from binaries, as run, to ./logs - which can aid debugging.

This application accepts a series of command line flags:

```bash
$ docker run goload/agent --help
Usage of /agent:
  -collector string
        Collector endpoint (default "http://localhost:8082")
  -insecure
        Allow access to https endpoints with invalid certs/ certificate chains
  -logs string
        Dir to log to (default "/var/log/loadtest-agent")
```

## API endpoints

Uploading and Queueing operations are done via an HTTP API. For sample implementation, see: [github.com/go-lo/golo-client/client.go](https://github.com/go-lo/golo-cli/blob/master/client.go)

### POST `/queue`

This endpoint takes a file encoded in a multipart form- see [here](html/upload.html) for an HTML representation of this.

| Status | Message                                     | Description                                                        |
|--------|---------------------------------------------|--------------------------------------------------------------------|
| 400    | `http: no such file`                        | The form field `file` was empty                                    |
| 400    | `open /some/path no such file or directory` | Not enough permissions to write uploads                            |
| 400    | `Binary $some_uuid exists`                  | Somehow, the same UUID was generated twice. Try again, raise a bug |
| 200    | `{"binary": "some-uuid"}`                   | Success! This binary ID will be needed when queueing the job       |


### POST `/queue`

This endpoint takes a [Job](https://github.com/go-lo/agent/blob/master/job.go#L39), in json form, and starts it

| Status | Message                                | Description                                                      |
|--------|----------------------------------------|------------------------------------------------------------------|
| 400    | `invalid character '' looking for....` | The request json body was invalid                                |
| 500    | any 500                                | Could not read request body                                      |
| 400    | `$some_uuid is an invalid binary`      | UUID not a valid binary on this instance- try re-up, or open bug |
| 201    | `{"queued": true}`                     | Success!                                                         |
