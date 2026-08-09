// Package invoice contains the invoice domain model and validation.
// It is pure: no I/O, no HTTP, no filesystem.
package invoice

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

// Status is the lifecycle state of an invoice.
type Status string

const (
	StatusDraft    Status = "draft"
	StatusSent     Status = "sent"
	StatusPaid     Status = "paid"
	StatusCanceled Status = "canceled"
)

// Statuses lists all valid states in display order.
var Statuses = []Status{StatusDraft, StatusSent, StatusPaid, StatusCanceled}

// Customer is the recipient of an invoice: a snapshot of the customer
// master data at creation time. Either Company or LastName must be set.
type Customer struct {
	Company   string `json:"company,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Address   string `json:"address,omitempty"`
	Email     string `json:"email,omitempty"`
	VATID     string `json:"vat_id,omitempty"` // customer's USt-IdNr., relevant for B2B
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

// Item is a single line item. Amounts are stored in cents to avoid
// floating-point rounding issues in the persisted format.
type Item struct {
	Description    string  `json:"description"`
	Quantity       float64 `json:"quantity"`
	UnitPriceCents int64   `json:"unit_price_cents"`
}

// TotalCents returns the line total, rounded to the nearest cent.
func (it Item) TotalCents() int64 {
	return int64(math.Round(it.Quantity * float64(it.UnitPriceCents)))
}

// Invoice is the aggregate root. Number doubles as the identifier and
// the filename on disk.
//
// SmallBusiness and TaxRatePercent are snapshotted per invoice (defaulted
// from the company profile at creation) so that changing the profile later
// never alters already-issued invoices.
type Invoice struct {
	Number   string `json:"number"`
	Date     string `json:"date"`               // ISO 8601, e.g. 2026-08-07
	DueDate  string `json:"due_date,omitempty"` // ISO 8601
	Status   Status `json:"status"`
	Currency string `json:"currency"`
	// Time of supply (§ 14 Abs. 4 Nr. 6 UStG): either a single ServiceDate
	// or a ServicePeriodStart–ServicePeriodEnd range. All ISO 8601. Optional
	// at the model level; presence is surfaced as a non-blocking warning in
	// the invoicing layer, not enforced here.
	ServiceDate        string   `json:"service_date,omitempty"`
	ServicePeriodStart string   `json:"service_period_start,omitempty"`
	ServicePeriodEnd   string   `json:"service_period_end,omitempty"`
	Customer           Customer `json:"customer"`
	Items              []Item   `json:"items"`
	SmallBusiness      bool     `json:"small_business"`   // § 19 UStG: no VAT
	TaxRatePercent     float64  `json:"tax_rate_percent"` // ignored when SmallBusiness
	Notes              string   `json:"notes,omitempty"`
}

// NetCents returns the sum of all line items in cents.
func (inv Invoice) NetCents() int64 {
	var sum int64
	for _, it := range inv.Items {
		sum += it.TotalCents()
	}
	return sum
}

// TaxCents returns the VAT in cents; zero under the small-business rule.
func (inv Invoice) TaxCents() int64 {
	if inv.SmallBusiness {
		return 0
	}
	return int64(math.Round(float64(inv.NetCents()) * inv.TaxRatePercent / 100))
}

// TotalCents returns the gross total (net + VAT) in cents.
func (inv Invoice) TotalCents() int64 {
	return inv.NetCents() + inv.TaxCents()
}

// HasServicePeriod reports whether a full service period (start and end) is set.
func (inv Invoice) HasServicePeriod() bool {
	return inv.ServicePeriodStart != "" && inv.ServicePeriodEnd != ""
}

// numberPattern keeps numbers safe to use as filenames: no path
// separators, no leading dot, bounded length.
var numberPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

// ValidNumber reports whether s can be used as an invoice number.
func ValidNumber(s string) bool {
	return numberPattern.MatchString(s)
}

// Validate checks invariants and returns all violations joined into one error.
func (inv Invoice) Validate() error {
	var errs []error
	if !ValidNumber(inv.Number) {
		errs = append(errs, errors.New("number is required and may only contain letters, digits, '.', '_' and '-'"))
	}
	if _, err := time.Parse("2006-01-02", inv.Date); err != nil {
		errs = append(errs, errors.New("date must be a valid date (YYYY-MM-DD)"))
	}
	if inv.DueDate != "" {
		if _, err := time.Parse("2006-01-02", inv.DueDate); err != nil {
			errs = append(errs, errors.New("due date must be a valid date (YYYY-MM-DD)"))
		}
	}
	for _, d := range []struct{ label, value string }{
		{"service date", inv.ServiceDate},
		{"service period start", inv.ServicePeriodStart},
		{"service period end", inv.ServicePeriodEnd},
	} {
		if d.value != "" {
			if _, err := time.Parse("2006-01-02", d.value); err != nil {
				errs = append(errs, fmt.Errorf("%s must be a valid date (YYYY-MM-DD)", d.label))
			}
		}
	}
	if (inv.ServicePeriodStart == "") != (inv.ServicePeriodEnd == "") {
		errs = append(errs, errors.New("service period needs both a start and an end date"))
	}
	if start, err1 := time.Parse("2006-01-02", inv.ServicePeriodStart); err1 == nil {
		if end, err2 := time.Parse("2006-01-02", inv.ServicePeriodEnd); err2 == nil && end.Before(start) {
			errs = append(errs, errors.New("service period end must not be before its start"))
		}
	}
	if !validStatus(inv.Status) {
		errs = append(errs, fmt.Errorf("status %q is not valid", inv.Status))
	}
	if inv.Currency == "" {
		errs = append(errs, errors.New("currency is required"))
	}
	if inv.Customer.Company == "" && inv.Customer.LastName == "" {
		errs = append(errs, errors.New("customer company or last name is required"))
	}
	if inv.TaxRatePercent < 0 || inv.TaxRatePercent > 100 {
		errs = append(errs, errors.New("tax rate must be between 0 and 100"))
	}
	for i, it := range inv.Items {
		if it.Description == "" {
			errs = append(errs, fmt.Errorf("item %d: description is required", i+1))
		}
		if it.Quantity <= 0 {
			errs = append(errs, fmt.Errorf("item %d: quantity must be greater than zero", i+1))
		}
		if it.UnitPriceCents < 0 {
			errs = append(errs, fmt.Errorf("item %d: unit price must not be negative", i+1))
		}
	}
	return errors.Join(errs...)
}

func validStatus(s Status) bool {
	for _, v := range Statuses {
		if s == v {
			return true
		}
	}
	return false
}
