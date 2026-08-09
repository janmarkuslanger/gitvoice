package i18n

import "testing"

// Every language must define exactly the same key set, so no page can
// render half-translated.
func TestDictionariesHaveSameKeys(t *testing.T) {
	for _, code := range Codes {
		if _, ok := dict[code]; !ok {
			t.Fatalf("no dictionary for supported language %q", code)
		}
	}
	base := dict[Fallback]
	for code, m := range dict {
		if code == Fallback {
			continue
		}
		for key := range base {
			if _, ok := m[key]; !ok {
				t.Errorf("%s: missing key %q", code, key)
			}
		}
		for key := range m {
			if _, ok := base[key]; !ok {
				t.Errorf("%s: extra key %q not in fallback %s", code, key, Fallback)
			}
		}
	}
}

func TestT(t *testing.T) {
	if got := T(DE, "nav.customers"); got != "Kunden" {
		t.Errorf("de nav.customers = %q", got)
	}
	if got := T(EN, "nav.customers"); got != "Customers" {
		t.Errorf("en nav.customers = %q", got)
	}
	// Unknown language falls back to English.
	if got := T("fr", "nav.customers"); got != "Customers" {
		t.Errorf("fr fallback = %q", got)
	}
	// Unknown key stays visible as-is.
	if got := T(EN, "no.such.key"); got != "no.such.key" {
		t.Errorf("missing key = %q", got)
	}
}

func TestParse(t *testing.T) {
	for _, code := range Codes {
		if got, ok := Parse(code); !ok || got != code {
			t.Errorf("Parse(%q) = %q, %v", code, got, ok)
		}
	}
	for _, bad := range []string{"", "fr", "DE", "en-US"} {
		if _, ok := Parse(bad); ok {
			t.Errorf("Parse(%q) should fail", bad)
		}
	}
}
