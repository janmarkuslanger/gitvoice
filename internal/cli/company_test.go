package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/janmarkuslanger/gitvoice/internal/company"
)

func TestCompanyShowDefaultProfile(t *testing.T) {
	res := run(t, "company", "show")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr: %s", res.code, res.err)
	}
	// Nothing is set yet, so nothing but the small-business default applies.
	if strings.Contains(res.out, "Company") || strings.Contains(res.out, "Address") {
		t.Errorf("stdout = %q, want no fields for an unset profile", res.out)
	}
}

func TestCompanySetKeepsUntouchedFields(t *testing.T) {
	r, svc, out, errOut := newRunner(t)
	if code := runOn(r, "company", "set",
		"-company", "Studio Muster",
		`-address`, `Hauptstraße 2\n10115 Berlin`,
		"-tax-number", "12/345/67890",
		"-iban", "DE02120300000000202051",
		"-language", "de",
	); code != 0 {
		t.Fatalf("set: exit = %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Studio Muster") {
		t.Errorf("stdout = %q, want the issuer it stored", out.String())
	}

	// A second call touching one field must not clear the others.
	if code := runOn(r, "company", "set", "-vat-id", "DE123456789"); code != 0 {
		t.Fatalf("second set: exit = %d, stderr: %s", code, errOut.String())
	}
	got, err := svc.Company()
	if err != nil {
		t.Fatal(err)
	}
	want := company.Company{
		Company: "Studio Muster", Address: "Hauptstraße 2\n10115 Berlin",
		TaxNumber: "12/345/67890", VATID: "DE123456789",
		IBAN: "DE02120300000000202051", Language: "de",
		// company set starts from the stored profile, so the default § 19
		// UStG sentence of an unset profile is carried over.
		SmallBusinessNote: company.DefaultSmallBusinessNote,
	}
	if got != want {
		t.Errorf("profile = %+v, want %+v", got, want)
	}
}

func TestCompanySetClearsFieldExplicitly(t *testing.T) {
	r, svc, _, errOut := newRunner(t)
	if code := runOn(r, "company", "set", "-company", "Studio Muster", "-phone", "+49 30 123456"); code != 0 {
		t.Fatalf("set: exit = %d, stderr: %s", code, errOut.String())
	}
	if code := runOn(r, "company", "set", "-phone", ""); code != 0 {
		t.Fatalf("clear: exit = %d, stderr: %s", code, errOut.String())
	}
	got, err := svc.Company()
	if err != nil {
		t.Fatal(err)
	}
	if got.Phone != "" || got.Company != "Studio Muster" {
		t.Errorf("profile = %+v, want the phone cleared and the name kept", got)
	}
}

func TestCompanySetSmallBusinessAffectsNewInvoices(t *testing.T) {
	r, svc, _, errOut := newRunner(t)
	if code := runOn(r, "company", "set", "-company", "Studio Muster", "-small-business"); code != 0 {
		t.Fatalf("set: exit = %d, stderr: %s", code, errOut.String())
	}
	if code := runOn(r, "invoice", "add", "-number", "2026-020",
		"-customer-company", "ACME GmbH", "-item", "Consulting;1;100.00"); code != 0 {
		t.Fatalf("add: exit = %d, stderr: %s", code, errOut.String())
	}
	inv, err := svc.Invoice("2026-020")
	if err != nil {
		t.Fatal(err)
	}
	if !inv.SmallBusiness || inv.TaxCents() != 0 {
		t.Errorf("invoice = %+v, want the § 19 UStG default from the profile", inv)
	}

	// Turning it off again must not touch the invoice already written.
	if code := runOn(r, "company", "set", "-small-business=false"); code != 0 {
		t.Fatalf("unset: exit = %d, stderr: %s", code, errOut.String())
	}
	if inv, err = svc.Invoice("2026-020"); err != nil {
		t.Fatal(err)
	}
	if !inv.SmallBusiness {
		t.Error("existing invoice lost its snapshotted small-business flag")
	}
}

func TestCompanySetRejectsUnknownLanguage(t *testing.T) {
	res := run(t, "company", "set", "-company", "Studio Muster", "-language", "fr")
	if res.code != 1 {
		t.Fatalf("exit = %d, want 1", res.code)
	}
	if !strings.Contains(res.err, `unknown language "fr"`) {
		t.Errorf("stderr = %q, want the rejected code", res.err)
	}
	if _, err := res.svc.Company(); err != nil {
		t.Fatal(err)
	}
}

func TestCompanyShowJSON(t *testing.T) {
	r, _, out, errOut := newRunner(t)
	if code := runOn(r, "company", "set", "-company", "Studio Muster", "-vat-id", "DE123456789"); code != 0 {
		t.Fatalf("set: exit = %d, stderr: %s", code, errOut.String())
	}
	out.Reset()
	if code := runOn(r, "company", "show", "-json"); code != 0 {
		t.Fatalf("show: exit = %d, stderr: %s", code, errOut.String())
	}
	var got struct {
		Company string `json:"company"`
		VATID   string `json:"vat_id"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if got.Company != "Studio Muster" || got.VATID != "DE123456789" {
		t.Errorf("profile = %+v, want the stored fields", got)
	}
}

func TestCompanyShowPrintsProfile(t *testing.T) {
	r, _, out, errOut := newRunner(t)
	if code := runOn(r, "company", "set",
		"-company", "Studio Muster", `-address`, `Hauptstraße 2\n10115 Berlin`,
		"-tax-number", "12/345/67890", "-small-business",
	); code != 0 {
		t.Fatalf("set: exit = %d, stderr: %s", code, errOut.String())
	}
	out.Reset()
	if code := runOn(r, "company", "show"); code != 0 {
		t.Fatalf("show: exit = %d, stderr: %s", code, errOut.String())
	}
	for _, want := range []string{
		"Studio Muster", "Hauptstraße 2", "10115 Berlin", "12/345/67890",
		"§ 19 UStG", company.DefaultSmallBusinessNote,
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("show output does not mention %q:\n%s", want, out.String())
		}
	}
}

func TestInvoiceWarningPointsAtCompanySet(t *testing.T) {
	r, _, _, errOut := newRunner(t)
	if code := runOn(r, "invoice", "add", "-number", "2026-021",
		"-customer-company", "ACME GmbH", "-item", "Consulting;1;500.00"); code != 0 {
		t.Fatalf("add: exit = %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), `gitvoice company set`) {
		t.Errorf("stderr = %q, want the command that fills the issuer in", errOut.String())
	}
}
