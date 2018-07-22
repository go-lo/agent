package main

import (
	"net/http"
)

func main() {
	jobs := make(chan Job)
	api := API{
		UploadDir: "/tmp/",
		Jobs:      jobs,
	}

	go func() {
		for j := range jobs {
			j.Start()
		}
	}()

	panic(http.ListenAndServe(":8081", api))
}
