package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// API exposes two routes:
//  1. `POST /upload` which takes a schedule binary, gives it an ID, and stores it
//  2. `POST /queue` which takes a Job and puts it in a queue to run
type API struct {
	UploadDir string
	Jobs      chan Job
	Binaries  *Binaries
}

func (a API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/upload":
		a.Upload(w, r)

	case "/queue":
		a.Queue(w, r)

	default:
		http.Error(w, fmt.Sprintf("%q does not exist", r.URL.Path), http.StatusNotFound)
	}
}

// Upload takes a form (see `html/upload.html' to see what this may look like)
// and reads an updloaded file from it which it then stores.
//
// There are no size constraints on this.
//
// This binary is given a newly minted UUID and then saved as filepath.Join(a.UploadDir, uuid)
func (a *API) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, fmt.Sprintf("Method %q is not supported", r.Method), http.StatusMethodNotAllowed)

		return
	}

	r.ParseMultipartForm(32 << 20)

	file, _, err := r.FormFile("file")

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	defer file.Close()

	bin, err := newBinary(a.UploadDir, file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	err = a.Binaries.Add(bin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	fmt.Fprintf(w, `{"binary": "%s"}`, bin.Name)
}

// Queue takes a Job, in json form, with a UUID returned by a.Upload() set as `Binary`.
// From there it validates this binary is a valid within a.Binaries, and queues it
// for running
func (a API) Queue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, fmt.Sprintf("Method %q is not supported", r.Method), http.StatusMethodNotAllowed)

		return
	}

	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	j := Job{}

	err = json.Unmarshal(body, &j)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	if !a.Binaries.Valid(j.Binary) {
		http.Error(w, fmt.Sprintf("%s is an invalid binary", j.Binary), http.StatusBadRequest)

		return
	}

	j.bin = (*a.Binaries)[j.Binary]

	a.Jobs <- j

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"queued": true}`)
}
