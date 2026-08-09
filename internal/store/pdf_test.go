package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSavePDFWritesFile(t *testing.T) {
	s := newStore(t)
	want := []byte("%PDF-1.3 test")
	if err := s.SavePDF("2026-001", want); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(s.pdfsDir, "2026-001.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("stored PDF = %q, want %q", got, want)
	}
}

func TestSavePDFOverwrites(t *testing.T) {
	s := newStore(t)
	if err := s.SavePDF("2026-001", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePDF("2026-001", []byte("second")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(s.pdfsDir, "2026-001.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "second" {
		t.Errorf("stored PDF = %q, want %q", got, "second")
	}
}

func TestSavePDFRejectsInvalidNumber(t *testing.T) {
	s := newStore(t)
	if err := s.SavePDF("../escape", []byte("x")); err == nil {
		t.Fatal("expected error for invalid invoice number")
	}
}
