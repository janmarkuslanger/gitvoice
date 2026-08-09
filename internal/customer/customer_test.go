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

func TestSlugID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"company", "ACME GmbH", "acme-gmbh"},
		{"umlauts and ampersand", "Müller & Söhne GmbH", "mueller-soehne-gmbh"},
		{"sharp s", "Straßenbau Nord", "strassenbau-nord"},
		{"punctuation collapses", "ACME  GmbH & Co. KG", "acme-gmbh-co-kg"},
		{"digits kept", "3M Deutschland 2000", "3m-deutschland-2000"},
		{"leading and trailing junk", "  -- ACME --  ", "acme"},
		{"person", "Max Muster", "max-muster"},
		{"empty", "", ""},
		{"nothing usable", "—— ///", ""},
		{"other script drops", "Ελλάδα", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SlugID(tt.in)
			if got != tt.want {
				t.Fatalf("SlugID(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if got != "" && !ValidID(got) {
				t.Errorf("SlugID(%q) = %q, which is not a valid id", tt.in, got)
			}
		})
	}
}

func TestSlugIDStaysWithinMaxLen(t *testing.T) {
	got := SlugID(strings.Repeat("Muster GmbH ", 40))
	if len(got) > MaxIDLen {
		t.Fatalf("len = %d, want at most %d", len(got), MaxIDLen)
	}
	if !ValidID(got) {
		t.Errorf("%q is not a valid id", got)
	}
}

func TestNumberedID(t *testing.T) {
	if got := NumberedID("acme-gmbh", 1); got != "acme-gmbh" {
		t.Errorf("n=1 gave %q, want the base unchanged", got)
	}
	if got := NumberedID("acme-gmbh", 7); got != "acme-gmbh-7" {
		t.Errorf("n=7 gave %q, want acme-gmbh-7", got)
	}
	if got := NumberedID("", 3); got != "" {
		t.Errorf("empty base gave %q, want it to stay empty", got)
	}
	long := NumberedID(strings.Repeat("a", MaxIDLen), 12)
	if len(long) > MaxIDLen {
		t.Fatalf("len = %d, want at most %d", len(long), MaxIDLen)
	}
	if !ValidID(long) {
		t.Errorf("%q is not a valid id", long)
	}
}
