// Package cli implements the gitvoice command line over the invoicing
// service: the same use cases the web UI drives, for scripts and terminals.
// Every command maps onto invoicing.Service calls, so the rules stay in one
// place and both frontends behave identically.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/janmarkuslanger/gitvoice/internal/customer"
	"github.com/janmarkuslanger/gitvoice/internal/invoice"
	"github.com/janmarkuslanger/gitvoice/internal/invoicing"
	"github.com/janmarkuslanger/gitvoice/internal/money"
)

// Usage is the top-level help text. The global flags it documents are parsed
// by the command binary, before the command name.
const Usage = `gitvoice manages invoices as JSON files inside your git repository.

Usage:
  gitvoice [-data <dir>] [-addr <host:port>] <command> [flags]

Commands:
  customer add    add a customer to the master data
  customer list   list the customers with their IDs
  invoice add     write a new invoice
  serve           serve the web UI
  help            show this text

Run "gitvoice <command> -h" for the flags of a command.
`

// Runner executes commands against one data directory. Serve starts the web
// UI and blocks; when it is nil the serve command reports itself unavailable.
type Runner struct {
	Svc   *invoicing.Service
	Serve func() error
	Out   io.Writer
	Err   io.Writer
}

// errReported marks an error the flag package already wrote to the output,
// so Run does not print it a second time.
var errReported = errors.New("already reported")

// usageError is a wrong invocation rather than a failed operation. It is
// printed like any other error but exits with the conventional code 2.
type usageError struct{ error }

// Run executes one command line and returns the process exit code: 0 on
// success, 1 when the command failed, 2 when it was invoked wrongly.
func (r Runner) Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(r.Err, Usage)
		return 2
	}
	err := r.dispatch(args)
	var wrongUsage usageError
	switch {
	case err == nil:
		return 0
	case errors.Is(err, flag.ErrHelp):
		return 0 // -h: the flag set printed its own usage
	case errors.Is(err, errReported):
		return 2
	case errors.As(err, &wrongUsage):
		fmt.Fprintf(r.Err, "gitvoice: %v\n", err)
		return 2
	default:
		fmt.Fprintf(r.Err, "gitvoice: %v\n", err)
		return 1
	}
}

func (r Runner) dispatch(args []string) error {
	switch args[0] {
	case "customer":
		return r.customer(args[1:])
	case "invoice":
		return r.invoice(args[1:])
	case "serve":
		return r.serve(args[1:])
	case "help", "-h", "-help", "--help":
		fmt.Fprint(r.Out, Usage)
		return nil
	}
	return usageError{fmt.Errorf("unknown command %q, see \"gitvoice help\"", args[0])}
}

func (r Runner) customer(args []string) error {
	if len(args) == 0 {
		return usageError{errors.New("usage: gitvoice customer <add|list> [flags]")}
	}
	switch args[0] {
	case "add":
		return r.customerAdd(args[1:])
	case "list":
		return r.customerList(args[1:])
	}
	return usageError{fmt.Errorf("unknown customer command %q, want add or list", args[0])}
}

func (r Runner) invoice(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return usageError{errors.New("usage: gitvoice invoice add [flags]")}
	}
	return r.invoiceAdd(args[1:])
}

func (r Runner) serve(args []string) error {
	if err := parse(r.flagSet("serve"), args); err != nil {
		return err
	}
	if r.Serve == nil {
		return errors.New("serving the web UI is not available here")
	}
	return r.Serve()
}

// customerAdd writes a new entry to the customer master data. Invoices copy
// these fields at creation time. There is no -id flag: the service derives
// the ID from the name and the command reports the one it stored.
func (r Runner) customerAdd(args []string) error {
	fs := r.flagSet("customer add")
	var c customer.Customer
	fs.StringVar(&c.Company, "company", "", "company name")
	fs.StringVar(&c.FirstName, "first-name", "", "given name")
	fs.StringVar(&c.LastName, "last-name", "", "family name")
	fs.StringVar(&c.Address, "address", "", `postal address, \n starts a new line`)
	fs.StringVar(&c.Email, "email", "", "email address")
	fs.StringVar(&c.Phone, "phone", "", "phone number, master data only")
	fs.StringVar(&c.VATID, "vat-id", "", "USt-IdNr., relevant for B2B")
	fs.StringVar(&c.Notes, "notes", "", "internal note, never printed")
	if err := parse(fs, args); err != nil {
		return err
	}
	c.Address = unescapeNewlines(c.Address)
	c.Notes = unescapeNewlines(c.Notes)
	stored, err := r.Svc.CreateCustomer(c)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.Out, "customer %s created\n", stored.ID)
	return nil
}

// customerList prints the master data as a table, sorted by name. It is how
// you look up the IDs that "invoice add -customer" expects.
func (r Runner) customerList(args []string) error {
	if err := parse(r.flagSet("customer list"), args); err != nil {
		return err
	}
	customers, err := r.Svc.Customers()
	if err != nil {
		return err
	}
	if len(customers) == 0 {
		fmt.Fprintln(r.Out, "no customers yet")
		return nil
	}
	w := tabwriter.NewWriter(r.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tCOMPANY\tNAME\tEMAIL")
	for _, c := range customers {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", c.ID, c.Company, c.PersonName(), c.Email)
	}
	return w.Flush()
}

// invoiceAdd writes a new invoice. Tax defaults come from the company
// profile via NewDraft, so the CLI and the web UI snapshot the same values.
// -customer prefills the recipient from the master data; the -customer-*
// flags fill in or override individual fields, as in the web form.
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
	return nil
}

const dateLayout = "2006-01-02"

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

// unescapeNewlines turns the literal two-character sequence \n into a real
// newline, so multi-line addresses and notes fit in one shell argument.
func unescapeNewlines(s string) string {
	return strings.ReplaceAll(s, `\n`, "\n")
}

func (r Runner) flagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet("gitvoice "+name, flag.ContinueOnError)
	fs.SetOutput(r.Err)
	return fs
}

// parse tags flag failures as reported: the flag set has already written the
// message and its usage to the output.
func parse(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return errors.Join(errReported, err)
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(fs.Output(), "unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return errReported
	}
	return nil
}
