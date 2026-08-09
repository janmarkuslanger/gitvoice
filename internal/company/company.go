// Package company holds the issuer profile: who the invoices come from.
// It is pure: no I/O, no HTTP, no filesystem.
package company

// DefaultSmallBusinessNote is the standard German § 19 UStG sentence,
// printed on invoices when the small-business rule applies.
const DefaultSmallBusinessNote = "Gemäß § 19 UStG wird keine Umsatzsteuer berechnet."

// Company is the invoice issuer. All fields are optional: the profile can
// be filled in incrementally and empty fields are simply not printed.
type Company struct {
	Name      string `json:"name,omitempty"`
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

// Note returns the § 19 UStG sentence to print, falling back to the default
// when the user cleared the field.
func (c Company) Note() string {
	if c.SmallBusinessNote == "" {
		return DefaultSmallBusinessNote
	}
	return c.SmallBusinessNote
}
