package main

import (
	"net/http"
)

func main() {
	jobs := make(chan Job, 1024)
	api := API{
		UploadDir: "/tmp/",
		Jobs:      jobs,
		Binaries:  NewBinaries(),
	}

	go func() {
		for j := range jobs {
			j.Start()
		}
	}()

	panic(http.ListenAndServe(":8081", api))
}
