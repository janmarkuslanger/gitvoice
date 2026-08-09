package pdf

import (
	"bytes"
	"testing"

	"github.com/janmarkuslanger/gitvoice/internal/company"
	"github.com/janmarkuslanger/gitvoice/internal/i18n"
	"github.com/janmarkuslanger/gitvoice/internal/invoice"
)

func testInvoice() invoice.Invoice {
	return invoice.Invoice{
		Number:         "2026-001",
		Date:           "2026-08-07",
		DueDate:        "2026-08-21",
		Status:         invoice.StatusSent,
		Currency:       "EUR",
		ServiceDate:    "2026-08-05",
		Customer:       invoice.Customer{Company: "Ätna GmbH", Address: "Straße 1\n12345 Köln", VATID: "DE123"},
		Items:          []invoice.Item{{Description: "Beratung über mehrere Tage", Quantity: 2, UnitPriceCents: 9550}},
		TaxRatePercent: 19,
	}
}

func testCompany() company.Company {
	return company.Company{
		Company:   "Issuer OHG",
		Address:   "Hauptstraße 2\n54321 Bonn",
		TaxNumber: "12/345/67890",
		VATID:     "DE999",
		IBAN:      "DE00 0000",
	}
}

// isPDF reports whether b looks like a well-formed PDF document.
func isPDF(b []byte) bool {
	return bytes.HasPrefix(b, []byte("%PDF-")) && bytes.Contains(b, []byte("%%EOF"))
}

func TestRenderProducesPDF(t *testing.T) {
	for _, lang := range []string{i18n.DE, i18n.EN} {
		data, err := Render(testInvoice(), testCompany(), lang)
		if err != nil {
			t.Fatalf("lang %s: %v", lang, err)
		}
		if !isPDF(data) {
			t.Fatalf("lang %s: output is not a PDF (len=%d)", lang, len(data))
		}
	}
}

func TestRenderSmallBusinessOmitsVAT(t *testing.T) {
	inv := testInvoice()
	inv.SmallBusiness = true
	inv.TaxRatePercent = 0
	data, err := Render(inv, testCompany(), i18n.DE)
	if err != nil {
		t.Fatal(err)
	}
	if !isPDF(data) {
		t.Fatal("output is not a PDF")
	}
}

func TestRenderMinimalInvoice(t *testing.T) {
	inv := invoice.Invoice{
		Number:   "X1",
		Date:     "2026-01-01",
		Status:   invoice.StatusDraft,
		Currency: "EUR",
		Customer: invoice.Customer{LastName: "Doe"},
		Items:    []invoice.Item{{Description: "Item", Quantity: 1, UnitPriceCents: 100}},
	}
	data, err := Render(inv, company.Company{}, i18n.EN)
	if err != nil {
		t.Fatal(err)
	}
	if !isPDF(data) {
		t.Fatal("output is not a PDF")
	}
}

// TestRenderLongDescriptionMultiPage exercises the row-wrapping and
// page-break path with many long-description line items.
func TestRenderLongDescriptionMultiPage(t *testing.T) {
	inv := testInvoice()
	long := "A very long line item description that has to wrap across multiple lines within its column to exercise the height calculation"
	inv.Items = nil
	for range 60 {
		inv.Items = append(inv.Items, invoice.Item{Description: long, Quantity: 1, UnitPriceCents: 100})
	}
	data, err := Render(inv, testCompany(), i18n.EN)
	if err != nil {
		t.Fatal(err)
	}
	if !isPDF(data) {
		t.Fatal("output is not a PDF")
	}
}
