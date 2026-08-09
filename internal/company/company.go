// Package company holds the issuer profile: who the invoices come from.
// It is pure: no I/O, no HTTP, no filesystem.
package company

import "strings"

// DefaultSmallBusinessNote is the standard German § 19 UStG sentence,
// printed on invoices when the small-business rule applies.
const DefaultSmallBusinessNote = "Gemäß § 19 UStG wird keine Umsatzsteuer berechnet."

// Company is the invoice issuer. All fields are optional: the profile can
// be filled in incrementally and empty fields are simply not printed.
type Company struct {
	// Company is the business name; FirstName/LastName are the person.
	// Either Company or LastName carries the printed issuer name, so both
	// registered businesses and sole traders work.
	Company   string `json:"company,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Address   string `json:"address,omitempty"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	TaxNumber string `json:"tax_number,omitempty"` // Steuernummer
	VATID     string `json:"vat_id,omitempty"`     // USt-IdNr.
	IBAN      string `json:"iban,omitempty"`
	BIC       string `json:"bic,omitempty"`
	BankName  string `json:"bank_name,omitempty"`
	// SmallBusiness enables the German Kleinunternehmerregelung (§ 19 UStG):
	// new invoices default to no VAT and carry SmallBusinessNote.
	SmallBusiness     bool   `json:"small_business"`
	SmallBusinessNote string `json:"small_business_note,omitempty"`
	// Language is the default UI language code ("de", "en"). Empty means
	// the application fallback; validation lives in the web layer.
	Language string `json:"language,omitempty"`
}

// Default returns the profile used before the user saved one.
func Default() Company {
	return Company{SmallBusinessNote: DefaultSmallBusinessNote}
}

// DisplayName returns the company name, falling back to the person's name.
func (c Company) DisplayName() string {
	if c.Company != "" {
		return c.Company
	}
	return strings.TrimSpace(c.FirstName + " " + c.LastName)
}

// PersonName returns "First Last", or "" when no person name is set.
func (c Company) PersonName() string {
	return strings.TrimSpace(c.FirstName + " " + c.LastName)
}

// Note returns the § 19 UStG sentence to print, falling back to the default
// when the user cleared the field.
func (c Company) Note() string {
	if c.SmallBusinessNote == "" {
		return DefaultSmallBusinessNote
	}
	return c.SmallBusinessNote
}
