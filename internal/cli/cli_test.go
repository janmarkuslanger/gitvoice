package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/janmarkuslanger/gitvoice/internal/company"
	"github.com/janmarkuslanger/gitvoice/internal/customer"
	"github.com/janmarkuslanger/gitvoice/internal/invoice"
	"github.com/janmarkuslanger/gitvoice/internal/invoicing"
	"github.com/janmarkuslanger/gitvoice/internal/store"
)

// result is what a command line did: its exit code, what it wrote, and the
// service to inspect the data directory it wrote into.
type result struct {
	code int
	out  string
	err  string
	svc  *invoicing.Service
}

func newRunner(t *testing.T) (Runner, *invoicing.Service, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	return newRunnerIn(t, t.TempDir())
}

// newRunnerIn returns a runner over dir, for tests that inspect the files it
// writes there.
func newRunnerIn(t *testing.T, dir string) (Runner, *invoicing.Service, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	st, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	svc := invoicing.New(st)
	var out, errOut bytes.Buffer
	return Runner{Svc: svc, Out: &out, Err: &errOut}, svc, &out, &errOut
}

func run(t *testing.T, args ...string) result {
	t.Helper()
	r, svc, out, errOut := newRunner(t)
	code := r.Run(args)
	return result{code: code, out: out.String(), err: errOut.String(), svc: svc}
}

// runOn reuses an existing runner, for commands that build on each other.
func runOn(r Runner, args ...string) int {
	return r.Run(args)
}

func TestCustomerAdd(t *testing.T) {
	res := run(t, "customer", "add",
		"-company", "ACME GmbH",
		"-first-name", "Max",
		"-last-name", "Muster",
		`-address`, `Musterstraße 1\n12345 Berlin`,
		"-email", "billing@acme.example",
		"-phone", "+49 30 123456",
		"-vat-id", "DE123456789",
		"-notes", "prefers email",
	)
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr: %s", res.code, res.err)
	}
	if !strings.Contains(res.out, "customer acme-gmbh created") {
		t.Errorf("stdout = %q, want a confirmation naming the derived id", res.out)
	}
	got, err := res.svc.Customer("acme-gmbh")
	if err != nil {
		t.Fatal(err)
	}
	want := customer.Customer{
		ID: "acme-gmbh", Company: "ACME GmbH", FirstName: "Max", LastName: "Muster",
		Address: "Musterstraße 1\n12345 Berlin", Email: "billing@acme.example",
		Phone: "+49 30 123456", VATID: "DE123456789", Notes: "prefers email",
	}
	if got != want {
		t.Errorf("customer = %+v, want %+v", got, want)
	}
}

func TestCustomerAddDerivesIDFromCompany(t *testing.T) {
	res := run(t, "customer", "add", "-company", "Müller & Söhne GmbH")
	if res.code != 0 {
		t.Fatalf("exit = %d, stderr: %s", res.code, res.err)
	}
	if !strings.Contains(res.out, "customer mueller-soehne-gmbh created") {
		t.Errorf("stdout = %q, want the derived id", res.out)
	}
	if _, err := res.svc.Customer("mueller-soehne-gmbh"); err != nil {
		t.Errorf("derived id not stored: %v", err)
	}
}

func TestCustomerAddNumbersDerivedIDOnCollision(t *testing.T) {
	r, svc, out, errOut := newRunner(t)
	for i := 0; i < 2; i++ {
		if code := runOn(r, "customer", "add", "-company", "ACME GmbH"); code != 0 {
			t.Fatalf("add %d: exit = %d, stderr: %s", i, code, errOut.String())
		}
	}
	if !strings.Contains(out.String(), "customer acme-gmbh created") ||
		!strings.Contains(out.String(), "customer acme-gmbh-2 created") {
		t.Errorf("stdout = %q, want acme-gmbh and acme-gmbh-2", out.String())
	}
	for _, id := range []string{"acme-gmbh", "acme-gmbh-2"} {
		if _, err := svc.Customer(id); err != nil {
			t.Errorf("customer %s missing: %v", id, err)
		}
	}
}

func TestCustomerAddRejectsMissingName(t *testing.T) {
	res := run(t, "customer", "add", "-email", "billing@acme.example")
	if res.code != 1 {
		t.Fatalf("exit = %d, want 1", res.code)
	}
	if !strings.Contains(res.err, "company or last name is required") {
		t.Errorf("stderr = %q, want the validation message", res.err)
	}
}

func TestCustomerList(t *testing.T) {
	r, _, out, errOut := newRunner(t)
	if code := runOn(r, "customer", "list"); code != 0 {
		t.Fatalf("empty list: exit = %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "no customers yet") {
		t.Errorf("stdout = %q, want a note that there is nothing yet", out.String())
	}

	for _, args := range [][]string{
		{"-company", "ACME GmbH", "-email", "billing@acme.example"},
		{"-first-name", "Max", "-last-name", "Muster"},
	} {
		if code := runOn(r, append([]string{"customer", "add"}, args...)...); code != 0 {
			t.Fatalf("add: exit = %d, stderr: %s", code, errOut.String())
		}
	}
	out.Reset()
	if code := runOn(r, "customer", "list"); code != 0 {
		t.Fatalf("list: exit = %d, stderr: %s", code, errOut.String())
	}

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want a header and two customers:\n%s", len(lines), out.String())
	}
	for i, want := range []string{
		"ID", "COMPANY", "NAME", "EMAIL",
	} {
		if !strings.Contains(lines[0], want) {
			t.Errorf("header column %d (%q) missing from %q", i, want, lines[0])
		}
	}
	// ListCustomers sorts by display name: "ACME GmbH" before "Max Muster".
	for _, want := range []string{"acme-gmbh", "ACME GmbH", "billing@acme.example"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("row %q does not contain %q", lines[1], want)
		}
	}
	for _, want := range []string{"max-muster", "Max Muster"} {
		if !strings.Contains(lines[2], want) {
			t.Errorf("row %q does not contain %q", lines[2], want)
		}
	}
}

func TestInvoiceAddCopiesCustomerAndDefaults(t *testing.T) {
	r, svc, out, errOut := newRunner(t)
	if _, err := svc.CreateCustomer(customer.Customer{
		Company: "ACME GmbH", Address: "Musterstraße 1", VATID: "DE123456789",
		Phone: "+49 30 123456", Notes: "internal",
	}); err != nil {
		t.Fatal(err)
	}
	code := runOn(r, "invoice", "add",
		"-number", "2026-001",
		"-customer", "acme-gmbh",
		"-item", "Consulting;3;120.00",
		"-item", "Travel;1;49,50",
	)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errOut.String())
	}

	inv, err := svc.Invoice("2026-001")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Date != time.Now().Format(dateLayout) {
		t.Errorf("date = %q, want today", inv.Date)
	}
	if inv.Status != invoice.StatusDraft || inv.Currency != "EUR" {
		t.Errorf("status/currency = %q/%q, want draft/EUR", inv.Status, inv.Currency)
	}
	if inv.TaxRatePercent != invoicing.DefaultTaxRatePercent {
		t.Errorf("tax rate = %v, want %d from the profile", inv.TaxRatePercent, invoicing.DefaultTaxRatePercent)
	}
	wantCustomer := invoice.Customer{Company: "ACME GmbH", Address: "Musterstraße 1", VATID: "DE123456789"}
	if inv.Customer != wantCustomer {
		t.Errorf("customer = %+v, want %+v (phone and notes stay master data)", inv.Customer, wantCustomer)
	}
	wantItems := []invoice.Item{
		{Description: "Consulting", Quantity: 3, UnitPriceCents: 12000},
		{Description: "Travel", Quantity: 1, UnitPriceCents: 4950},
	}
	if len(inv.Items) != len(wantItems) {
		t.Fatalf("items = %+v, want %+v", inv.Items, wantItems)
	}
	for i, want := range wantItems {
		if inv.Items[i] != want {
			t.Errorf("item %d = %+v, want %+v", i, inv.Items[i], want)
		}
	}
	if inv.TotalCents() != 48731 { // 40950 net + 19 % VAT, rounded to the cent
		t.Errorf("total = %d, want 48731", inv.TotalCents())
	}
	if !strings.Contains(out.String(), "invoice 2026-001 created (487.31 EUR)") {
		t.Errorf("stdout = %q, want the total", out.String())
	}
}

func TestInvoiceAddWithoutMasterData(t *testing.T) {
	r, svc, _, errOut := newRunner(t)
	code := runOn(r, "invoice", "add",
		"-number", "2026-002",
		"-customer-last-name", "Muster",
		"-customer-first-name", "Max",
		`-customer-address`, `Musterstraße 1\n12345 Berlin`,
		"-date", "2026-08-07",
		"-due-date", "2026-08-21",
		"-status", "sent",
		"-currency", "CHF",
		"-service-date", "2026-08-01",
		"-tax-rate", "7,7",
		"-notes", `Thanks!\nPlease pay on time.`,
		"-item", "Consulting;1;100.00",
	)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errOut.String())
	}
	inv, err := svc.Invoice("2026-002")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Customer.Address != "Musterstraße 1\n12345 Berlin" {
		t.Errorf("address = %q, want two lines", inv.Customer.Address)
	}
	if inv.Notes != "Thanks!\nPlease pay on time." {
		t.Errorf("notes = %q, want two lines", inv.Notes)
	}
	if inv.Status != invoice.StatusSent || inv.Currency != "CHF" || inv.DueDate != "2026-08-21" {
		t.Errorf("invoice = %+v, want the flags applied", inv)
	}
	if inv.TaxRatePercent != 7.7 || inv.TaxCents() != 770 {
		t.Errorf("tax = %v%% / %d cents, want 7.7%% / 770", inv.TaxRatePercent, inv.TaxCents())
	}
}

func TestInvoiceAddOverridesLoadedCustomer(t *testing.T) {
	r, svc, _, errOut := newRunner(t)
	if _, err := svc.CreateCustomer(customer.Customer{Company: "ACME GmbH", Email: "old@acme.example"}); err != nil {
		t.Fatal(err)
	}
	code := runOn(r, "invoice", "add",
		"-number", "2026-003", "-customer", "acme-gmbh",
		"-customer-email", "new@acme.example",
		"-item", "Consulting;1;100.00",
	)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errOut.String())
	}
	inv, err := svc.Invoice("2026-003")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Customer.Company != "ACME GmbH" || inv.Customer.Email != "new@acme.example" {
		t.Errorf("customer = %+v, want the company kept and the email overridden", inv.Customer)
	}
}

func TestInvoiceAddSmallBusinessIgnoresTaxRate(t *testing.T) {
	r, svc, _, errOut := newRunner(t)
	code := runOn(r, "invoice", "add",
		"-number", "2026-004", "-customer-company", "ACME GmbH",
		"-small-business", "-tax-rate", "19",
		"-item", "Consulting;1;100.00",
	)
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errOut.String())
	}
	inv, err := svc.Invoice("2026-004")
	if err != nil {
		t.Fatal(err)
	}
	if !inv.SmallBusiness || inv.TaxRatePercent != 0 || inv.TaxCents() != 0 {
		t.Errorf("invoice = %+v, want no VAT under § 19 UStG", inv)
	}
}

func TestInvoiceAddSnapshotsSmallBusinessProfile(t *testing.T) {
	r, svc, _, errOut := newRunner(t)
	if err := svc.SaveCompany(company.Company{SmallBusiness: true}); err != nil {
		t.Fatal(err)
	}
	code := runOn(r, "invoice", "add", "-number", "2026-005", "-customer-company", "ACME GmbH",
		"-item", "Consulting;1;100.00")
	if code != 0 {
		t.Fatalf("exit = %d, stderr: %s", code, errOut.String())
	}
	inv, err := svc.Invoice("2026-005")
	if err != nil {
		t.Fatal(err)
	}
	if !inv.SmallBusiness || inv.TaxRatePercent != 0 {
		t.Errorf("invoice = %+v, want the profile's small-business default", inv)
	}
}

func TestInvoiceAddUnknownCustomer(t *testing.T) {
	res := run(t, "invoice", "add", "-number", "2026-006", "-customer", "ghost", "-item", "Consulting;1;100.00")
	if res.code != 1 {
		t.Fatalf("exit = %d, want 1", res.code)
	}
	if !strings.Contains(res.err, "not found") {
		t.Errorf("stderr = %q, want a not-found message", res.err)
	}
}

func TestInvoiceAddRejectsInvalid(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing number", []string{"-customer-company", "ACME GmbH"}, "number is required"},
		{"bad status", []string{"-number", "x", "-customer-company", "A", "-status", "nope"}, `status "nope" is not valid`},
		{"bad date", []string{"-number", "x", "-customer-company", "A", "-date", "07.08.2026"}, "date must be a valid date"},
		{"no customer", []string{"-number", "x"}, "customer company or last name is required"},
		{"bad item", []string{"-number", "x", "-customer-company", "A", "-item", "Consulting;3"}, "expected"},
		{"bad quantity", []string{"-number", "x", "-customer-company", "A", "-item", "Consulting;many;12"}, "invalid quantity"},
		{"bad unit price", []string{"-number", "x", "-customer-company", "A", "-item", "Consulting;1;free"}, "invalid unit price"},
		{"bad tax rate", []string{"-number", "x", "-customer-company", "A", "-tax-rate", "high"}, "invalid tax rate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := run(t, append([]string{"invoice", "add"}, tt.args...)...)
			if res.code != 1 {
				t.Fatalf("exit = %d, want 1", res.code)
			}
			if !strings.Contains(res.err, tt.want) {
				t.Errorf("stderr = %q, want it to mention %q", res.err, tt.want)
			}
		})
	}
}

func TestParseItemKeepsSemicolonInDescription(t *testing.T) {
	item, err := parseItem("Consulting; incl. travel;2;80.00")
	if err != nil {
		t.Fatal(err)
	}
	want := invoice.Item{Description: "Consulting; incl. travel", Quantity: 2, UnitPriceCents: 8000}
	if item != want {
		t.Errorf("item = %+v, want %+v", item, want)
	}
}

func TestServeCommand(t *testing.T) {
	r, _, _, _ := newRunner(t)
	called := false
	r.Serve = func() error { called = true; return nil }
	if code := r.Run([]string{"serve"}); code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !called {
		t.Error("serve command did not start the web UI")
	}
}

func TestUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no arguments", nil, "Usage:"},
		{"unknown command", []string{"nope"}, `unknown command "nope"`},
		{"unknown subcommand", []string{"customer", "remove"}, `unknown customer command "remove"`},
		{"missing customer subcommand", []string{"customer"}, "usage: gitvoice customer <add|list>"},
		{"missing subcommand", []string{"invoice"}, "usage: gitvoice invoice <add|list|show|pdf>"},
		{"unknown invoice subcommand", []string{"invoice", "remove"}, `unknown invoice command "remove"`},
		{"missing invoice number", []string{"invoice", "show"}, "usage: gitvoice invoice show [flags] <number>"},
		{"number and stray flag order", []string{"invoice", "show", "2026-001", "-json"}, "usage: gitvoice invoice show [flags] <number>"},
		{"unknown flag", []string{"customer", "add", "-nope"}, "not defined"},
		{"id is not a flag", []string{"customer", "add", "-id", "acme"}, "not defined"},
		{"stray argument", []string{"customer", "list", "acme"}, `unexpected argument "acme"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := run(t, tt.args...)
			if res.code != 2 {
				t.Fatalf("exit = %d, want 2", res.code)
			}
			if !strings.Contains(res.err, tt.want) {
				t.Errorf("stderr = %q, want it to mention %q", res.err, tt.want)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	res := run(t, "help")
	if res.code != 0 {
		t.Fatalf("exit = %d, want 0", res.code)
	}
	for _, want := range []string{
		"customer add", "customer list",
		"invoice add", "invoice list", "invoice show", "invoice pdf",
		"serve", "-json",
	} {
		if !strings.Contains(res.out, want) {
			t.Errorf("help does not mention %q", want)
		}
	}
}

func TestCommandHelpFlagExitsZero(t *testing.T) {
	res := run(t, "invoice", "add", "-h")
	if res.code != 0 {
		t.Fatalf("exit = %d, want 0", res.code)
	}
	if !strings.Contains(res.err, "-item") {
		t.Errorf("stderr = %q, want the flag list", res.err)
	}
}
