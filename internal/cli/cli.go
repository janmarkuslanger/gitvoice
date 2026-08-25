// Package cli implements the gitvoice command line over the invoicing
// service: the same use cases the web UI drives, for scripts, terminals and
// agents. Every command maps onto invoicing.Service calls, so the rules stay
// in one place and both frontends behave identically.
//
// Commands that read data take a -json flag, so a caller that is not a human
// does not have to scrape the table output.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/janmarkuslanger/gitvoice/internal/invoicing"
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
  invoice list    list the invoices with their totals
  invoice show    print one invoice
  invoice pdf     render an invoice to PDF
  company show    print the issuer profile
  company set     fill in the issuer profile
  serve           serve the web UI
  help            show this text

The reading commands take -json for machine-readable output.
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
	case "company":
		return r.company(args[1:])
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
	if len(args) == 0 {
		return usageError{errors.New("usage: gitvoice invoice <add|list|show|pdf> [flags]")}
	}
	switch args[0] {
	case "add":
		return r.invoiceAdd(args[1:])
	case "list":
		return r.invoiceList(args[1:])
	case "show":
		return r.invoiceShow(args[1:])
	case "pdf":
		return r.invoicePDF(args[1:])
	}
	return usageError{fmt.Errorf("unknown invoice command %q, want add, list, show or pdf", args[0])}
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

const dateLayout = "2006-01-02"

// unescapeNewlines turns the literal two-character sequence \n into a real
// newline, so multi-line addresses and notes fit in one shell argument.
func unescapeNewlines(s string) string {
	return strings.ReplaceAll(s, `\n`, "\n")
}

// printJSON writes v as indented JSON, the machine-readable form of the
// reading commands.
func (r Runner) printJSON(v any) error {
	enc := json.NewEncoder(r.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (r Runner) flagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet("gitvoice "+name, flag.ContinueOnError)
	fs.SetOutput(r.Err)
	return fs
}

// parse handles a command without positional arguments.
func parse(fs *flag.FlagSet, args []string) error {
	_, err := parseArgs(fs, args, 0, "")
	return err
}

// parseArgs parses args, requires exactly want positional arguments, and
// returns them. Flags must precede the arguments, as the flag package stops
// at the first non-flag word. Failures are tagged as reported: the flag set
// has already written the message and its usage to the output.
func parseArgs(fs *flag.FlagSet, args []string, want int, names string) ([]string, error) {
	if err := fs.Parse(args); err != nil {
		return nil, errors.Join(errReported, err)
	}
	if fs.NArg() != want {
		if want == 0 {
			fmt.Fprintf(fs.Output(), "unexpected argument %q\n", fs.Arg(0))
		} else {
			fmt.Fprintf(fs.Output(), "usage: %s [flags] %s\n", fs.Name(), names)
		}
		fs.Usage()
		return nil, errReported
	}
	return fs.Args(), nil
}
