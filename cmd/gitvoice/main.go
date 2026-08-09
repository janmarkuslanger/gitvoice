// Command gitvoice is the standalone command line. Run it inside your
// invoice repository to add customers and invoices, or to serve the web UI:
//
//	go run github.com/janmarkuslanger/gitvoice/cmd/gitvoice customer add -id acme -company "ACME GmbH"
//	go run github.com/janmarkuslanger/gitvoice/cmd/gitvoice invoice add -number 2026-001 -customer acme -item "Consulting;3;120.00"
//	go run github.com/janmarkuslanger/gitvoice/cmd/gitvoice serve
//
// The global flags below come before the command name.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/janmarkuslanger/gitvoice"
)

func main() {
	var cfg gitvoice.Config
	fs := flag.NewFlagSet("gitvoice", flag.ExitOnError)
	fs.StringVar(&cfg.DataDir, "data", "data", "directory holding the JSON data")
	fs.StringVar(&cfg.Addr, "addr", "127.0.0.1:8080", "listen address of the web UI")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, gitvoice.CLIUsage)
		fmt.Fprintln(os.Stderr, "\nGlobal flags:")
		fs.PrintDefaults()
	}
	// ExitOnError: Parse never returns on failure.
	_ = fs.Parse(os.Args[1:])
	os.Exit(gitvoice.RunCLI(cfg, fs.Args(), os.Stdout, os.Stderr))
}
