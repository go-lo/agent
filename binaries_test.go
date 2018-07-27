package main

import (
	"testing"
)

func TestNewBinaries(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("creating new Binaries failed")
		}
	}()

	_ = NewBinaries()
}

func TestBinary_Add(t *testing.T) {
	b := NewBinaries()
	_ = b.Add(binary{Name: "example"})

	// and again
	err := b.Add(binary{Name: "example"})
	if err == nil {
		t.Errorf("expected error")
	}
}
