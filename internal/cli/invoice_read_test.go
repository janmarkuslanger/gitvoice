package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janmarkuslanger/gitvoice/internal/company"
)

// invoiceJSON mirrors the documented -json keys, so a rename in the output
// contract fails here.
type invoiceJSON struct {
	Number   string `json:"number"`
	Status   string `json:"status"`
	Currency string `json:"currency"`
	Net      int64  `json:"net_cents"`
	Tax      int64  `json:"tax_cents"`
	Total    int64  `json:"total_cents"`
	Warnings []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"warnings"`
}

// summary is the comparable part of invoiceJSON: identity, amounts, and the
// values derived from them.
type summary struct {
	Number, Status, Currency string
	Net, Tax, Total          int64
}

func (j invoiceJSON) summary() summary {
	return summary{j.Number, j.Status, j.Currency, j.Net, j.Tax, j.Total}
}

func TestInvoiceList(t *testing.T) {
	r, _, out, errOut := newRunner(t)
	if code := runOn(r, "invoice", "list"); code != 0 {
		t.Fatalf("empty list: exit = %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "no invoices yet") {
		t.Errorf("stdout = %q, want a note that there is nothing yet", out.String())
	}

	for _, args := range [][]string{
		{"-number", "2026-001", "-date", "2026-08-01", "-customer-company", "ACME GmbH", "-item", "Consulting;1;100.00"},
		{"-number", "2026-002", "-date", "2026-08-09", "-customer-last-name", "Muster", "-status", "paid", "-item", "Design;2;50.00"},
	} {
		if code := runOn(r, append([]string{"invoice", "add"}, args...)...); code != 0 {
			t.Fatalf("add: exit = %d, stderr: %s", code, errOut.String())
		}
	}
	out.Reset()
	if code := runOn(r, "invoice", "list"); code != 0 {
		t.Fatalf("list: exit = %d, stderr: %s", code, errOut.String())
	}

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want a header and two invoices:\n%s", len(lines), out.String())
	}
	for _, want := range []string{"NUMBER", "DATE", "STATUS", "RECIPIENT", "TOTAL"} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("header %q missing column %q", lines[0], want)
		}
	}
	// List returns the newest invoice first.
	for _, want := range []string{"2026-002", "2026-08-09", "paid", "Muster", "119.00 EUR"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("row %q does not contain %q", lines[1], want)
		}
	}
	if !strings.Contains(lines[2], "2026-001") || !strings.Contains(lines[2], "ACME GmbH") {
		t.Errorf("row %q does not describe the older invoice", lines[2])
	}
}

func TestInvoiceListJSON(t *testing.T) {
	r, _, out, errOut := newRunner(t)
	if code := runOn(r, "invoice", "list", "-json"); code != 0 {
		t.Fatalf("empty list: exit = %d, stderr: %s", code, errOut.String())
	}
	if got := strings.TrimSpace(out.String()); got != "[]" {
		t.Errorf("empty list = %q, want an empty JSON array", got)
	}

	out.Reset()
	if code := runOn(r, "invoice", "add", "-number", "2026-001",
		"-customer-company", "ACME GmbH", "-item", "Consulting;3;120.00"); code != 0 {
		t.Fatalf("add: exit = %d, stderr: %s", code, errOut.String())
	}
	out.Reset()
	if code := runOn(r, "invoice", "list", "-json"); code != 0 {
		t.Fatalf("list: exit = %d, stderr: %s", code, errOut.String())
	}

	var got []invoiceJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if len(got) != 1 {
		t.Fatalf("got %d invoices, want 1: %s", len(got), out.String())
	}
	want := summary{"2026-001", "draft", "EUR", 36000, 6840, 42840}
	if got[0].summary() != want {
		t.Errorf("invoice = %+v, want %+v", got[0].summary(), want)
	}
}

func TestInvoiceShow(t *testing.T) {
	r, _, out, errOut := newRunner(t)
	if code := runOn(r, "invoice", "add",
		"-number", "2026-001", "-date", "2026-08-01", "-due-date", "2026-08-15",
		"-customer-company", "ACME GmbH", "-customer-last-name", "Muster",
		`-customer-address`, `Musterstraße 1\n12345 Berlin`,
		"-service-period-start", "2026-07-01", "-service-period-end", "2026-07-31",
		"-notes", `Thanks!\nPay on time.`,
		"-item", "Consulting;3;120.00", "-item", "Travel;1;49,50"); code != 0 {
		t.Fatalf("add: exit = %d, stderr: %s", code, errOut.String())
	}
	out.Reset()
	if code := runOn(r, "invoice", "show", "2026-001"); code != 0 {
		t.Fatalf("show: exit = %d, stderr: %s", code, errOut.String())
	}
	for _, want := range []string{
		"2026-001", "2026-08-01", "2026-08-15", "2026-07-01 – 2026-07-31", "draft",
		"ACME GmbH", "Muster", "Musterstraße 1", "12345 Berlin",
		"Consulting", "3 x 120.00", "360.00", "Travel", "49.50",
		"409.50 EUR", "VAT 19%", "77.81", "487.31 EUR",
		"Thanks! / Pay on time.",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("show output does not mention %q:\n%s", want, out.String())
		}
	}
	// Above the § 33 UStDV small-amount limit and with an empty profile, the
	// completeness check has something to say.
	if !strings.Contains(out.String(), "Missing mandatory details") {
		t.Errorf("show output does not report the missing details:\n%s", out.String())
	}
}

func TestInvoiceShowSmallBusinessAndJSON(t *testing.T) {
	r, svc, out, errOut := newRunner(t)
	if err := svc.SaveCompany(company.Company{
		Company: "Studio Muster", Address: "Hauptstraße 2\n10115 Berlin", TaxNumber: "12/345/67890",
	}); err != nil {
		t.Fatal(err)
	}
	if code := runOn(r, "invoice", "add", "-number", "2026-007", "-small-business",
		"-customer-company", "ACME GmbH", "-customer-address", "Musterstraße 1",
		"-service-date", "2026-08-01", "-item", "Consulting;1;100.00"); code != 0 {
		t.Fatalf("add: exit = %d, stderr: %s", code, errOut.String())
	}
	out.Reset()
	if code := runOn(r, "invoice", "show", "2026-007"); code != 0 {
		t.Fatalf("show: exit = %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "§ 19 UStG") {
		t.Errorf("show output does not mark the invoice as VAT-free:\n%s", out.String())
	}
	if strings.Contains(out.String(), "Missing mandatory details") {
		t.Errorf("complete invoice still reports missing details:\n%s", out.String())
	}

	out.Reset()
	if code := runOn(r, "invoice", "show", "-json", "2026-007"); code != 0 {
		t.Fatalf("show -json: exit = %d, stderr: %s", code, errOut.String())
	}
	var got invoiceJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	want := summary{"2026-007", "draft", "EUR", 10000, 0, 10000}
	if got.summary() != want {
		t.Errorf("invoice = %+v, want %+v", got.summary(), want)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("warnings = %+v, want none for a complete invoice", got.Warnings)
	}
}

func TestInvoiceShowReportsWarningsAsJSON(t *testing.T) {
	r, _, out, errOut := newRunner(t)
	if code := runOn(r, "invoice", "add", "-number", "2026-008",
		"-customer-company", "ACME GmbH", "-item", "Consulting;1;500.00"); code != 0 {
		t.Fatalf("add: exit = %d, stderr: %s", code, errOut.String())
	}
	out.Reset()
	if code := runOn(r, "invoice", "show", "-json", "2026-008"); code != 0 {
		t.Fatalf("show -json: exit = %d, stderr: %s", code, errOut.String())
	}
	var got invoiceJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	codes := map[string]string{}
	for _, w := range got.Warnings {
		codes[w.Code] = w.Message
	}
	for _, want := range []string{
		"compliance.issuer_name", "compliance.issuer_address",
		"compliance.issuer_tax_id", "compliance.recipient_address", "compliance.service_date",
	} {
		msg, ok := codes[want]
		if !ok {
			t.Errorf("warnings %+v do not include %q", got.Warnings, want)
			continue
		}
		if msg == "" || msg == want {
			t.Errorf("warning %q has no readable message (%q)", want, msg)
		}
	}
}

func TestInvoiceAddWarnsAboutMissingDetails(t *testing.T) {
	r, _, out, errOut := newRunner(t)
	if code := runOn(r, "invoice", "add", "-number", "2026-009",
		"-customer-company", "ACME GmbH", "-item", "Consulting;1;500.00"); code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "invoice 2026-009 created") {
		t.Errorf("stdout = %q, want the invoice to be written anyway", out.String())
	}
	for _, want := range []string{"warning", "Missing mandatory details", "Recipient address", "Time of supply"} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("stderr = %q, want it to mention %q", errOut.String(), want)
		}
	}
}

func TestInvoiceAddStaysQuietWhenComplete(t *testing.T) {
	r, svc, _, errOut := newRunner(t)
	if err := svc.SaveCompany(company.Company{
		Company: "Studio Muster", Address: "Hauptstraße 2", VATID: "DE123456789",
	}); err != nil {
		t.Fatal(err)
	}
	if code := runOn(r, "invoice", "add", "-number", "2026-010",
		"-customer-company", "ACME GmbH", "-customer-address", "Musterstraße 1",
		"-service-date", "2026-08-01", "-item", "Consulting;1;500.00"); code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errOut.String())
	}
	if errOut.String() != "" {
		t.Errorf("stderr = %q, want no warnings for a complete invoice", errOut.String())
	}
}

func TestInvoicePDF(t *testing.T) {
	dir := t.TempDir()
	r, _, out, errOut := newRunnerIn(t, dir)
	if code := runOn(r, "invoice", "add", "-number", "2026-011",
		"-customer-company", "ACME GmbH", "-item", "Consulting;1;100.00"); code != 0 {
		t.Fatalf("add: exit = %d, stderr: %s", code, errOut.String())
	}
	out.Reset()
	if code := runOn(r, "invoice", "pdf", "2026-011"); code != 0 {
		t.Fatalf("pdf: exit = %d, stderr: %s", code, errOut.String())
	}
	path := filepath.Join(dir, "pdfs", "2026-011.pdf")
	if !strings.Contains(out.String(), path) {
		t.Errorf("stdout = %q, want the path %q it wrote", out.String(), path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("PDF not written: %v", err)
	}
	if len(data) == 0 || string(data[:4]) != "%PDF" {
		t.Errorf("file is not a PDF (%d bytes)", len(data))
	}

	// Rendering again overwrites, so the command is safe to repeat.
	out.Reset()
	if code := runOn(r, "invoice", "pdf", "-lang", "de", "2026-011"); code != 0 {
		t.Fatalf("pdf -lang de: exit = %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "2026-011") {
		t.Errorf("stdout = %q, want the invoice number", out.String())
	}
}

func TestInvoicePDFErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"unknown invoice", []string{"pdf", "ghost"}, "not found"},
		{"unknown language", []string{"pdf", "-lang", "fr", "2026-001"}, `unknown language "fr"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := run(t, append([]string{"invoice"}, tt.args...)...)
			if res.code != 1 {
				t.Fatalf("exit = %d, want 1", res.code)
			}
			if !strings.Contains(res.err, tt.want) {
				t.Errorf("stderr = %q, want it to mention %q", res.err, tt.want)
			}
		})
	}
}

func TestInvoiceShowUnknownNumber(t *testing.T) {
	res := run(t, "invoice", "show", "ghost")
	if res.code != 1 {
		t.Fatalf("exit = %d, want 1", res.code)
	}
	if !strings.Contains(res.err, "not found") {
		t.Errorf("stderr = %q, want a not-found message", res.err)
	}
}

func TestCustomerListJSON(t *testing.T) {
	r, _, out, errOut := newRunner(t)
	if code := runOn(r, "customer", "list", "-json"); code != 0 {
		t.Fatalf("empty list: exit = %d, stderr: %s", code, errOut.String())
	}
	if got := strings.TrimSpace(out.String()); got != "[]" {
		t.Errorf("empty list = %q, want an empty JSON array", got)
	}
	out.Reset()
	if code := runOn(r, "customer", "add", "-company", "ACME GmbH", "-email", "billing@acme.example"); code != 0 {
		t.Fatalf("add: exit = %d, stderr: %s", code, errOut.String())
	}
	out.Reset()
	if code := runOn(r, "customer", "list", "-json"); code != 0 {
		t.Fatalf("list: exit = %d, stderr: %s", code, errOut.String())
	}
	var got []struct {
		ID      string `json:"id"`
		Company string `json:"company"`
		Email   string `json:"email"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if len(got) != 1 || got[0].ID != "acme-gmbh" || got[0].Company != "ACME GmbH" || got[0].Email != "billing@acme.example" {
		t.Errorf("customers = %+v, want the stored record with its id", got)
	}
}
