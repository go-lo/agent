package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

var (
	emptyAPI = API{"/tmp", make(chan Job), &Binaries{"abc": binary{"abc", "/tmp/abc", time.Now()}}}

	// Note: this is fucking stupid
	permsAPI = API{"/proc", make(chan Job), &Binaries{"abc": binary{"abc", "/tmp/abc", time.Now()}}}
)

func TestServeHTTP_Upload(t *testing.T) {
	for _, test := range []struct {
		name         string
		api          API
		method       string
		field        string
		expectStatus int
	}{
		{"happy path", emptyAPI, "POST", "file", 200},
		{"incorrect method", emptyAPI, "PUT", "file", 405},
		{"incorrect form field ", emptyAPI, "POST", "filezzzz", 400},
		{"no write access to upload dir", permsAPI, "POST", "file", 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			go func() {
				for {
					<-test.api.Jobs
				}
			}()

			r, _ := os.Open("testdata/dummy-schedule")

			b := bytes.Buffer{}
			w := multipart.NewWriter(&b)

			fw, _ := w.CreateFormFile(test.field, "testdata/dummy-schedule")
			io.Copy(fw, r)
			w.Close()

			request := httptest.NewRequest(test.method, "http://example.com/upload", &b)
			request.Header.Set("Content-Type", w.FormDataContentType())

			rw := httptest.NewRecorder()
			test.api.ServeHTTP(rw, request)

			if test.expectStatus != rw.Result().StatusCode {
				t.Errorf("expected %v, received %v", test.expectStatus, rw.Result().StatusCode)
			}
		})
	}
}

func TestServeHTTP_Queue(t *testing.T) {
	for _, test := range []struct {
		name         string
		api          API
		method       string
		body         string
		expectStatus int
	}{
		{"happy path", emptyAPI, "POST", `{"binary":"abc"}`, 201},
		{"wrong method", emptyAPI, "PUT", `{"binary":"abc"}`, 405},
		{"no such binary", emptyAPI, "POST", `{"binary":"123"}`, 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			b := bytes.NewBufferString(test.body)
			r := httptest.NewRequest(test.method, "http://example.com/queue", b)

			rw := httptest.NewRecorder()
			test.api.ServeHTTP(rw, r)

			if test.expectStatus != rw.Result().StatusCode {
				t.Errorf("expected %v, received %v", test.expectStatus, rw.Result().StatusCode)
			}

		})
	}
}

func TestServeHTTP(t *testing.T) {
	for _, test := range []struct {
		name         string
		url          string
		expectStatus int
	}{
		{"404 route", "http://example.com/whatever", 404},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", test.url, nil)

			rw := httptest.NewRecorder()
			API{}.ServeHTTP(rw, r)

			if test.expectStatus != rw.Result().StatusCode {
				t.Errorf("expected %v, received %v", test.expectStatus, rw.Result().StatusCode)
			}
		})
	}
}
