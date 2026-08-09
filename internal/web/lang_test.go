package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func getWithCookie(t *testing.T, srv *Server, path, lang string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.AddCookie(&http.Cookie{Name: langCookie, Value: lang})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestLangSwitchSetsCookieAndRedirectsBack(t *testing.T) {
	srv := newTestServer(t)
	rec := get(t, srv, "/lang/de?back=/customers")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/customers" {
		t.Errorf("redirect to %q, want /customers", loc)
	}
	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == langCookie && c.Value == "de" && c.Path == "/" {
			found = true
		}
	}
	if !found {
		t.Error("lang=de cookie not set")
	}
}

func TestLangSwitchRejectsUnsupportedCode(t *testing.T) {
	srv := newTestServer(t)
	if rec := get(t, srv, "/lang/fr"); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestLangSwitchBlocksOpenRedirect(t *testing.T) {
	srv := newTestServer(t)
	for _, back := range []string{"//evil.example", "https://evil.example", ""} {
		rec := get(t, srv, "/lang/de?back="+url.QueryEscape(back))
		if loc := rec.Header().Get("Location"); loc != "/" {
			t.Errorf("back=%q redirects to %q, want /", back, loc)
		}
	}
}

func TestDefaultLanguageIsEnglish(t *testing.T) {
	srv := newTestServer(t)
	body := get(t, srv, "/").Body.String()
	if !strings.Contains(body, "<html lang=\"en\">") || !strings.Contains(body, "Invoices") {
		t.Error("default UI is not English")
	}
}

func TestCookieSwitchesUIToGerman(t *testing.T) {
	srv := newTestServer(t)
	body := getWithCookie(t, srv, "/", "de").Body.String()
	for _, want := range []string{"<html lang=\"de\">", "Rechnungen", "Kunden", "Einstellungen", "Neue Rechnung"} {
		if !strings.Contains(body, want) {
			t.Errorf("German UI missing %q", want)
		}
	}
}

func TestInvalidCookieFallsBack(t *testing.T) {
	srv := newTestServer(t)
	body := getWithCookie(t, srv, "/", "fr").Body.String()
	if !strings.Contains(body, "Invoices") {
		t.Error("invalid cookie should fall back to English")
	}
}

func TestProfileDefaultLanguage(t *testing.T) {
	srv := newTestServer(t)
	rec := postForm(t, srv, "/settings", url.Values{"language": {"de"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save settings: status = %d", rec.Code)
	}

	// No cookie: the profile default applies.
	body := get(t, srv, "/").Body.String()
	if !strings.Contains(body, "Rechnungen") {
		t.Error("profile default de not applied")
	}
	// The settings form shows the saved default as selected.
	body = get(t, srv, "/settings").Body.String()
	if !strings.Contains(body, `value="de" selected`) {
		t.Error("settings form does not preselect de")
	}
	// A cookie overrides the profile default.
	body = getWithCookie(t, srv, "/", "en").Body.String()
	if !strings.Contains(body, "Invoices") {
		t.Error("cookie en should override profile default de")
	}
}

func TestUnsupportedProfileLanguageDropped(t *testing.T) {
	srv := newTestServer(t)
	postForm(t, srv, "/settings", url.Values{"language": {"fr"}})
	body := get(t, srv, "/").Body.String()
	if !strings.Contains(body, "Invoices") {
		t.Error("unsupported profile language should fall back to English")
	}
}

func TestGermanInvoiceView(t *testing.T) {
	srv := newTestServer(t)
	form := createForm()
	form.Set("tax_rate", "19")
	if rec := postForm(t, srv, "/invoices", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("create: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := getWithCookie(t, srv, "/invoices/2026-001", "de").Body.String()
	for _, want := range []string{"Rechnung 2026-001", "Rechnung an", "Netto", "USt. 19%", "Gesamt", "Entwurf"} {
		if !strings.Contains(body, want) {
			t.Errorf("German invoice view missing %q", want)
		}
	}
}
