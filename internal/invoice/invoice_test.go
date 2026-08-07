package invoice

import (
	"strings"
	"testing"
)

func validInvoice() Invoice {
	return Invoice{
		Number:   "2026-001",
		Date:     "2026-08-07",
		Status:   StatusDraft,
		Currency: "EUR",
		Customer: Customer{Company: "ACME GmbH"},
		Items: []Item{
			{Description: "Consulting", Quantity: 1.5, UnitPriceCents: 10000},
			{Description: "Hosting", Quantity: 1, UnitPriceCents: 999},
		},
	}
}

func TestValidateOK(t *testing.T) {
	if err := validInvoice().Validate(); err != nil {
		t.Fatalf("valid invoice rejected: %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Invoice)
		wantMsg string
	}{
		{"empty number", func(i *Invoice) { i.Number = "" }, "number"},
		{"path traversal number", func(i *Invoice) { i.Number = "../etc" }, "number"},
		{"slash in number", func(i *Invoice) { i.Number = "a/b" }, "number"},
		{"bad date", func(i *Invoice) { i.Date = "07.08.2026" }, "date"},
		{"bad due date", func(i *Invoice) { i.DueDate = "next week" }, "due date"},
		{"bad status", func(i *Invoice) { i.Status = "open" }, "status"},
		{"no currency", func(i *Invoice) { i.Currency = "" }, "currency"},
		{"no customer", func(i *Invoice) { i.Customer = Customer{FirstName: "Max"} }, "company or last name"},
		{"item without description", func(i *Invoice) { i.Items[0].Description = "" }, "item 1"},
		{"zero quantity", func(i *Invoice) { i.Items[1].Quantity = 0 }, "item 2"},
		{"negative price", func(i *Invoice) { i.Items[0].UnitPriceCents = -1 }, "item 1"},
		{"negative tax rate", func(i *Invoice) { i.TaxRatePercent = -1 }, "tax rate"},
		{"tax rate over 100", func(i *Invoice) { i.TaxRatePercent = 101 }, "tax rate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inv := validInvoice()
			tc.mutate(&inv)
			err := inv.Validate()
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error %q does not mention %q", err, tc.wantMsg)
			}
		})
	}
}

func TestCustomerNames(t *testing.T) {
	c := Customer{Company: "ACME GmbH", FirstName: "Max", LastName: "Muster"}
	if got := c.DisplayName(); got != "ACME GmbH" {
		t.Errorf("DisplayName = %q, want company", got)
	}
	if got := c.PersonName(); got != "Max Muster" {
		t.Errorf("PersonName = %q, want %q", got, "Max Muster")
	}
	c.Company = ""
	if got := c.DisplayName(); got != "Max Muster" {
		t.Errorf("DisplayName = %q, want person name", got)
	}
	if got := (Customer{LastName: "Muster"}).DisplayName(); got != "Muster" {
		t.Errorf("DisplayName = %q, want %q", got, "Muster")
	}
}

func TestTotalCents(t *testing.T) {
	inv := validInvoice()
	// 1.5 * 10000 + 1 * 999 = 15999
	if got := inv.TotalCents(); got != 15999 {
		t.Fatalf("TotalCents = %d, want 15999", got)
	}
	if got := (Invoice{}).TotalCents(); got != 0 {
		t.Fatalf("empty invoice TotalCents = %d, want 0", got)
	}
}

func TestVATMath(t *testing.T) {
	inv := validInvoice() // net 15999
	inv.TaxRatePercent = 19
	if got := inv.TaxCents(); got != 3040 { // 15999 * 0.19 = 3039.81 -> 3040
		t.Errorf("TaxCents = %d, want 3040", got)
	}
	if got := inv.TotalCents(); got != 19039 {
		t.Errorf("TotalCents = %d, want 19039", got)
	}
}

func TestSmallBusinessDisablesVAT(t *testing.T) {
	inv := validInvoice()
	inv.TaxRatePercent = 19
	inv.SmallBusiness = true
	if got := inv.TaxCents(); got != 0 {
		t.Errorf("TaxCents = %d, want 0 under § 19 UStG", got)
	}
	if got := inv.TotalCents(); got != inv.NetCents() {
		t.Errorf("TotalCents = %d, want net %d", got, inv.NetCents())
	}
}

func TestItemTotalRounds(t *testing.T) {
	// 0.333 * 100 = 33.3 -> rounds to 33
	it := Item{Quantity: 0.333, UnitPriceCents: 100}
	if got := it.TotalCents(); got != 33 {
		t.Fatalf("TotalCents = %d, want 33", got)
	}
}
