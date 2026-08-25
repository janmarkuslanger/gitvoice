// Package invoicing implements the application use cases over the store:
// drafts with profile defaults, create with uniqueness checks, rename-aware
// updates. Frontends (the web UI, the command line) call this instead of the
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
// Renaming onto a number that is already taken returns ErrExists rather than
// overwriting the invoice sitting there.
func (s *Service) UpdateInvoice(oldNumber string, inv invoice.Invoice) error {
	exists, err := s.store.Exists(oldNumber)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: invoice %s", ErrNotFound, oldNumber)
	}
	if inv.Number != oldNumber {
		taken, err := s.store.Exists(inv.Number)
		if err != nil {
			return err
		}
		if taken {
			return fmt.Errorf("invoice %s: %w", inv.Number, ErrExists)
		}
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

// PDFPath returns the file GeneratePDF writes the invoice to, so callers
// that render a PDF can report where it landed.
func (s *Service) PDFPath(number string) (string, error) {
	return s.store.PDFPath(number)
}

// Customer loads a single customer by ID.
func (s *Service) Customer(id string) (customer.Customer, error) {
	return s.store.GetCustomer(id)
}

// CustomerSnapshot returns the invoice-side copy of a customer's master
// data: the fields the invoice form prefills when a customer is selected.
// Missing customers return ErrNotFound.
func (s *Service) CustomerSnapshot(id string) (invoice.Customer, error) {
	c, err := s.store.GetCustomer(id)
	if err != nil {
		return invoice.Customer{}, err
	}
	return invoice.Customer{
		Company:   c.Company,
		FirstName: c.FirstName,
		LastName:  c.LastName,
		Address:   c.Address,
		Email:     c.Email,
		VATID:     c.VATID,
	}, nil
}

// Customers returns all customers sorted by display name.
func (s *Service) Customers() ([]customer.Customer, error) {
	return s.store.ListCustomers()
}

// maxIDAttempts bounds the walk to a free derived ID. Reaching it means a
// hundred customers share one name, which is a data problem, not a retry.
const maxIDAttempts = 100

// CreateCustomer saves a new customer under an ID derived from its name
// ("ACME GmbH" becomes "acme-gmbh", with a counter appended while that ID is
// taken) and returns the stored record, whose ID the caller needs to report
// or link to.
//
// The ID on the argument is ignored: IDs are derived here and nowhere else,
// so no caller can put two customers on the same file or invent an ID that
// does not match the name.
func (s *Service) CreateCustomer(c customer.Customer) (customer.Customer, error) {
	if c.DisplayName() == "" {
		// Reporting the empty ID on top of this would point at a field the
		// user cannot fill in anyway.
		return customer.Customer{}, customer.ErrNoName
	}
	id, err := s.freeCustomerID(customer.SlugID(c.DisplayName()))
	if err != nil {
		return customer.Customer{}, err
	}
	if id == "" {
		return customer.Customer{}, fmt.Errorf("cannot derive an id from %q: the name needs letters or digits", c.DisplayName())
	}
	c.ID = id
	if err := c.Validate(); err != nil {
		return customer.Customer{}, err
	}
	if err := s.store.SaveCustomer(c); err != nil {
		return customer.Customer{}, err
	}
	return c, nil
}

// freeCustomerID returns base, or base-2, base-3, … for the first one no
// customer occupies. An empty base stays empty: validation reports the
// missing name, which is the actual problem.
func (s *Service) freeCustomerID(base string) (string, error) {
	if base == "" {
		return "", nil
	}
	for n := 1; n <= maxIDAttempts; n++ {
		id := customer.NumberedID(base, n)
		exists, err := s.store.CustomerExists(id)
		if err != nil {
			return "", err
		}
		if !exists {
			return id, nil
		}
	}
	return "", fmt.Errorf("no free customer id derived from %q after %d attempts", base, maxIDAttempts)
}

// UpdateCustomer replaces the customer stored under id and returns the
// stored record. The ID on the argument is ignored, so an edit can never
// move a customer onto another one's file.
//
// The ID is assigned once at creation and then stays put, even when the name
// it was derived from changes: it doubles as the file name, and rewriting it
// on every name correction would move the file through the git history for
// no gain. Missing customers return ErrNotFound.
func (s *Service) UpdateCustomer(id string, c customer.Customer) (customer.Customer, error) {
	exists, err := s.store.CustomerExists(id)
	if err != nil {
		return customer.Customer{}, err
	}
	if !exists {
		return customer.Customer{}, fmt.Errorf("%w: customer %s", ErrNotFound, id)
	}
	c.ID = id
	if err := s.store.SaveCustomer(c); err != nil {
		return customer.Customer{}, err
	}
	return c, nil
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
