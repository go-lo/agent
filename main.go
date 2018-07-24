package main

import (
	"log"
	"net/http"

	"github.com/jspc/loadtest"
)

func main() {
	jobs := make(chan Job, 32)
	outputs := make(chan loadtest.Output)

	api := API{
		UploadDir: "/tmp/",
		Jobs:      jobs,
		Binaries:  NewBinaries(),
	}

	collector, err := NewCollector("http://localhost:8082")
	if err != nil {
		panic(err)
	}

	go func() {
		for j := range jobs {
			j.Start(outputs)

			log.Printf("ran %d times", j.items)
		}
	}()

	go func() {
		for o := range outputs {
			err := collector.Push(o)
			if err != nil {
				log.Print(err)
			}
		}
	}()

	panic(http.ListenAndServe(":8081", api))
}
