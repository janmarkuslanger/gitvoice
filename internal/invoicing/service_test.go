package invoicing

import (
	"bytes"
	"errors"
	"testing"

	"github.com/janmarkuslanger/gitvoice/internal/company"
	"github.com/janmarkuslanger/gitvoice/internal/customer"
	"github.com/janmarkuslanger/gitvoice/internal/invoice"
	"github.com/janmarkuslanger/gitvoice/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(st)
}

func testInvoice(number string) invoice.Invoice {
	return invoice.Invoice{
		Number:   number,
		Date:     "2026-08-07",
		Status:   invoice.StatusDraft,
		Currency: "EUR",
		Customer: invoice.Customer{Company: "ACME GmbH"},
		Items:    []invoice.Item{{Description: "Consulting", Quantity: 1, UnitPriceCents: 10000}},
	}
}

func testCustomer(id string) customer.Customer {
	return customer.Customer{ID: id, Company: "ACME GmbH"}
}

func TestNewDraftDefaultsToVAT(t *testing.T) {
	svc := newTestService(t)
	inv, err := svc.NewDraft()
	if err != nil {
		t.Fatal(err)
	}
	if inv.Status != invoice.StatusDraft || inv.Currency != "EUR" {
		t.Errorf("draft = %+v, want draft status and EUR", inv)
	}
	if inv.SmallBusiness || inv.TaxRatePercent != DefaultTaxRatePercent {
		t.Errorf("draft tax: small_business=%v rate=%v, want rate %d", inv.SmallBusiness, inv.TaxRatePercent, DefaultTaxRatePercent)
	}
}

func TestNewDraftSnapshotsSmallBusinessProfile(t *testing.T) {
	svc := newTestService(t)
	if err := svc.SaveCompany(company.Company{SmallBusiness: true}); err != nil {
		t.Fatal(err)
	}
	inv, err := svc.NewDraft()
	if err != nil {
		t.Fatal(err)
	}
	if !inv.SmallBusiness || inv.TaxRatePercent != 0 {
		t.Errorf("draft tax: small_business=%v rate=%v, want small business with rate 0", inv.SmallBusiness, inv.TaxRatePercent)
	}
}

func TestCreateInvoiceRejectsDuplicate(t *testing.T) {
	svc := newTestService(t)
	if err := svc.CreateInvoice(testInvoice("2026-001")); err != nil {
		t.Fatal(err)
	}
	err := svc.CreateInvoice(testInvoice("2026-001"))
	if !errors.Is(err, ErrExists) {
		t.Fatalf("duplicate create: err = %v, want ErrExists", err)
	}
}

func TestCreateInvoiceRejectsInvalid(t *testing.T) {
	svc := newTestService(t)
	inv := testInvoice("2026-001")
	inv.Customer = invoice.Customer{}
	if err := svc.CreateInvoice(inv); err == nil {
		t.Fatal("invalid invoice was accepted")
	}
}

func TestUpdateInvoiceRenameDropsOld(t *testing.T) {
	svc := newTestService(t)
	if err := svc.CreateInvoice(testInvoice("2026-001")); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateInvoice("2026-001", testInvoice("2026-002")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Invoice("2026-001"); !errors.Is(err, ErrNotFound) {
		t.Errorf("old number still resolves: %v", err)
	}
	if _, err := svc.Invoice("2026-002"); err != nil {
		t.Errorf("new number missing: %v", err)
	}
}

func TestUpdateInvoiceMissing(t *testing.T) {
	svc := newTestService(t)
	err := svc.UpdateInvoice("nope", testInvoice("nope"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteInvoiceMissing(t *testing.T) {
	svc := newTestService(t)
	if err := svc.DeleteInvoice("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestCreateCustomerRejectsDuplicate(t *testing.T) {
	svc := newTestService(t)
	if err := svc.CreateCustomer(testCustomer("acme")); err != nil {
		t.Fatal(err)
	}
	err := svc.CreateCustomer(testCustomer("acme"))
	if !errors.Is(err, ErrExists) {
		t.Fatalf("duplicate create: err = %v, want ErrExists", err)
	}
}

func TestUpdateCustomerRenameDropsOld(t *testing.T) {
	svc := newTestService(t)
	if err := svc.CreateCustomer(testCustomer("acme")); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateCustomer("acme", testCustomer("acme-ag")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Customer("acme"); !errors.Is(err, ErrNotFound) {
		t.Errorf("old id still resolves: %v", err)
	}
	if _, err := svc.Customer("acme-ag"); err != nil {
		t.Errorf("new id missing: %v", err)
	}
}

func TestUpdateCustomerMissing(t *testing.T) {
	svc := newTestService(t)
	err := svc.UpdateCustomer("nope", testCustomer("nope"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestCustomerSnapshotCopiesPrintedFieldsOnly(t *testing.T) {
	svc := newTestService(t)
	c := customer.Customer{
		ID: "acme", Company: "ACME GmbH", FirstName: "Max", LastName: "Muster",
		Address: "Musterstraße 1", Email: "billing@acme.example",
		Phone: "+49 30 123456", VATID: "DE123456789", Notes: "prefers email",
	}
	if err := svc.CreateCustomer(c); err != nil {
		t.Fatal(err)
	}
	got, err := svc.CustomerSnapshot("acme")
	if err != nil {
		t.Fatal(err)
	}
	want := invoice.Customer{
		Company: "ACME GmbH", FirstName: "Max", LastName: "Muster",
		Address: "Musterstraße 1", Email: "billing@acme.example", VATID: "DE123456789",
	}
	if got != want {
		t.Errorf("snapshot = %+v, want %+v (phone and notes stay master data)", got, want)
	}
}

func TestCustomerSnapshotMissing(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.CustomerSnapshot("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGeneratePDFFilesAndReturnsBytes(t *testing.T) {
	svc := newTestService(t)
	if err := svc.CreateInvoice(testInvoice("2026-001")); err != nil {
		t.Fatal(err)
	}
	data, err := svc.GeneratePDF("2026-001", "de")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("returned bytes are not a PDF (len=%d)", len(data))
	}
	// The PDF must also be filed on disk.
	if _, err := svc.GeneratePDF("2026-001", "de"); err != nil {
		t.Fatalf("regenerating overwrote-existing failed: %v", err)
	}
}

func TestGeneratePDFMissingInvoice(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.GeneratePDF("nope", "en"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
