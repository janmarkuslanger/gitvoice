package company

import "testing"

func TestDisplayName(t *testing.T) {
	tests := []struct {
		name string
		c    Company
		want string
	}{
		{"company wins over person", Company{Company: "ACME GmbH", FirstName: "Jan", LastName: "Langer"}, "ACME GmbH"},
		{"person when no company", Company{FirstName: "Jan", LastName: "Langer"}, "Jan Langer"},
		{"last name only", Company{LastName: "Langer"}, "Langer"},
		{"empty profile", Company{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.DisplayName(); got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPersonName(t *testing.T) {
	tests := []struct {
		name string
		c    Company
		want string
	}{
		{"first and last", Company{FirstName: "Jan", LastName: "Langer"}, "Jan Langer"},
		{"company only has no person", Company{Company: "ACME GmbH"}, ""},
		{"last name only", Company{LastName: "Langer"}, "Langer"},
		{"empty", Company{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.PersonName(); got != tt.want {
				t.Errorf("PersonName() = %q, want %q", got, tt.want)
			}
		})
	}
}
