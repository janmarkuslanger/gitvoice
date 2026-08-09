package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func customerForm() url.Values {
	return url.Values{
		"id":         {"acme"},
		"company":    {"ACME GmbH"},
		"first_name": {"Max"},
		"last_name":  {"Muster"},
		"address":    {"Musterstraße 1\n12345 Berlin"},
		"email":      {"billing@acme.example"},
		"phone":      {"+49 30 123456"},
		"vat_id":     {"DE123456789"},
		"notes":      {"prefers email"},
	}
}

func TestCustomerCrudFlow(t *testing.T) {
	srv := newTestServer(t)

	rec := postForm(t, srv, "/customers", customerForm())
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: status = %d, body: %s", rec.Code, rec.Body.String())
	}

	body := get(t, srv, "/customers").Body.String()
	for _, want := range []string{"ACME GmbH", "Max Muster", "billing@acme.example"} {
		if !strings.Contains(body, want) {
			t.Errorf("list does not contain %q", want)
		}
	}

	form := customerForm()
	form.Set("company", "ACME AG")
	if rec := postForm(t, srv, "/customers/acme", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("update: status = %d", rec.Code)
	}
	if body := get(t, srv, "/customers").Body.String(); !strings.Contains(body, "ACME AG") {
		t.Error("update not reflected in list")
	}

	if rec := postForm(t, srv, "/customers/acme/delete", url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("delete: status = %d", rec.Code)
	}
	if body := get(t, srv, "/customers").Body.String(); strings.Contains(body, "ACME") {
		t.Error("customer still listed after delete")
	}
}

func TestCustomerCreateDuplicateRejected(t *testing.T) {
	srv := newTestServer(t)
	postForm(t, srv, "/customers", customerForm())
	rec := postForm(t, srv, "/customers", customerForm())
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "already exists") {
		t.Fatalf("duplicate create: status = %d", rec.Code)
	}
}

func TestCustomerCreateInvalidShowsError(t *testing.T) {
	srv := newTestServer(t)
	form := customerForm()
	form.Set("company", "")
	form.Set("last_name", "")
	rec := postForm(t, srv, "/customers", form)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "company or last name is required") {
		t.Fatalf("status = %d, error not shown", rec.Code)
	}
}

func TestCustomerRename(t *testing.T) {
	srv := newTestServer(t)
	postForm(t, srv, "/customers", customerForm())
	form := customerForm()
	form.Set("id", "acme-ag")
	if rec := postForm(t, srv, "/customers/acme", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("rename: status = %d", rec.Code)
	}
	if rec := get(t, srv, "/customers/acme/edit"); rec.Code != http.StatusNotFound {
		t.Fatalf("old id still resolves: %d", rec.Code)
	}
	if rec := get(t, srv, "/customers/acme-ag/edit"); rec.Code != http.StatusOK {
		t.Fatalf("new id missing: %d", rec.Code)
	}
}

func TestCustomerCreateDerivesIDFromCompany(t *testing.T) {
	srv := newTestServer(t)
	form := customerForm()
	form.Set("id", "")
	form.Set("company", "Müller & Söhne GmbH")
	if rec := postForm(t, srv, "/customers", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("create: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if rec := get(t, srv, "/customers/mueller-soehne-gmbh/edit"); rec.Code != http.StatusOK {
		t.Fatalf("derived id not reachable: %d", rec.Code)
	}
	if body := get(t, srv, "/customers").Body.String(); !strings.Contains(body, "mueller-soehne-gmbh") {
		t.Error("derived id not listed")
	}
}

func TestCustomerUpdateKeepsIDWhenCleared(t *testing.T) {
	srv := newTestServer(t)
	postForm(t, srv, "/customers", customerForm())
	form := customerForm()
	form.Set("id", "")
	form.Set("company", "ACME AG")
	if rec := postForm(t, srv, "/customers/acme", form); rec.Code != http.StatusSeeOther {
		t.Fatalf("update: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := get(t, srv, "/customers/acme/edit").Body.String()
	if !strings.Contains(body, "ACME AG") {
		t.Error("edit not applied under the unchanged id")
	}
}

func TestCustomerRenameOntoTakenIDRejected(t *testing.T) {
	srv := newTestServer(t)
	postForm(t, srv, "/customers", customerForm())
	other := customerForm()
	other.Set("id", "acme-ag")
	other.Set("company", "ACME AG")
	postForm(t, srv, "/customers", other)

	form := customerForm()
	form.Set("id", "acme-ag")
	rec := postForm(t, srv, "/customers/acme", form)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "already exists") {
		t.Fatalf("rename onto a taken id: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	// Neither customer may have been lost or overwritten.
	if rec := get(t, srv, "/customers/acme/edit"); rec.Code != http.StatusOK {
		t.Errorf("renamed customer was dropped: %d", rec.Code)
	}
	if body := get(t, srv, "/customers/acme-ag/edit").Body.String(); !strings.Contains(body, "ACME AG") {
		t.Error("customer acme-ag was overwritten")
	}
}

func TestInvoiceRenameOntoTakenNumberRejected(t *testing.T) {
	srv := newTestServer(t)
	postForm(t, srv, "/invoices", createForm())
	other := createForm()
	other.Set("number", "2026-002")
	other.Set("customer_company", "Other GmbH")
	postForm(t, srv, "/invoices", other)

	form := createForm()
	form.Set("number", "2026-002")
	rec := postForm(t, srv, "/invoices/2026-001", form)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "already exists") {
		t.Fatalf("rename onto a taken number: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	if rec := get(t, srv, "/invoices/2026-001"); rec.Code != http.StatusOK {
		t.Errorf("renamed invoice was dropped: %d", rec.Code)
	}
	if body := get(t, srv, "/invoices/2026-002").Body.String(); !strings.Contains(body, "Other GmbH") {
		t.Error("invoice 2026-002 was overwritten")
	}
}

func TestInvoiceFormOffersCustomerSelect(t *testing.T) {
	srv := newTestServer(t)

	// Without customers there is no select element (the JS guard stays).
	body := get(t, srv, "/invoices/new").Body.String()
	if strings.Contains(body, `id="customer-select"`) {
		t.Error("select shown although no customers exist")
	}

	postForm(t, srv, "/customers", customerForm())
	body = get(t, srv, "/invoices/new").Body.String()
	for _, want := range []string{
		`id="customer-select"`,
		`data-company="ACME GmbH"`,
		`data-first-name="Max"`,
		`data-last-name="Muster"`,
		`data-email="billing@acme.example"`,
		`data-vat-id="DE123456789"`,
		"ACME GmbH (acme)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("invoice form does not contain %q", want)
		}
	}

	// The edit form offers the select too.
	postForm(t, srv, "/invoices", createForm())
	body = get(t, srv, "/invoices/2026-001/edit").Body.String()
	if !strings.Contains(body, `id="customer-select"`) {
		t.Error("edit form does not offer the customer select")
	}
}
