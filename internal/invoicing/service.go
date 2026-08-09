// Package invoicing implements the application use cases over the store:
// drafts with profile defaults, create with uniqueness checks, rename-aware
// updates. Frontends (the web UI, a future CLI) call this instead of the
// store, so the rules live in exactly one place.
package invoicing

import (
	"errors"
	"fmt"

	"github.com/janmarkuslanger/gitvoice/internal/company"
	"github.com/janmarkuslanger/gitvoice/internal/customer"
	"github.com/janmarkuslanger/gitvoice/internal/invoice"
	"github.com/janmarkuslanger/gitvoice/internal/pdf"
	"github.com/janmarkuslanger/gitvoice/internal/store"
)

// ErrNotFound mirrors store.ErrNotFound so callers only import this package.
var ErrNotFound = store.ErrNotFound

// ErrExists is returned when creating a record whose identifier is taken.
var ErrExists = errors.New("already exists")

// DefaultTaxRatePercent is the German regular VAT rate, applied to new
// drafts unless the profile enables the small-business rule.
const DefaultTaxRatePercent = 19

// Service bundles the use cases over one data directory.
type Service struct {
	store *store.Store
}

// New returns a Service over the given store.
func New(st *store.Store) *Service {
	return &Service{store: st}
}

// NewDraft returns an unsaved invoice with the tax defaults snapshotted
// from the company profile, so later profile changes never alter it.
func (s *Service) NewDraft() (invoice.Invoice, error) {
	comp, err := s.store.Company()
	if err != nil {
		return invoice.Invoice{}, err
	}
	inv := invoice.Invoice{
		Status:        invoice.StatusDraft,
		Currency:      "EUR",
		SmallBusiness: comp.SmallBusiness,
	}
	if !comp.SmallBusiness {
		inv.TaxRatePercent = DefaultTaxRatePercent
	}
	return inv, nil
}

// Invoice loads a single invoice by number.
func (s *Service) Invoice(number string) (invoice.Invoice, error) {
	return s.store.Get(number)
}

// Invoices returns all invoices, newest first.
func (s *Service) Invoices() ([]invoice.Invoice, error) {
	return s.store.List()
}

// CreateInvoice validates and saves a new invoice; the number must be free.
func (s *Service) CreateInvoice(inv invoice.Invoice) error {
	if err := inv.Validate(); err != nil {
		return err
	}
	exists, err := s.store.Exists(inv.Number)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("invoice %s: %w", inv.Number, ErrExists)
	}
	return s.store.Save(inv)
}

// UpdateInvoice replaces the invoice stored under oldNumber. The number
// doubles as the filename: a rename saves the new file and drops the old.
func (s *Service) UpdateInvoice(oldNumber string, inv invoice.Invoice) error {
	exists, err := s.store.Exists(oldNumber)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: invoice %s", ErrNotFound, oldNumber)
	}
	if err := s.store.Save(inv); err != nil {
		return err
	}
	if inv.Number != oldNumber {
		if err := s.store.Delete(oldNumber); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	return nil
}

// DeleteInvoice removes an invoice; missing numbers return ErrNotFound.
func (s *Service) DeleteInvoice(number string) error {
	return s.store.Delete(number)
}

// GeneratePDF renders the invoice to a PDF (labels in lang), files it under
// <DataDir>/pdfs/<number>.pdf, and returns the rendered bytes so the caller
// can also stream them. Missing invoices return ErrNotFound.
func (s *Service) GeneratePDF(number, lang string) ([]byte, error) {
	inv, err := s.store.Get(number)
	if err != nil {
		return nil, err
	}
	comp, err := s.store.Company()
	if err != nil {
		return nil, err
	}
	data, err := pdf.Render(inv, comp, lang)
	if err != nil {
		return nil, err
	}
	if err := s.store.SavePDF(inv.Number, data); err != nil {
		return nil, err
	}
	return data, nil
}

// Customer loads a single customer by ID.
func (s *Service) Customer(id string) (customer.Customer, error) {
	return s.store.GetCustomer(id)
}

// Customers returns all customers sorted by display name.
func (s *Service) Customers() ([]customer.Customer, error) {
	return s.store.ListCustomers()
}

// CreateCustomer validates and saves a new customer; the ID must be free.
func (s *Service) CreateCustomer(c customer.Customer) error {
	if err := c.Validate(); err != nil {
		return err
	}
	exists, err := s.store.CustomerExists(c.ID)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("customer %s: %w", c.ID, ErrExists)
	}
	return s.store.SaveCustomer(c)
}

// UpdateCustomer replaces the customer stored under oldID. The ID doubles
// as the filename: a rename saves the new file and drops the old.
func (s *Service) UpdateCustomer(oldID string, c customer.Customer) error {
	exists, err := s.store.CustomerExists(oldID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: customer %s", ErrNotFound, oldID)
	}
	if err := s.store.SaveCustomer(c); err != nil {
		return err
	}
	if c.ID != oldID {
		if err := s.store.DeleteCustomer(oldID); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	return nil
}

// DeleteCustomer removes a customer; invoices keep their data snapshot.
func (s *Service) DeleteCustomer(id string) error {
	return s.store.DeleteCustomer(id)
}

// Company loads the issuer profile, or the default one if none was saved.
func (s *Service) Company() (company.Company, error) {
	return s.store.Company()
}

// SaveCompany writes the issuer profile.
func (s *Service) SaveCompany(c company.Company) error {
	return s.store.SaveCompany(c)
}
