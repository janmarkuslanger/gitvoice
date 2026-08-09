package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janmarkuslanger/gitvoice/internal/company"
	"github.com/janmarkuslanger/gitvoice/internal/invoice"
)

func testInvoice(number, date string) invoice.Invoice {
	return invoice.Invoice{
		Number:   number,
		Date:     date,
		Status:   invoice.StatusDraft,
		Currency: "EUR",
		Customer: invoice.Customer{Company: "ACME"},
		Items:    []invoice.Item{{Description: "Work", Quantity: 2, UnitPriceCents: 5000}},
	}
}

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSaveGetRoundtrip(t *testing.T) {
	s := newStore(t)
	want := testInvoice("2026-001", "2026-08-07")
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get("2026-001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != want.Number || got.Customer.Company != want.Customer.Company ||
		got.TotalCents() != want.TotalCents() {
		t.Errorf("roundtrip mismatch: got %+v", got)
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	s := newStore(t)
	inv := testInvoice("2026-001", "2026-08-07")
	inv.Customer = invoice.Customer{}
	if err := s.Save(inv); err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if _, err := s.Get("2026-001"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("invalid invoice was persisted: %v", err)
	}
}

func TestGetRejectsUnsafeNumber(t *testing.T) {
	s := newStore(t)
	if _, err := s.Get("../../etc/passwd"); err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}

func TestGetNotFound(t *testing.T) {
	s := newStore(t)
	if _, err := s.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestListSortsNewestFirst(t *testing.T) {
	s := newStore(t)
	for _, inv := range []invoice.Invoice{
		testInvoice("2026-001", "2026-01-10"),
		testInvoice("2026-003", "2026-03-01"),
		testInvoice("2026-002", "2026-03-01"),
	} {
		if err := s.Save(inv); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	var numbers []string
	for _, inv := range got {
		numbers = append(numbers, inv.Number)
	}
	want := "2026-003,2026-002,2026-001"
	if strings.Join(numbers, ",") != want {
		t.Fatalf("order = %s, want %s", strings.Join(numbers, ","), want)
	}
}

func TestListIgnoresForeignFiles(t *testing.T) {
	s := newStore(t)
	if err := s.Save(testInvoice("2026-001", "2026-08-07")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.invoicesDir, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
}

func TestDelete(t *testing.T) {
	s := newStore(t)
	if err := s.Save(testInvoice("2026-001", "2026-08-07")); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("2026-001"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get("2026-001"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("invoice still exists after delete: %v", err)
	}
	if err := s.Delete("2026-001"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete err = %v, want ErrNotFound", err)
	}
}

func TestCompanyDefaultWhenMissing(t *testing.T) {
	s := newStore(t)
	c, err := s.Company()
	if err != nil {
		t.Fatal(err)
	}
	if c.SmallBusiness {
		t.Error("default profile should not enable small business")
	}
	if c.Note() != company.DefaultSmallBusinessNote {
		t.Errorf("Note() = %q, want default", c.Note())
	}
}

func TestCompanyRoundtrip(t *testing.T) {
	s := newStore(t)
	want := company.Company{
		Company:       "Jan Langer IT",
		TaxNumber:     "12/345/67890",
		IBAN:          "DE00123456781234567890",
		SmallBusiness: true,
	}
	if err := s.SaveCompany(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Company()
	if err != nil {
		t.Fatal(err)
	}
	if got.Company != want.Company || got.TaxNumber != want.TaxNumber ||
		got.IBAN != want.IBAN || !got.SmallBusiness {
		t.Errorf("roundtrip mismatch: got %+v", got)
	}
}

func TestExists(t *testing.T) {
	s := newStore(t)
	ok, err := s.Exists("2026-001")
	if err != nil || ok {
		t.Fatalf("Exists = %v, %v; want false, nil", ok, err)
	}
	if err := s.Save(testInvoice("2026-001", "2026-08-07")); err != nil {
		t.Fatal(err)
	}
	ok, err = s.Exists("2026-001")
	if err != nil || !ok {
		t.Fatalf("Exists = %v, %v; want true, nil", ok, err)
	}
}
