package main

import (
    "testing"
)

func TestPayload(t *testing.T) {
    for _, test := range []struct {
        name        string
        filename    string
        expectError bool
    }{
        {"Happy path", "testdata/config.yaml", false},
        {"Missing config", "testdata/nonsuch.yaml", true},
        {"Invalid config", "testdata/invalid.yaml", true},
    } {
        t.Run(test.name, func(t *testing.T) {
            p, err := Payload(test.filename)

            if test.expectError && err == nil {
                t.Errorf("expected error")
            }

            if !test.expectError && err != nil {
                t.Errorf("unexpected error: %+v", err)
            }

            t.Logf("Payload: %+v", p)
        })
    }
}
