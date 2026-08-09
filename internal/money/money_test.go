package money

import "testing"

func TestParseCents(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"12.34", 1234, false},
		{"12,34", 1234, false},
		{"1234", 123400, false},
		{"1.234,56", 123456, false},
		{"1,234.56", 123456, false},
		{" 99.9 ", 9990, false},
		{"-5", -500, false},
		{"abc", 0, true},
		{"12.34.56", 0, true},
	}
	for _, tc := range cases {
		got, err := ParseCents(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseCents(%q): expected error, got %d", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseCents(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseCents(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestFormatCents(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0.00"},
		{5, "0.05"},
		{1234, "12.34"},
		{123456, "1234.56"},
		{-1234, "-12.34"},
	}
	for _, tc := range cases {
		if got := FormatCents(tc.in); got != tc.want {
			t.Errorf("FormatCents(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseDecimal(t *testing.T) {
	cases := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{"1.5", 1.5, false},
		{"1,5", 1.5, false},
		{" 2 ", 2, false},
		{"19", 19, false},
		{"", 0, true},
		{"abc", 0, true},
	}
	for _, tc := range cases {
		got, err := ParseDecimal(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseDecimal(%q): expected error, got %v", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseDecimal(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseDecimal(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestFormatQuantity(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{1.5, "1.5"},
		{2, "2"},
		{0.25, "0.25"},
	}
	for _, tc := range cases {
		if got := FormatQuantity(tc.in); got != tc.want {
			t.Errorf("FormatQuantity(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
