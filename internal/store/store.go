// Package store persists invoices, customers, and the company profile as
// JSON files inside a directory, so the data can live in (and be diffed by)
// a git repository. Layout:
//
//	<dir>/company.json
//	<dir>/invoices/<number>.json
//	<dir>/customers/<id>.json
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotFound is returned when no record with the given identifier exists.
var ErrNotFound = errors.New("not found")

// Store reads and writes below dataDir (e.g. <repo>/data).
type Store struct {
	dataDir      string
	invoicesDir  string
	customersDir string
}

// New creates the directory layout if needed and returns a Store for it.
func New(dataDir string) (*Store, error) {
	s := &Store{
		dataDir:      dataDir,
		invoicesDir:  filepath.Join(dataDir, "invoices"),
		customersDir: filepath.Join(dataDir, "customers"),
	}
	for _, dir := range []string{s.invoicesDir, s.customersDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}
	return s, nil
}

// writeJSON writes atomically (temp file + rename) so a crash never leaves
// a half-written JSON file in the repo.
func (s *Store) writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
