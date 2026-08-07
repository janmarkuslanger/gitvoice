package customer

import (
	"strings"
	"testing"
)

func TestValidateOK(t *testing.T) {
	cases := []Customer{
		{ID: "acme", Company: "ACME GmbH"},
		{ID: "muster", LastName: "Muster"},
		{ID: "both", Company: "ACME GmbH", FirstName: "Max", LastName: "Muster"},
	}
	for _, c := range cases {
		if err := c.Validate(); err != nil {
			t.Errorf("valid customer %q rejected: %v", c.ID, err)
		}
	}
}

func TestValidateErrors(t *testing.T) {
	cases := []struct {
		name    string
		c       Customer
		wantMsg string
	}{
		{"empty id", Customer{Company: "ACME"}, "id"},
		{"path traversal id", Customer{ID: "../etc", Company: "ACME"}, "id"},
		{"neither company nor last name", Customer{ID: "acme", FirstName: "Max"}, "company or last name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.Validate()
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error %q does not mention %q", err, tc.wantMsg)
			}
		})
	}
}

func TestDisplayName(t *testing.T) {
	c := Customer{Company: "ACME GmbH", FirstName: "Max", LastName: "Muster"}
	if got := c.DisplayName(); got != "ACME GmbH" {
		t.Errorf("DisplayName = %q, want company", got)
	}
	c.Company = ""
	if got := c.DisplayName(); got != "Max Muster" {
		t.Errorf("DisplayName = %q, want person name", got)
	}
	if got := c.PersonName(); got != "Max Muster" {
		t.Errorf("PersonName = %q, want %q", got, "Max Muster")
	}
}
