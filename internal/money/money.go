// Package money parses and formats monetary amounts (stored as cents) and
// decimal user input. It is pure: no I/O, no HTTP.
package money

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseCents converts user input like "12.34", "12,34" or "1.234,56" into
// cents. It accepts both '.' and ',' as decimal separator; when both occur,
// the last one wins and the other is treated as a thousands separator.
func ParseCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	dot, comma := strings.LastIndex(s, "."), strings.LastIndex(s, ",")
	switch {
	case dot >= 0 && comma >= 0:
		if comma > dot {
			s = strings.ReplaceAll(s, ".", "")
			s = strings.Replace(s, ",", ".", 1)
		} else {
			s = strings.ReplaceAll(s, ",", "")
		}
	case comma >= 0:
		s = strings.Replace(s, ",", ".", 1)
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q", s)
	}
	return int64(math.Round(f * 100)), nil
}

// FormatCents renders cents as a plain decimal, e.g. 123450 -> "1234.50".
func FormatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

// ParseDecimal parses a plain decimal (quantity, tax rate) accepting ','
// as decimal separator, e.g. "1,5" -> 1.5.
func ParseDecimal(s string) (float64, error) {
	return strconv.ParseFloat(strings.Replace(strings.TrimSpace(s), ",", ".", 1), 64)
}

// FormatQuantity renders a quantity without trailing zeros, e.g. 1.5 -> "1.5", 2 -> "2".
func FormatQuantity(q float64) string {
	return strconv.FormatFloat(q, 'f', -1, 64)
}
