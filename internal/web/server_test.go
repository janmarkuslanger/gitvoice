package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/janmarkuslanger/gitvoice/internal/company"
	"github.com/janmarkuslanger/gitvoice/internal/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(st)
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func createForm() url.Values {
	return url.Values{
		"number":           {"2026-001"},
		"date":             {"2026-08-07"},
		"due_date":         {"2026-08-21"},
		"status":           {"draft"},
		"currency":         {"EUR"},
		"customer_company": {"ACME GmbH"},
		"tax_rate":         {"0"},
		"item_description": {"Consulting", ""},
		"item_quantity":    {"1,5", ""},
		"item_price":       {"100,00", ""},
	}
}

func postForm(t *testing.T, srv *Server, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func get(t *testing.T, srv *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestCreateViewListFlow(t *testing.T) {
	srv := newTestServer(t)

	rec := postForm(t, srv, "/invoices", createForm())
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/invoices/2026-001" {
		t.Fatalf("create: redirect to %q", loc)
	}

	rec = get(t, srv, "/invoices/2026-001")
	if rec.Code != http.StatusOK {
		t.Fatalf("view: status = %d", rec.Code)
	}
	body := rec.Body.String()
	// 1.5 * 100.00 EUR = 150.00
	for _, want := range []string{"ACME GmbH", "Consulting", "150.00", "EUR"} {
		if !strings.Contains(body, want) {
			t.Errorf("view: body does not contain %q", want)
		}
	}

	rec = get(t, srv, "/")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "2026-001") {
		t.Fatalf("list: status = %d, missing invoice", rec.Code)
	}
}

func TestInvoiceViewShowsCustomerDetails(t *testing.T) {
	srv := newTestServer(t)
	form := createForm()
	form.Set("customer_first_name", "Max")
	form.Set("customer_last_name", "Muster")
	form.Set("customer_vat_id", "DE123456789")
	if rec := postForm(t, srv, "/invoices", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("create: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := get(t, srv, "/invoices/2026-001").Body.String()
	for _, want := range []string{"ACME GmbH", "Max Muster", "USt-IdNr.: DE123456789"} {
		if !strings.Contains(body, want) {
			t.Errorf("view does not contain %q", want)
		}
	}
}

func TestCreateInvalidRendersFormWithError(t *testing.T) {
	srv := newTestServer(t)
	form := createForm()
	form.Set("customer_company", "")
	rec := postForm(t, srv, "/invoices", form)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "company or last name is required") {
		t.Fatal("error message not shown")
	}
}

func TestCreateDuplicateRejected(t *testing.T) {
	srv := newTestServer(t)
	if rec := postForm(t, srv, "/invoices", createForm()); rec.Code != http.StatusSeeOther {
		t.Fatalf("first create failed: %d", rec.Code)
	}
	rec := postForm(t, srv, "/invoices", createForm())
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "already exists") {
		t.Fatalf("duplicate create: status = %d", rec.Code)
	}
}

func TestUpdateRenamesFile(t *testing.T) {
	srv := newTestServer(t)
	postForm(t, srv, "/invoices", createForm())

	form := createForm()
	form.Set("number", "2026-002")
	rec := postForm(t, srv, "/invoices/2026-001", form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if rec := get(t, srv, "/invoices/2026-001"); rec.Code != http.StatusNotFound {
		t.Fatalf("old number still resolves: %d", rec.Code)
	}
	if rec := get(t, srv, "/invoices/2026-002"); rec.Code != http.StatusOK {
		t.Fatalf("new number missing: %d", rec.Code)
	}
}

func TestUpdateMissingInvoice(t *testing.T) {
	srv := newTestServer(t)
	if rec := postForm(t, srv, "/invoices/nope", createForm()); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteFlow(t *testing.T) {
	srv := newTestServer(t)
	postForm(t, srv, "/invoices", createForm())

	rec := postForm(t, srv, "/invoices/2026-001/delete", url.Values{})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: status = %d", rec.Code)
	}
	if rec := get(t, srv, "/invoices/2026-001"); rec.Code != http.StatusNotFound {
		t.Fatalf("invoice still there: %d", rec.Code)
	}
	if rec := postForm(t, srv, "/invoices/2026-001/delete", url.Values{}); rec.Code != http.StatusNotFound {
		t.Fatalf("delete missing: status = %d, want 404", rec.Code)
	}
}

func TestViewNotFound(t *testing.T) {
	srv := newTestServer(t)
	if rec := get(t, srv, "/invoices/missing"); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestStaticServed(t *testing.T) {
	srv := newTestServer(t)
	rec := get(t, srv, "/static/style.css")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "invoice-sheet") {
		t.Fatalf("static css: status = %d", rec.Code)
	}
}

func TestInvoiceWithVATShowsBreakdown(t *testing.T) {
	srv := newTestServer(t)
	form := createForm()
	form.Set("tax_rate", "19")
	if rec := postForm(t, srv, "/invoices", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("create: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := get(t, srv, "/invoices/2026-001").Body.String()
	// net 150.00, VAT 19% = 28.50, gross 178.50
	for _, want := range []string{"Net", "VAT 19%", "28.50", "178.50"} {
		if !strings.Contains(body, want) {
			t.Errorf("view: body does not contain %q", want)
		}
	}
}

func TestSmallBusinessInvoiceShowsNote(t *testing.T) {
	srv := newTestServer(t)
	form := createForm()
	form.Set("small_business", "on")
	form.Set("tax_rate", "19") // must be ignored under § 19 UStG
	if rec := postForm(t, srv, "/invoices", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("create: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := get(t, srv, "/invoices/2026-001").Body.String()
	if !strings.Contains(body, "§ 19 UStG") {
		t.Error("small-business note missing on invoice")
	}
	if strings.Contains(body, "VAT 19%") {
		t.Error("VAT breakdown shown despite § 19 UStG")
	}
	if !strings.Contains(body, "150.00") {
		t.Error("gross total should equal net 150.00")
	}
}

func TestSettingsSaveAndUseAsDefaults(t *testing.T) {
	srv := newTestServer(t)

	rec := postForm(t, srv, "/settings", url.Values{
		"name":           {"Jan Langer IT"},
		"address":        {"Musterstraße 1\n12345 Berlin"},
		"tax_number":     {"12/345/67890"},
		"iban":           {"DE00123456781234567890"},
		"small_business": {"on"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save settings: status = %d", rec.Code)
	}

	body := get(t, srv, "/settings").Body.String()
	for _, want := range []string{"Jan Langer IT", "12/345/67890", "checked"} {
		if !strings.Contains(body, want) {
			t.Errorf("settings form does not contain %q", want)
		}
	}

	// New-invoice form defaults to the profile's small-business setting.
	body = get(t, srv, "/invoices/new").Body.String()
	if !strings.Contains(body, `name="small_business" checked`) {
		t.Error("new invoice does not default to small business")
	}

	// Issuer data and the § 19 note appear on the printed invoice.
	form := createForm()
	form.Set("small_business", "on")
	postForm(t, srv, "/invoices", form)
	body = get(t, srv, "/invoices/2026-001").Body.String()
	for _, want := range []string{"Jan Langer IT", "Musterstraße 1", "12/345/67890",
		"DE00123456781234567890", company.DefaultSmallBusinessNote} {
		if !strings.Contains(body, want) {
			t.Errorf("invoice view does not contain %q", want)
		}
	}
}

func TestNewFormDefaultsToVAT(t *testing.T) {
	srv := newTestServer(t)
	body := get(t, srv, "/invoices/new").Body.String()
	if !strings.Contains(body, `name="tax_rate" value="19"`) {
		t.Error("new invoice should default to 19% VAT when small business is off")
	}
}

func TestNewFormRenders(t *testing.T) {
	srv := newTestServer(t)
	rec := get(t, srv, "/invoices/new")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "New invoice") {
		t.Fatalf("new form: status = %d", rec.Code)
	}
}
