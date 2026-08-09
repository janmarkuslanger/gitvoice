package invoicing

import (
	"bytes"
	"errors"
	"strings"
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

func TestUpdateInvoiceRenameOntoTakenNumberKeepsBoth(t *testing.T) {
	svc := newTestService(t)
	for _, number := range []string{"2026-001", "2026-002"} {
		if err := svc.CreateInvoice(testInvoice(number)); err != nil {
			t.Fatal(err)
		}
	}
	victim := testInvoice("2026-002")
	victim.Customer = invoice.Customer{Company: "Other GmbH"}
	if err := svc.UpdateInvoice("2026-002", victim); err != nil {
		t.Fatal(err)
	}

	if err := svc.UpdateInvoice("2026-001", testInvoice("2026-002")); !errors.Is(err, ErrExists) {
		t.Fatalf("rename onto a taken number: err = %v, want ErrExists", err)
	}
	if _, err := svc.Invoice("2026-001"); err != nil {
		t.Errorf("renamed invoice was dropped: %v", err)
	}
	got, err := svc.Invoice("2026-002")
	if err != nil {
		t.Fatal(err)
	}
	if got.Customer.Company != "Other GmbH" {
		t.Errorf("invoice 2026-002 was overwritten: customer = %+v", got.Customer)
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
	if _, err := svc.CreateCustomer(testCustomer("acme")); err != nil {
		t.Fatal(err)
	}
	_, err := svc.CreateCustomer(testCustomer("acme"))
	if !errors.Is(err, ErrExists) {
		t.Fatalf("duplicate create: err = %v, want ErrExists", err)
	}
}

func TestUpdateCustomerRenameDropsOld(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.CreateCustomer(testCustomer("acme")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateCustomer("acme", testCustomer("acme-ag")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Customer("acme"); !errors.Is(err, ErrNotFound) {
		t.Errorf("old id still resolves: %v", err)
	}
	if _, err := svc.Customer("acme-ag"); err != nil {
		t.Errorf("new id missing: %v", err)
	}
}

func TestUpdateCustomerRenameOntoTakenIDKeepsBoth(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.CreateCustomer(testCustomer("acme")); err != nil {
		t.Fatal(err)
	}
	victim := customer.Customer{ID: "acme-ag", Company: "ACME AG"}
	if _, err := svc.CreateCustomer(victim); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.UpdateCustomer("acme", testCustomer("acme-ag")); !errors.Is(err, ErrExists) {
		t.Fatalf("rename onto a taken id: err = %v, want ErrExists", err)
	}
	if _, err := svc.Customer("acme"); err != nil {
		t.Errorf("renamed customer was dropped: %v", err)
	}
	got, err := svc.Customer("acme-ag")
	if err != nil {
		t.Fatal(err)
	}
	if got != victim {
		t.Errorf("customer acme-ag was overwritten: %+v, want %+v", got, victim)
	}
}

func TestUpdateCustomerKeepsIDWhenCleared(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.CreateCustomer(testCustomer("acme")); err != nil {
		t.Fatal(err)
	}
	stored, err := svc.UpdateCustomer("acme", customer.Customer{Company: "ACME AG"})
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != "acme" {
		t.Errorf("id = %q, want the current one kept", stored.ID)
	}
	got, err := svc.Customer("acme")
	if err != nil {
		t.Fatal(err)
	}
	if got.Company != "ACME AG" {
		t.Errorf("company = %q, want the edit applied", got.Company)
	}
}

func TestCreateCustomerDerivesID(t *testing.T) {
	svc := newTestService(t)
	tests := []struct {
		name string
		in   customer.Customer
		want string
	}{
		{"company", customer.Customer{Company: "Müller & Söhne GmbH"}, "mueller-soehne-gmbh"},
		{"person without company", customer.Customer{FirstName: "Max", LastName: "Muster"}, "max-muster"},
		{"company wins over person", customer.Customer{Company: "ACME GmbH", LastName: "Muster"}, "acme-gmbh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stored, err := svc.CreateCustomer(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if stored.ID != tt.want {
				t.Fatalf("id = %q, want %q", stored.ID, tt.want)
			}
			if _, err := svc.Customer(tt.want); err != nil {
				t.Errorf("customer not stored under the derived id: %v", err)
			}
		})
	}
}

func TestCreateCustomerNumbersDerivedIDOnCollision(t *testing.T) {
	svc := newTestService(t)
	for i, want := range []string{"acme-gmbh", "acme-gmbh-2", "acme-gmbh-3"} {
		stored, err := svc.CreateCustomer(customer.Customer{Company: "ACME GmbH"})
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		if stored.ID != want {
			t.Errorf("create %d: id = %q, want %q", i, stored.ID, want)
		}
	}
	// The counter must not reuse an id a manual entry already occupies.
	if _, err := svc.CreateCustomer(customer.Customer{ID: "acme-gmbh-4", Company: "ACME GmbH"}); err != nil {
		t.Fatal(err)
	}
	stored, err := svc.CreateCustomer(customer.Customer{Company: "ACME GmbH"})
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != "acme-gmbh-5" {
		t.Errorf("id = %q, want acme-gmbh-5", stored.ID)
	}
}

func TestCreateCustomerGivenIDWins(t *testing.T) {
	svc := newTestService(t)
	stored, err := svc.CreateCustomer(customer.Customer{ID: "big-client", Company: "ACME GmbH"})
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != "big-client" {
		t.Errorf("id = %q, want the given one", stored.ID)
	}
}

func TestCreateCustomerReportsUnusableName(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.CreateCustomer(customer.Customer{Company: "Ελλάδα"})
	if err == nil {
		t.Fatal("a name yielding no id was accepted")
	}
	if !strings.Contains(err.Error(), "cannot derive an id") {
		t.Errorf("err = %v, want it to ask for an explicit id", err)
	}
}

func TestUpdateCustomerMissing(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.UpdateCustomer("nope", testCustomer("nope"))
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
	if _, err := svc.CreateCustomer(c); err != nil {
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
