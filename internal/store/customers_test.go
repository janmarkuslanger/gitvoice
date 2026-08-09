package store

import (
	"errors"
	"strings"
	"testing"

	"github.com/janmarkuslanger/gitvoice/internal/customer"
)

func TestCustomerRoundtrip(t *testing.T) {
	s := newStore(t)
	want := customer.Customer{
		ID:        "acme",
		Company:   "ACME GmbH",
		FirstName: "Max",
		LastName:  "Muster",
		Address:   "Musterstraße 1\n12345 Berlin",
		Email:     "billing@acme.example",
		Phone:     "+49 30 123456",
		VATID:     "DE123456789",
		Notes:     "prefers email",
	}
	if err := s.SaveCustomer(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetCustomer("acme")
	if err != nil {
		t.Fatal(err)
	}
	if got.Company != want.Company || got.FirstName != want.FirstName ||
		got.LastName != want.LastName || got.Address != want.Address ||
		got.Email != want.Email || got.Phone != want.Phone ||
		got.VATID != want.VATID || got.Notes != want.Notes {
		t.Errorf("roundtrip mismatch: got %+v", got)
	}
}

func TestSaveCustomerRejectsInvalid(t *testing.T) {
	s := newStore(t)
	if err := s.SaveCustomer(customer.Customer{ID: "acme"}); err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if _, err := s.GetCustomer("acme"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("invalid customer was persisted: %v", err)
	}
}

func TestGetCustomerNotFoundAndUnsafeID(t *testing.T) {
	s := newStore(t)
	if _, err := s.GetCustomer("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if _, err := s.GetCustomer("../../etc/passwd"); err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}

func TestListCustomersSortedByName(t *testing.T) {
	s := newStore(t)
	// Mix of company and private customers: sorted by display name.
	for _, c := range []customer.Customer{
		{ID: "zeta", Company: "Zeta AG"},
		{ID: "acme", Company: "ACME GmbH"},
		{ID: "beta", LastName: "Beta"},
	} {
		if err := s.SaveCustomer(c); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.ListCustomers()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, c := range got {
		ids = append(ids, c.ID)
	}
	if strings.Join(ids, ",") != "acme,beta,zeta" {
		t.Fatalf("order = %s, want acme,beta,zeta", strings.Join(ids, ","))
	}
}

func TestDeleteCustomer(t *testing.T) {
	s := newStore(t)
	if err := s.SaveCustomer(customer.Customer{ID: "acme", Company: "ACME"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCustomer("acme"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCustomer("acme"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete err = %v, want ErrNotFound", err)
	}
}

func TestCustomerExists(t *testing.T) {
	s := newStore(t)
	if ok, err := s.CustomerExists("acme"); err != nil || ok {
		t.Fatalf("Exists = %v, %v; want false, nil", ok, err)
	}
	if err := s.SaveCustomer(customer.Customer{ID: "acme", Company: "ACME"}); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.CustomerExists("acme"); err != nil || !ok {
		t.Fatalf("Exists = %v, %v; want true, nil", ok, err)
	}
}
