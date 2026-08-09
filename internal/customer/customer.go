// Package customer holds the customer master data. Invoices copy the
// customer's fields at creation time (snapshot), so editing or deleting a
// customer never changes already-issued invoices.
// It is pure: no I/O, no HTTP, no filesystem.
package customer

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// idPattern keeps IDs safe to use as filenames: no path separators, no
// leading dot, bounded length. (Same rule as invoice numbers.)
var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

// MaxIDLen is the longest ID idPattern accepts.
const MaxIDLen = 100

// ValidID reports whether s can be used as a customer ID.
func ValidID(s string) bool {
	return idPattern.MatchString(s)
}

// umlauts spells out the German letters that SlugID would otherwise drop, so
// "Müller Straßenbau" keeps its sounds instead of losing characters.
var umlauts = strings.NewReplacer(
	"ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss",
)

// SlugID derives an ID from a display name: lower case, German umlauts
// spelled out, and every run of other characters collapsed into a single
// '-'. "Müller & Söhne GmbH" becomes "mueller-soehne-gmbh".
//
// Characters outside a-z and 0-9 that are not German umlauts act as
// separators, so a name written entirely in another script yields "" — the
// caller then has to supply an ID.
func SlugID(name string) string {
	var b strings.Builder
	separated := false
	for _, r := range umlauts.Replace(strings.ToLower(name)) {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			separated = true
			continue
		}
		// A separator only becomes a '-' once a character follows it, so
		// the result never starts or ends with one.
		if separated && b.Len() > 0 {
			b.WriteByte('-')
		}
		separated = false
		b.WriteRune(r)
	}
	return clampID(b.String(), MaxIDLen)
}

// NumberedID returns base for n <= 1 and "base-<n>" above, shortening base
// so the result still fits MaxIDLen. It is how callers walk to a free ID
// when the derived one is taken. An empty base stays empty.
func NumberedID(base string, n int) string {
	if base == "" || n <= 1 {
		return base
	}
	suffix := "-" + strconv.Itoa(n)
	return clampID(base, MaxIDLen-len(suffix)) + suffix
}

// clampID shortens s (ASCII by construction) to at most n bytes without
// leaving a trailing '-' behind.
func clampID(s string, n int) string {
	if len(s) > n {
		s = s[:n]
	}
	return strings.TrimRight(s, "-")
}

// Customer is one entry in the master data. ID doubles as the identifier
// and the filename on disk. Either Company or LastName must be set, so both
// business and private customers work.
//
// Phone and Notes are master-data only: they are never copied onto invoices.
type Customer struct {
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

// ErrNoName reports a customer without a company and without a last name.
// There is nothing to print on an invoice then, and nothing for SlugID to
// derive an ID from, so callers deriving IDs report it on its own.
var ErrNoName = errors.New("company or last name is required")

// Validate checks invariants and returns all violations joined into one error.
func (c Customer) Validate() error {
	var errs []error
	if !ValidID(c.ID) {
		errs = append(errs, errors.New("id is required and may only contain letters, digits, '.', '_' and '-'"))
	}
	if c.Company == "" && c.LastName == "" {
		errs = append(errs, ErrNoName)
	}
	return errors.Join(errs...)
}
