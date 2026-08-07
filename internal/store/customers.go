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

	"github.com/janmarkuslanger/gitvoice/internal/customer"
)

func (s *Store) customerPath(id string) (string, error) {
	if !customer.ValidID(id) {
		return "", fmt.Errorf("invalid customer id %q", id)
	}
	return filepath.Join(s.customersDir, id+".json"), nil
}

// GetCustomer loads a single customer by ID.
func (s *Store) GetCustomer(id string) (customer.Customer, error) {
	p, err := s.customerPath(id)
	if err != nil {
		return customer.Customer{}, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return customer.Customer{}, fmt.Errorf("%w: customer %s", ErrNotFound, id)
	}
	if err != nil {
		return customer.Customer{}, fmt.Errorf("read customer %s: %w", id, err)
	}
	var c customer.Customer
	if err := json.Unmarshal(data, &c); err != nil {
		return customer.Customer{}, fmt.Errorf("parse customer %s: %w", id, err)
	}
	return c, nil
}

// ListCustomers returns all customers sorted by display name, then ID.
func (s *Store) ListCustomers() ([]customer.Customer, error) {
	entries, err := os.ReadDir(s.customersDir)
	if err != nil {
		return nil, fmt.Errorf("read data dir: %w", err)
	}
	var customers []customer.Customer
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		c, err := s.GetCustomer(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		customers = append(customers, c)
	}
	sort.Slice(customers, func(i, j int) bool {
		if customers[i].DisplayName() != customers[j].DisplayName() {
			return customers[i].DisplayName() < customers[j].DisplayName()
		}
		return customers[i].ID < customers[j].ID
	})
	return customers, nil
}

// SaveCustomer validates and writes a customer.
func (s *Store) SaveCustomer(c customer.Customer) error {
	if err := c.Validate(); err != nil {
		return err
	}
	c.Schema = customer.CurrentSchema
	p, err := s.customerPath(c.ID)
	if err != nil {
		return err
	}
	if err := s.writeJSON(p, c); err != nil {
		return fmt.Errorf("write customer %s: %w", c.ID, err)
	}
	return nil
}

// DeleteCustomer removes a customer. Invoices are unaffected: they carry a
// snapshot of the customer data. Deleting a missing customer returns ErrNotFound.
func (s *Store) DeleteCustomer(id string) error {
	p, err := s.customerPath(id)
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: customer %s", ErrNotFound, id)
	}
	if err != nil {
		return fmt.Errorf("delete customer %s: %w", id, err)
	}
	return nil
}

// CustomerExists reports whether a customer with the given ID is stored.
func (s *Store) CustomerExists(id string) (bool, error) {
	p, err := s.customerPath(id)
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
