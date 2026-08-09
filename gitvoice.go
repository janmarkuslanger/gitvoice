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
	"log"
	"net/http"

	"github.com/janmarkuslanger/gitvoice/internal/invoicing"
	"github.com/janmarkuslanger/gitvoice/internal/store"
	"github.com/janmarkuslanger/gitvoice/internal/web"
)

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
	handler http.Handler
}

// New prepares the data directory and the web UI.
func New(cfg Config) (*App, error) {
	cfg = cfg.withDefaults()
	st, err := store.New(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	srv, err := web.New(invoicing.New(st))
	if err != nil {
		return nil, err
	}
	return &App{cfg: cfg, handler: srv}, nil
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
