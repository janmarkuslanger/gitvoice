package invoicing

import (
	"slices"
	"testing"

	"github.com/janmarkuslanger/gitvoice/internal/company"
	"github.com/janmarkuslanger/gitvoice/internal/invoice"
)

// fullCompany is a § 14-complete issuer profile.
func fullCompany() company.Company {
	return company.Company{Name: "Jan Langer IT", Address: "Musterstr. 1", TaxNumber: "12/345/67890"}
}

// bigInvoice grosses above the § 33 small-amount limit and carries full
// recipient and service data.
func bigInvoice() invoice.Invoice {
	return invoice.Invoice{
		Number:      "2026-001",
		Date:        "2026-08-07",
		Status:      invoice.StatusDraft,
		Currency:    "EUR",
		ServiceDate: "2026-08-07",
		Customer:    invoice.Customer{Company: "ACME GmbH", Address: "Kundenweg 2"},
		Items:       []invoice.Item{{Description: "Consulting", Quantity: 1, UnitPriceCents: 100000}}, // 1000 EUR
	}
}

func has(ws []Warning, want Warning) bool {
	return slices.Contains(ws, want)
}

func TestComplianceCompleteInvoiceHasNoWarnings(t *testing.T) {
	if ws := ComplianceWarnings(bigInvoice(), fullCompany()); len(ws) != 0 {
		t.Fatalf("complete invoice produced warnings: %v", ws)
	}
}

func TestComplianceFlagsMissingIssuerAndRecipient(t *testing.T) {
	inv := bigInvoice()
	inv.Customer.Address = ""
	inv.ServiceDate = ""
	ws := ComplianceWarnings(inv, company.Company{})
	for _, want := range []Warning{WarnIssuerName, WarnIssuerAddress, WarnIssuerTaxID, WarnRecipientAddress, WarnServiceDate} {
		if !has(ws, want) {
			t.Errorf("missing warning %q in %v", want, ws)
		}
	}
}

func TestComplianceServicePeriodCountsAsTimeOfSupply(t *testing.T) {
	inv := bigInvoice()
	inv.ServiceDate = ""
	inv.ServicePeriodStart = "2026-08-01"
	inv.ServicePeriodEnd = "2026-08-31"
	if ws := ComplianceWarnings(inv, fullCompany()); has(ws, WarnServiceDate) {
		t.Errorf("service period should satisfy time-of-supply, got %v", ws)
	}
}

func TestComplianceEitherTaxNumberOrVATID(t *testing.T) {
	comp := fullCompany()
	comp.TaxNumber = ""
	comp.VATID = "DE123456789"
	if ws := ComplianceWarnings(bigInvoice(), comp); has(ws, WarnIssuerTaxID) {
		t.Errorf("VAT ID alone should satisfy the tax-id requirement, got %v", ws)
	}
}

func TestComplianceSmallAmountRelaxesExtendedSet(t *testing.T) {
	// A Kleinbetragsrechnung (<= 250 EUR gross) needs only issuer name +
	// address, not recipient address, tax id, or time of supply.
	small := bigInvoice()
	small.Items = []invoice.Item{{Description: "Coffee", Quantity: 1, UnitPriceCents: 10000}} // 100 EUR
	small.Customer.Address = ""
	small.ServiceDate = ""
	comp := company.Company{Name: "Jan Langer IT", Address: "Musterstr. 1"} // no tax number
	if ws := ComplianceWarnings(small, comp); len(ws) != 0 {
		t.Fatalf("small-amount invoice produced extended warnings: %v", ws)
	}

	// The 250 EUR boundary itself is still "small".
	small.Items = []invoice.Item{{Description: "Service", Quantity: 1, UnitPriceCents: 25000}} // exactly 250 EUR
	if ws := ComplianceWarnings(small, comp); len(ws) != 0 {
		t.Fatalf("250 EUR boundary treated as full invoice: %v", ws)
	}

	// One cent above the limit flips it to the full set.
	small.Items = []invoice.Item{{Description: "Service", Quantity: 1, UnitPriceCents: 25001}}
	if ws := ComplianceWarnings(small, comp); !has(ws, WarnRecipientAddress) {
		t.Fatalf("above-limit invoice missing recipient-address warning: %v", ws)
	}
}
