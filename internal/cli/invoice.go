package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/janmarkuslanger/gitvoice/internal/invoice"
	"github.com/janmarkuslanger/gitvoice/internal/money"
)

// invoiceAdd writes a new invoice. Tax defaults come from the company
// profile via NewDraft, so the CLI and the web UI snapshot the same values.
// -customer prefills the recipient from the master data; the -customer-*
// flags fill in or override individual fields, as in the web form.
//
// Mandatory details missing under § 14 UStG are reported afterwards, like
// the invoice view does. The check is advisory: the invoice is written and
// the command succeeds either way.
func (r Runner) invoiceAdd(args []string) error {
	inv, err := r.Svc.NewDraft()
	if err != nil {
		return err
	}
	fs := r.flagSet("invoice add")
	var (
		customerID string
		status     = string(inv.Status)
		taxRate    = money.FormatQuantity(inv.TaxRatePercent)
		override   invoice.Customer
		items      itemFlag
	)
	fs.StringVar(&inv.Number, "number", "", "invoice number, doubles as the file name (required)")
	fs.StringVar(&inv.Date, "date", time.Now().Format(dateLayout), "invoice date (YYYY-MM-DD)")
	fs.StringVar(&inv.DueDate, "due-date", "", "payment due date (YYYY-MM-DD)")
	fs.StringVar(&status, "status", status, "draft, sent, paid or canceled")
	fs.StringVar(&inv.Currency, "currency", inv.Currency, "currency code")
	fs.StringVar(&inv.ServiceDate, "service-date", "", "time of supply (YYYY-MM-DD)")
	fs.StringVar(&inv.ServicePeriodStart, "service-period-start", "", "start of the service period (YYYY-MM-DD)")
	fs.StringVar(&inv.ServicePeriodEnd, "service-period-end", "", "end of the service period (YYYY-MM-DD)")
	fs.BoolVar(&inv.SmallBusiness, "small-business", inv.SmallBusiness, "§ 19 UStG: issue without VAT")
	fs.StringVar(&taxRate, "tax-rate", taxRate, "VAT rate in percent, ignored under -small-business")
	fs.StringVar(&inv.Notes, "notes", "", `note printed on the invoice, \n starts a new line`)
	fs.StringVar(&customerID, "customer", "", "customer ID to copy the recipient from")
	fs.StringVar(&override.Company, "customer-company", "", "recipient company name")
	fs.StringVar(&override.FirstName, "customer-first-name", "", "recipient given name")
	fs.StringVar(&override.LastName, "customer-last-name", "", "recipient family name")
	fs.StringVar(&override.Address, "customer-address", "", `recipient address, \n starts a new line`)
	fs.StringVar(&override.Email, "customer-email", "", "recipient email address")
	fs.StringVar(&override.VATID, "customer-vat-id", "", "recipient USt-IdNr.")
	fs.Var(&items, "item", `line item "description;quantity;unit price", repeatable`)
	if err := parse(fs, args); err != nil {
		return err
	}

	inv.Status = invoice.Status(status)
	inv.Notes = unescapeNewlines(inv.Notes)
	if customerID != "" {
		if inv.Customer, err = r.Svc.CustomerSnapshot(customerID); err != nil {
			return err
		}
	}
	override.Address = unescapeNewlines(override.Address)
	applyOverrides(&inv.Customer, override)

	// A small-business invoice carries no VAT, so -tax-rate does not apply.
	if inv.SmallBusiness {
		inv.TaxRatePercent = 0
	} else if inv.TaxRatePercent, err = money.ParseDecimal(taxRate); err != nil {
		return fmt.Errorf("invalid tax rate %q", taxRate)
	}
	for _, raw := range items {
		item, err := parseItem(raw)
		if err != nil {
			return err
		}
		inv.Items = append(inv.Items, item)
	}

	if err := r.Svc.CreateInvoice(inv); err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "invoice %s created (%s %s)\n", inv.Number, money.FormatCents(inv.TotalCents()), inv.Currency)
	return r.reportWarnings(inv)
}

// reportWarnings writes the § 14 UStG completeness check of a just-written
// invoice to the error stream. A missing detail is a warning, never a
// failure: it does not stop the invoice from being stored, and a broken
// profile read is reported instead of hiding the warnings.
func (r Runner) reportWarnings(inv invoice.Invoice) error {
	comp, err := r.Svc.Company()
	if err != nil {
		return err
	}
	warnings := view(inv, comp).Warnings
	if len(warnings) == 0 {
		return nil
	}
	fmt.Fprintf(r.Err, "gitvoice: warning: %s\n", warningHeading)
	for _, w := range warnings {
		fmt.Fprintf(r.Err, "  - %s\n", w.Message)
	}
	if hint := issuerHint(warnings); hint != "" {
		fmt.Fprintf(r.Err, "  (%s)\n", hint)
	}
	return nil
}

// itemFlag collects repeated -item flags in the order they were given.
type itemFlag []string

func (f *itemFlag) String() string { return strings.Join(*f, ", ") }

func (f *itemFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}

// parseItem reads "<description>;<quantity>;<unit price>". Quantity and unit
// price are taken from the end, so a description may itself contain ';'.
func parseItem(s string) (invoice.Item, error) {
	malformed := fmt.Errorf("item %q: expected \"description;quantity;unit price\"", s)
	rest, price, ok := cutLast(s, ";")
	if !ok {
		return invoice.Item{}, malformed
	}
	desc, quantity, ok := cutLast(rest, ";")
	if !ok {
		return invoice.Item{}, malformed
	}
	q, err := money.ParseDecimal(quantity)
	if err != nil {
		return invoice.Item{}, fmt.Errorf("item %q: invalid quantity %q", s, quantity)
	}
	cents, err := money.ParseCents(price)
	if err != nil {
		return invoice.Item{}, fmt.Errorf("item %q: invalid unit price %q", s, price)
	}
	return invoice.Item{
		Description:    strings.TrimSpace(desc),
		Quantity:       q,
		UnitPriceCents: cents,
	}, nil
}

// cutLast splits s around the last instance of sep.
func cutLast(s, sep string) (before, after string, found bool) {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return s, "", false
	}
	return s[:i], s[i+len(sep):], true
}

// applyOverrides copies the fields the user set onto the recipient loaded
// from the master data. Empty means "keep", so clearing a field that the
// master data provides is not expressible — edit the invoice for that.
func applyOverrides(dst *invoice.Customer, src invoice.Customer) {
	for _, f := range []struct {
		dst *string
		src string
	}{
		{&dst.Company, src.Company},
		{&dst.FirstName, src.FirstName},
		{&dst.LastName, src.LastName},
		{&dst.Address, src.Address},
		{&dst.Email, src.Email},
		{&dst.VATID, src.VATID},
	} {
		if f.src != "" {
			*f.dst = f.src
		}
	}
}
