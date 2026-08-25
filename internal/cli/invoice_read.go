package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/janmarkuslanger/gitvoice/internal/company"
	"github.com/janmarkuslanger/gitvoice/internal/i18n"
	"github.com/janmarkuslanger/gitvoice/internal/invoice"
	"github.com/janmarkuslanger/gitvoice/internal/invoicing"
	"github.com/janmarkuslanger/gitvoice/internal/money"
)

// invoiceView is the -json shape of an invoice: the stored record plus the
// values a caller would otherwise have to recompute — the totals and the
// § 14 UStG completeness check. It is the command line's output contract:
// fields may be added, but renaming or dropping one breaks scripts.
type invoiceView struct {
	invoice.Invoice
	Net      int64         `json:"net_cents"`
	Tax      int64         `json:"tax_cents"`
	Total    int64         `json:"total_cents"`
	Warnings []warningView `json:"warnings"`
}

// warningView reports a missing mandatory detail with both its stable code
// and the English text, so a caller can branch on the one and print the other.
type warningView struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// The completeness check yields i18n keys. The command line is English, so
// it renders them with the English dictionary; the profile language governs
// the web UI and the PDF only.
var warningHeading = i18n.T(i18n.EN, "compliance.heading")

func warningText(w invoicing.Warning) string { return i18n.T(i18n.EN, string(w)) }

// view pairs an invoice with its totals and warnings. The issuer profile is
// passed in so listing many invoices reads it once.
func view(inv invoice.Invoice, comp company.Company) invoiceView {
	v := invoiceView{
		Invoice:  inv,
		Net:      inv.NetCents(),
		Tax:      inv.TaxCents(),
		Total:    inv.TotalCents(),
		Warnings: []warningView{}, // an empty array, not null
	}
	for _, w := range invoicing.ComplianceWarnings(inv, comp) {
		v.Warnings = append(v.Warnings, warningView{Code: string(w), Message: warningText(w)})
	}
	return v
}

// invoiceList prints all invoices, newest first: a table for reading, or the
// full records with their totals under -json.
func (r Runner) invoiceList(args []string) error {
	fs := r.flagSet("invoice list")
	asJSON := fs.Bool("json", false, "print the invoices as a JSON array")
	if err := parse(fs, args); err != nil {
		return err
	}
	invoices, err := r.Svc.Invoices()
	if err != nil {
		return err
	}
	if *asJSON {
		comp, err := r.Svc.Company()
		if err != nil {
			return err
		}
		views := make([]invoiceView, 0, len(invoices))
		for _, inv := range invoices {
			views = append(views, view(inv, comp))
		}
		return r.printJSON(views)
	}
	if len(invoices) == 0 {
		fmt.Fprintln(r.Out, "no invoices yet")
		return nil
	}
	w := tabwriter.NewWriter(r.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NUMBER\tDATE\tSTATUS\tRECIPIENT\tTOTAL")
	for _, inv := range invoices {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s %s\n",
			inv.Number, inv.Date, inv.Status, inv.Customer.DisplayName(),
			money.FormatCents(inv.TotalCents()), inv.Currency)
	}
	return w.Flush()
}

// invoiceShow prints one invoice with its totals and its § 14 UStG
// completeness check, or the same data as JSON under -json.
func (r Runner) invoiceShow(args []string) error {
	fs := r.flagSet("invoice show")
	asJSON := fs.Bool("json", false, "print the invoice as JSON")
	rest, err := parseArgs(fs, args, 1, "<number>")
	if err != nil {
		return err
	}
	inv, err := r.Svc.Invoice(rest[0])
	if err != nil {
		return err
	}
	comp, err := r.Svc.Company()
	if err != nil {
		return err
	}
	v := view(inv, comp)
	if *asJSON {
		return r.printJSON(v)
	}
	r.printInvoice(v)
	return nil
}

// printInvoice writes the human-readable form of an invoice: header fields,
// the line items, the totals, and the missing mandatory details.
func (r Runner) printInvoice(v invoiceView) {
	w := tabwriter.NewWriter(r.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Invoice\t%s\n", v.Number)
	fmt.Fprintf(w, "Date\t%s\n", v.Date)
	if v.DueDate != "" {
		fmt.Fprintf(w, "Due\t%s\n", v.DueDate)
	}
	if supply := timeOfSupply(v.Invoice); supply != "" {
		fmt.Fprintf(w, "Time of supply\t%s\n", supply)
	}
	fmt.Fprintf(w, "Status\t%s\n", v.Status)
	for i, line := range recipientLines(v.Customer) {
		label := ""
		if i == 0 {
			label = "Recipient"
		}
		fmt.Fprintf(w, "%s\t%s\n", label, line)
	}
	for i, it := range v.Items {
		label := ""
		if i == 0 {
			label = "Items"
		}
		fmt.Fprintf(w, "%s\t%s x %s\t%s\t%s\n", label,
			money.FormatQuantity(it.Quantity), money.FormatCents(it.UnitPriceCents),
			it.Description, money.FormatCents(it.TotalCents()))
	}
	fmt.Fprintf(w, "Net\t%s %s\n", money.FormatCents(v.Net), v.Currency)
	if v.SmallBusiness {
		fmt.Fprintf(w, "VAT\tnone (§ 19 UStG)\n")
	} else {
		fmt.Fprintf(w, "VAT %s%%\t%s %s\n", money.FormatQuantity(v.TaxRatePercent), money.FormatCents(v.Tax), v.Currency)
	}
	fmt.Fprintf(w, "Total\t%s %s\n", money.FormatCents(v.Total), v.Currency)
	if v.Notes != "" {
		fmt.Fprintf(w, "Notes\t%s\n", strings.ReplaceAll(v.Notes, "\n", " / "))
	}
	// Errors from a tabwriter over the command's own output stream are the
	// caller's broken pipe, not something this command can act on.
	_ = w.Flush()
	if len(v.Warnings) == 0 {
		return
	}
	fmt.Fprintf(r.Out, "\n%s:\n", warningHeading)
	for _, warn := range v.Warnings {
		fmt.Fprintf(r.Out, "  - %s\n", warn.Message)
	}
	if hint := issuerHint(v.Warnings); hint != "" {
		fmt.Fprintf(r.Out, "  (%s)\n", hint)
	}
}

// issuerHint names the command that fixes the issuer-side warnings. The
// translated texts point at the Settings page, which a caller on the command
// line does not have.
func issuerHint(warnings []warningView) string {
	for _, w := range warnings {
		if strings.HasPrefix(w.Code, "compliance.issuer") {
			return `fill the issuer fields in with "gitvoice company set"`
		}
	}
	return ""
}

// timeOfSupply renders the service date or period as one field.
func timeOfSupply(inv invoice.Invoice) string {
	if inv.HasServicePeriod() {
		return inv.ServicePeriodStart + " – " + inv.ServicePeriodEnd
	}
	return inv.ServiceDate
}

// recipientLines returns the recipient block: name, person, address lines
// and contact details, skipping the fields that are empty.
func recipientLines(c invoice.Customer) []string {
	lines := []string{c.DisplayName()}
	if c.Company != "" && c.PersonName() != "" {
		lines = append(lines, c.PersonName())
	}
	for _, line := range strings.Split(c.Address, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	for _, s := range []string{c.Email, c.VATID} {
		if s != "" {
			lines = append(lines, s)
		}
	}
	return lines
}

// invoicePDF renders an invoice and reports the file it was written to.
// Rendering is idempotent: it overwrites the previous PDF of that number.
func (r Runner) invoicePDF(args []string) error {
	fs := r.flagSet("invoice pdf")
	lang := fs.String("lang", "", "label language ("+strings.Join(i18n.Codes, ", ")+"), default from the company profile")
	rest, err := parseArgs(fs, args, 1, "<number>")
	if err != nil {
		return err
	}
	code, err := r.pdfLang(*lang)
	if err != nil {
		return err
	}
	data, err := r.Svc.GeneratePDF(rest[0], code)
	if err != nil {
		return err
	}
	path, err := r.Svc.PDFPath(rest[0])
	if err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "invoice %s written to %s (%d bytes)\n", rest[0], path, len(data))
	return nil
}

// pdfLang resolves the PDF label language: the flag wins, then the profile
// default, then the application fallback — as in the web UI, minus its
// per-browser language switch.
func (r Runner) pdfLang(flagValue string) (string, error) {
	if flagValue != "" {
		code, ok := i18n.Parse(flagValue)
		if !ok {
			return "", fmt.Errorf("unknown language %q, want %s", flagValue, strings.Join(i18n.Codes, " or "))
		}
		return code, nil
	}
	comp, err := r.Svc.Company()
	if err != nil {
		return "", err
	}
	if code, ok := i18n.Parse(comp.Language); ok {
		return code, nil
	}
	return i18n.Fallback, nil
}
