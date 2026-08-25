package cli

import (
	"errors"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/janmarkuslanger/gitvoice/internal/company"
	"github.com/janmarkuslanger/gitvoice/internal/i18n"
)

// company dispatches the issuer profile commands. § 14 UStG requires most of
// the profile on every invoice, so a caller that only has the command line
// can fill it in here instead of on the Settings page.
func (r Runner) company(args []string) error {
	if len(args) == 0 {
		return usageError{errors.New("usage: gitvoice company <show|set> [flags]")}
	}
	switch args[0] {
	case "show":
		return r.companyShow(args[1:])
	case "set":
		return r.companySet(args[1:])
	}
	return usageError{fmt.Errorf("unknown company command %q, want show or set", args[0])}
}

// companyShow prints the issuer profile, skipping the fields that are empty.
func (r Runner) companyShow(args []string) error {
	fs := r.flagSet("company show")
	asJSON := fs.Bool("json", false, "print the profile as JSON")
	if err := parse(fs, args); err != nil {
		return err
	}
	comp, err := r.Svc.Company()
	if err != nil {
		return err
	}
	if *asJSON {
		return r.printJSON(comp)
	}
	// Letterhead order: who the issuer is, where they are, then how to
	// reach and pay them.
	rows := [][2]string{
		{"Company", comp.Company},
		{"Name", comp.PersonName()},
	}
	rows = append(rows, addressRows(comp.Address)...)
	rows = append(rows,
		[2]string{"Email", comp.Email},
		[2]string{"Phone", comp.Phone},
		[2]string{"Tax number", comp.TaxNumber},
		[2]string{"VAT ID", comp.VATID},
		[2]string{"IBAN", comp.IBAN},
		[2]string{"BIC", comp.BIC},
		[2]string{"Bank", comp.BankName},
		[2]string{"Language", comp.Language},
	)
	if comp.SmallBusiness {
		rows = append(rows, [2]string{"Small business", "§ 19 UStG: " + comp.Note()})
	}
	w := tabwriter.NewWriter(r.Out, 0, 0, 2, ' ', 0)
	for _, row := range rows {
		if row[1] != "" {
			fmt.Fprintf(w, "%s\t%s\n", row[0], row[1])
		}
	}
	return w.Flush()
}

// companySet updates the issuer profile. Every flag defaults to the value
// currently stored, so setting one field keeps the rest and passing an empty
// string clears a single field on purpose.
func (r Runner) companySet(args []string) error {
	comp, err := r.Svc.Company()
	if err != nil {
		return err
	}
	fs := r.flagSet("company set")
	lang := comp.Language
	fs.StringVar(&comp.Company, "company", comp.Company, "business name")
	fs.StringVar(&comp.FirstName, "first-name", comp.FirstName, "given name")
	fs.StringVar(&comp.LastName, "last-name", comp.LastName, "family name")
	fs.StringVar(&comp.Address, "address", comp.Address, `postal address, \n starts a new line`)
	fs.StringVar(&comp.Email, "email", comp.Email, "email address")
	fs.StringVar(&comp.Phone, "phone", comp.Phone, "phone number")
	fs.StringVar(&comp.TaxNumber, "tax-number", comp.TaxNumber, "Steuernummer")
	fs.StringVar(&comp.VATID, "vat-id", comp.VATID, "USt-IdNr.")
	fs.StringVar(&comp.IBAN, "iban", comp.IBAN, "IBAN printed on the invoice")
	fs.StringVar(&comp.BIC, "bic", comp.BIC, "BIC printed on the invoice")
	fs.StringVar(&comp.BankName, "bank-name", comp.BankName, "bank name printed on the invoice")
	fs.BoolVar(&comp.SmallBusiness, "small-business", comp.SmallBusiness, "§ 19 UStG: new invoices default to no VAT")
	fs.StringVar(&comp.SmallBusinessNote, "small-business-note", comp.SmallBusinessNote,
		"§ 19 UStG sentence printed on such invoices, empty for the default")
	fs.StringVar(&lang, "language", lang, "default UI and PDF language ("+strings.Join(i18n.Codes, ", ")+")")
	if err := parse(fs, args); err != nil {
		return err
	}
	comp.Address = unescapeNewlines(comp.Address)
	comp.SmallBusinessNote = unescapeNewlines(comp.SmallBusinessNote)
	// An unsupported code would silently fall back to English on every
	// invoice, so it is rejected rather than dropped.
	if lang != "" {
		code, ok := i18n.Parse(lang)
		if !ok {
			return fmt.Errorf("unknown language %q, want %s", lang, strings.Join(i18n.Codes, " or "))
		}
		comp.Language = code
	} else {
		comp.Language = ""
	}
	if err := r.Svc.SaveCompany(comp); err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "company profile saved (issuer: %s)\n", issuerOrPlaceholder(comp))
	return nil
}

// issuerOrPlaceholder names the issuer the profile now prints, or says that
// it still has none — the first § 14 UStG detail a caller has to fix.
func issuerOrPlaceholder(comp company.Company) string {
	if name := comp.DisplayName(); name != "" {
		return name
	}
	return "no name set"
}

// addressRows splits a multi-line address into label/value rows, labelling
// the first line only, so the block lines up under one heading.
func addressRows(address string) [][2]string {
	var rows [][2]string
	for _, line := range strings.Split(address, "\n") {
		if line == "" {
			continue
		}
		label := ""
		if len(rows) == 0 {
			label = "Address"
		}
		rows = append(rows, [2]string{label, line})
	}
	return rows
}
