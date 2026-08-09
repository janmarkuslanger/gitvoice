package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/janmarkuslanger/gitvoice/internal/company"
)

func (s *Store) companyPath() string {
	return filepath.Join(s.dataDir, "company.json")
}

// Company loads the issuer profile, or the default one if none was saved yet.
func (s *Store) Company() (company.Company, error) {
	data, err := os.ReadFile(s.companyPath())
	if errors.Is(err, fs.ErrNotExist) {
		return company.Default(), nil
	}
	if err != nil {
		return company.Company{}, fmt.Errorf("read company profile: %w", err)
	}
	var c company.Company
	if err := json.Unmarshal(data, &c); err != nil {
		return company.Company{}, fmt.Errorf("parse company profile: %w", err)
	}
	return c, nil
}

// SaveCompany writes the issuer profile.
func (s *Store) SaveCompany(c company.Company) error {
	if err := s.writeJSON(s.companyPath(), c); err != nil {
		return fmt.Errorf("write company profile: %w", err)
	}
	return nil
}
