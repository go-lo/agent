package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/satori/go.uuid"
)

type Binaries []string

func (b Binaries) Valid(n string) bool {
	for _, i := range b {
		if i == n {
			return true
		}
	}

	return false
}

type API struct {
	UploadDir string
	Jobs      chan Job
	Binaries  Binaries
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

	fn, fp, err := a.Filename()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	f, err := os.OpenFile(fp, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	defer f.Close()

	io.Copy(f, file)

	a.Binaries = append(a.Binaries, fn)

	fmt.Fprintf(w, `{"binary": "%s"}`, fn)
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

	a.Jobs <- j

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"queued": true}`)
}

func (a API) Filename() (n, f string, err error) {
	u, err := uuid.NewV4()
	if err != nil {
		return
	}

	n = u.String()
	f = a.Filepath(n)

	return
}

func (a API) Filepath(f string) string {
	return filepath.Join(a.UploadDir, f)
}
