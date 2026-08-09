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

// testCustomer names a customer; the service derives the id from the name,
// so callers look the record up under customer.SlugID(company).
func testCustomer(company string) customer.Customer {
	return customer.Customer{Company: company}
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
}

func TestCreateCustomerReportsUnusableName(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.CreateCustomer(customer.Customer{Company: "Ελλάδα"})
	if err == nil {
		t.Fatal("a name yielding no id was accepted")
	}
	if !strings.Contains(err.Error(), "cannot derive an id") {
		t.Errorf("err = %v, want it to name the unusable name", err)
	}
}

func TestCreateCustomerIgnoresGivenID(t *testing.T) {
	svc := newTestService(t)
	stored, err := svc.CreateCustomer(customer.Customer{ID: "hand-picked", Company: "ACME GmbH"})
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != "acme-gmbh" {
		t.Fatalf("id = %q, want the derived one", stored.ID)
	}
	if _, err := svc.Customer("hand-picked"); !errors.Is(err, ErrNotFound) {
		t.Errorf("the given id was honoured after all: %v", err)
	}
}

// The ID is assigned once and never moves, so an edit cannot rename a
// customer's file — and with that, cannot overwrite another customer.
func TestUpdateCustomerKeepsIDAndCannotOverwrite(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.CreateCustomer(testCustomer("ACME GmbH")); err != nil {
		t.Fatal(err)
	}
	victim := customer.Customer{Company: "Other GmbH", Email: "other@example.com"}
	if _, err := svc.CreateCustomer(victim); err != nil {
		t.Fatal(err)
	}

	stored, err := svc.UpdateCustomer("acme-gmbh", customer.Customer{ID: "other-gmbh", Company: "ACME AG"})
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != "acme-gmbh" {
		t.Errorf("id = %q, want it unchanged", stored.ID)
	}
	got, err := svc.Customer("acme-gmbh")
	if err != nil {
		t.Fatal(err)
	}
	if got.Company != "ACME AG" {
		t.Errorf("company = %q, want the edit applied", got.Company)
	}
	other, err := svc.Customer("other-gmbh")
	if err != nil {
		t.Fatal(err)
	}
	if other.Company != "Other GmbH" || other.Email != "other@example.com" {
		t.Errorf("customer other-gmbh was overwritten: %+v", other)
	}
}

// A renamed company keeps its ID: the ID is the file name, and rewriting it
// on every name correction would churn the git history.
func TestUpdateCustomerKeepsIDWhenNameChanges(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.CreateCustomer(testCustomer("ACME GmbH")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateCustomer("acme-gmbh", customer.Customer{Company: "Globex GmbH"}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Customer("acme-gmbh")
	if err != nil {
		t.Fatalf("customer moved away from its id: %v", err)
	}
	if got.Company != "Globex GmbH" {
		t.Errorf("company = %q, want the new name", got.Company)
	}
	if _, err := svc.Customer("globex-gmbh"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a second file appeared under the new name: %v", err)
	}
}

func TestUpdateCustomerMissing(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.UpdateCustomer("nope", testCustomer("ACME GmbH"))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestCustomerSnapshotCopiesPrintedFieldsOnly(t *testing.T) {
	svc := newTestService(t)
	c := customer.Customer{
		Company: "ACME GmbH", FirstName: "Max", LastName: "Muster",
		Address: "Musterstraße 1", Email: "billing@acme.example",
		Phone: "+49 30 123456", VATID: "DE123456789", Notes: "prefers email",
	}
	if _, err := svc.CreateCustomer(c); err != nil {
		t.Fatal(err)
	}
	got, err := svc.CustomerSnapshot("acme-gmbh")
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
