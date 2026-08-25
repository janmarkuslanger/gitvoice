// Package gitvoice is an embeddable invoice manager: import it, point it at
// a directory inside your git repository, and it serves a small web UI that
// reads and writes invoices as JSON files. Commit the files with git as usual.
//
// Minimal usage:
//
//	package main
//
//	import "github.com/janmarkuslanger/gitvoice"
//
//	func main() {
//		if err := gitvoice.Run(gitvoice.Config{}); err != nil {
//			log.Fatal(err)
//		}
//	}
package gitvoice

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/janmarkuslanger/gitvoice/internal/cli"
	"github.com/janmarkuslanger/gitvoice/internal/invoicing"
	"github.com/janmarkuslanger/gitvoice/internal/store"
	"github.com/janmarkuslanger/gitvoice/internal/web"
)

// CLIUsage is the help text of the command line, for wrappers that want to
// print it alongside their own flags. See cmd/gitvoice.
const CLIUsage = cli.Usage

// Config controls where data lives and where the UI is served.
// The zero value is usable: data in ./data, UI on :8080.
type Config struct {
	// DataDir is the directory holding the JSON data, typically a path
	// inside your git repository. Defaults to "data".
	DataDir string
	// Addr is the listen address for the web UI. Defaults to "127.0.0.1:8080".
	Addr string
}

func (c Config) withDefaults() Config {
	if c.DataDir == "" {
		c.DataDir = "data"
	}
	if c.Addr == "" {
		c.Addr = "127.0.0.1:8080"
	}
	return c
}

// App is a configured gitvoice instance.
type App struct {
	cfg     Config
	svc     *invoicing.Service
	handler http.Handler
}

// New prepares the data directory and the web UI.
func New(cfg Config) (*App, error) {
	cfg = cfg.withDefaults()
	st, err := store.New(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	svc := invoicing.New(st)
	srv, err := web.New(svc)
	if err != nil {
		return nil, err
	}
	return &App{cfg: cfg, svc: svc, handler: srv}, nil
}

// Handler returns the UI as an http.Handler, for mounting into an existing
// server or wrapping with middleware (e.g. auth).
func (a *App) Handler() http.Handler {
	return a.handler
}

// Run serves the UI on the configured address and blocks.
func (a *App) Run() error {
	log.Printf("gitvoice: serving on http://%s (data: %s)", a.cfg.Addr, a.cfg.DataDir)
	return http.ListenAndServe(a.cfg.Addr, a.handler)
}

// Run is the one-call entry point: New + App.Run.
func Run(cfg Config) error {
	app, err := New(cfg)
	if err != nil {
		return err
	}
	return app.Run()
}

// CLI runs one command line ("company set …", "customer add …",
// "invoice add …", "invoice pdf …", "serve") against the app's data
// directory and returns the process exit code: 0 on success, 1 when the
// command failed, 2 when it was invoked wrongly. See CLIUsage for the full
// command list.
func (a *App) CLI(args []string, out, errOut io.Writer) int {
	return cli.Runner{Svc: a.svc, Serve: a.Run, Out: out, Err: errOut}.Run(args)
}

// RunCLI is the one-call CLI entry point: New + App.CLI. Pass it the
// arguments after your own global flags and exit with the code it returns.
func RunCLI(cfg Config, args []string, out, errOut io.Writer) int {
	app, err := New(cfg)
	if err != nil {
		fmt.Fprintf(errOut, "gitvoice: %v\n", err)
		return 1
	}
	return app.CLI(args, out, errOut)
}
