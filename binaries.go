package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/satori/go.uuid"
)

// Binaries contains all of the binaries uploaded to the server
// *inside this current executaion of the server* - this is because
// that should the server go away, there's no guarantee that the
// binary will even valid/ usable/ available any more, so we don't
// even bother persisting this data
type Binaries map[string]binary

// NewBinaries will return a reference to a Binaries, ready to use
func NewBinaries() *Binaries {
	b := make(Binaries)

	return &b
}

// Add will safely update Binaries with a new binary. It will error
// if a binary with this UUID exists- we do this to avoid overwriting
// a binary in case of UUID collision
func (b *Binaries) Add(bin binary) error {
	_, ok := (*b)[bin.Name]
	if ok {
		return fmt.Errorf("Binary %s exists", bin.Name)
	}

	(*b)[bin.Name] = bin

	return nil
}

// Valid returns true if a UUID is a valid binary
func (b *Binaries) Valid(s string) bool {
	_, ok := (*b)[s]

	return ok
}

type binary struct {
	Name     string
	Path     string
	Uploaded time.Time
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
