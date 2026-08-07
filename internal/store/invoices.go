package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/janmarkuslanger/gitvoice/internal/invoice"
)

func (s *Store) invoicePath(number string) (string, error) {
	if !invoice.ValidNumber(number) {
		return "", fmt.Errorf("invalid invoice number %q", number)
	}
	return filepath.Join(s.invoicesDir, number+".json"), nil
}

// Get loads a single invoice by number.
func (s *Store) Get(number string) (invoice.Invoice, error) {
	p, err := s.invoicePath(number)
	if err != nil {
		return invoice.Invoice{}, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return invoice.Invoice{}, fmt.Errorf("%w: invoice %s", ErrNotFound, number)
	}
	if err != nil {
		return invoice.Invoice{}, fmt.Errorf("read invoice %s: %w", number, err)
	}
	var inv invoice.Invoice
	if err := json.Unmarshal(data, &inv); err != nil {
		return invoice.Invoice{}, fmt.Errorf("parse invoice %s: %w", number, err)
	}
	return inv, nil
}

// List returns all invoices, newest date first, then by number descending.
func (s *Store) List() ([]invoice.Invoice, error) {
	entries, err := os.ReadDir(s.invoicesDir)
	if err != nil {
		return nil, fmt.Errorf("read data dir: %w", err)
	}
	var invoices []invoice.Invoice
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		inv, err := s.Get(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		invoices = append(invoices, inv)
	}
	sort.Slice(invoices, func(i, j int) bool {
		if invoices[i].Date != invoices[j].Date {
			return invoices[i].Date > invoices[j].Date
		}
		return invoices[i].Number > invoices[j].Number
	})
	return invoices, nil
}

// Save validates and writes an invoice.
func (s *Store) Save(inv invoice.Invoice) error {
	if err := inv.Validate(); err != nil {
		return err
	}
	inv.Schema = invoice.CurrentSchema
	p, err := s.invoicePath(inv.Number)
	if err != nil {
		return err
	}
	if err := s.writeJSON(p, inv); err != nil {
		return fmt.Errorf("write invoice %s: %w", inv.Number, err)
	}
	return nil
}

// Delete removes an invoice. Deleting a missing invoice returns ErrNotFound.
func (s *Store) Delete(number string) error {
	p, err := s.invoicePath(number)
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: invoice %s", ErrNotFound, number)
	}
	if err != nil {
		return fmt.Errorf("delete invoice %s: %w", number, err)
	}
	return nil
}

// Exists reports whether an invoice with the given number is stored.
func (s *Store) Exists(number string) (bool, error) {
	p, err := s.invoicePath(number)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
