// Package customer holds the customer master data. Invoices copy the
// customer's fields at creation time (snapshot), so editing or deleting a
// customer never changes already-issued invoices.
// It is pure: no I/O, no HTTP, no filesystem.
package customer

import (
	"errors"
	"regexp"
	"strings"
)

// CurrentSchema is written into every persisted customer so the on-disk
// format can be migrated in later versions.
const CurrentSchema = 1

// idPattern keeps IDs safe to use as filenames: no path separators, no
// leading dot, bounded length. (Same rule as invoice numbers.)
var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

// ValidID reports whether s can be used as a customer ID.
func ValidID(s string) bool {
	return idPattern.MatchString(s)
}

// Customer is one entry in the master data. ID doubles as the identifier
// and the filename on disk. Either Company or LastName must be set, so both
// business and private customers work.
//
// Phone and Notes are master-data only: they are never copied onto invoices.
type Customer struct {
	Schema    int    `json:"schema"`
	ID        string `json:"id"`
	Company   string `json:"company,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Address   string `json:"address,omitempty"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	VATID     string `json:"vat_id,omitempty"` // customer's USt-IdNr., relevant for B2B
	Notes     string `json:"notes,omitempty"`  // internal, never printed
}

// DisplayName returns the company name, falling back to the person's name.
func (c Customer) DisplayName() string {
	if c.Company != "" {
		return c.Company
	}
	return strings.TrimSpace(c.FirstName + " " + c.LastName)
}

// PersonName returns "First Last", or "" for company-only customers.
func (c Customer) PersonName() string {
	return strings.TrimSpace(c.FirstName + " " + c.LastName)
}

// Validate checks invariants and returns all violations joined into one error.
func (c Customer) Validate() error {
	var errs []error
	if !ValidID(c.ID) {
		errs = append(errs, errors.New("id is required and may only contain letters, digits, '.', '_' and '-'"))
	}
	if c.Company == "" && c.LastName == "" {
		errs = append(errs, errors.New("company or last name is required"))
	}
	return errors.Join(errs...)
}
