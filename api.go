package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

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
	}

	err = a.Binaries.Add(bin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	fmt.Fprintf(w, `{"binary": "%s"}`, bin.Name)
}

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
