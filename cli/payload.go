package main

import (
    "flag"
    "io/ioutil"

    "github.com/go-lo/agent/agent"
    "gopkg.in/yaml.v2"
)

var (
    payloadFile = flag.String("f", "payload.yaml", "YAML file containing go-lo definition")
)

func Payload(filename string) (p agent.Payload, err error) {
    data, err := ioutil.ReadFile(filename)
    if err != nil {
        return
    }

    err = yaml.Unmarshal(data, &p)

    return
}
