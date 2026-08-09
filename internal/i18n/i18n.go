// Package i18n holds the UI translations. It is pure: no I/O, no HTTP.
package i18n

// Supported language codes. English is the fallback so installs that
// predate the language setting keep their current UI.
const (
	DE = "de"
	EN = "en"

	Fallback = EN
)

// Codes lists the supported language codes in display order.
var Codes = []string{DE, EN}

// Parse returns the supported language code matching s, or ok=false.
func Parse(s string) (string, bool) {
	for _, c := range Codes {
		if s == c {
			return c, true
		}
	}
	return "", false
}

// T returns the translation of key in lang, falling back to English and
// finally to the key itself so missing translations stay visible.
func T(lang, key string) string {
	if m, ok := dict[lang]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	if s, ok := dict[Fallback][key]; ok {
		return s
	}
	return key
}
