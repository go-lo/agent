package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/satori/go.uuid"
)

type Binaries map[string]binary

func NewBinaries() *Binaries {
	b := make(Binaries)

	return &b
}

func (b *Binaries) Add(bin binary) error {
	_, ok := (*b)[bin.Name]
	if ok {
		return fmt.Errorf("Binary %s exists", bin.Name)
	}

	(*b)[bin.Name] = bin

	return nil
}

func (c *Binaries) Valid(s string) bool {
	_, ok := (*c)[s]

	return ok
}

type binary struct {
	Name     string
	Path     string
	Uploaded *time.Time
}

func newBinary(dir string, data io.Reader) (b binary, err error) {
	err = b.Filename(dir)
	if err != nil {
		return
	}

	f, err := os.OpenFile(b.Path, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return
	}

	defer f.Close()

	io.Copy(f, data)

	return
}

func (b *binary) Filename(dir string) (err error) {
	u, err := uuid.NewV4()
	if err != nil {
		return
	}

	b.Name = u.String()
	b.Path = filepath.Join(dir, b.Name)

	return
}
