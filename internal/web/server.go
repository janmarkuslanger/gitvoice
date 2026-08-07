// Package web serves the embedded UI over the store.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/janmarkuslanger/gitvoice/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

// Server holds the parsed templates and routes.
type Server struct {
	store *store.Store
	mux   *http.ServeMux
	pages map[string]*template.Template
}

var funcs = template.FuncMap{
	"money": FormatCents,
	"qty":   formatQuantity,
	"nl2br": func(s string) template.HTML {
		escaped := template.HTMLEscapeString(s)
		return template.HTML(strings.ReplaceAll(escaped, "\n", "<br>"))
	},
}

// New builds the server. Template parsing errors are programmer errors in
// the embedded UI, so they surface immediately.
func New(st *store.Store) (*Server, error) {
	pages := make(map[string]*template.Template)
	pageNames := []string{
		"list.html", "view.html", "form.html", "settings.html",
		"customers.html", "customer_form.html",
	}
	for _, name := range pageNames {
		t, err := template.New("layout").Funcs(funcs).ParseFS(templateFS, "templates/layout.html", "templates/"+name)
		if err != nil {
			return nil, fmt.Errorf("parse template %s: %w", name, err)
		}
		pages[name] = t
	}
	s := &Server{store: st, mux: http.NewServeMux(), pages: pages}

	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	s.mux.HandleFunc("GET /{$}", s.handleList)
	s.mux.HandleFunc("GET /invoices/new", s.handleNewForm)
	s.mux.HandleFunc("POST /invoices", s.handleCreate)
	s.mux.HandleFunc("GET /invoices/{number}", s.handleView)
	s.mux.HandleFunc("GET /invoices/{number}/edit", s.handleEditForm)
	s.mux.HandleFunc("POST /invoices/{number}", s.handleUpdate)
	s.mux.HandleFunc("POST /invoices/{number}/delete", s.handleDelete)
	s.mux.HandleFunc("GET /customers", s.handleCustomers)
	s.mux.HandleFunc("GET /customers/new", s.handleCustomerNewForm)
	s.mux.HandleFunc("POST /customers", s.handleCustomerCreate)
	s.mux.HandleFunc("GET /customers/{id}/edit", s.handleCustomerEditForm)
	s.mux.HandleFunc("POST /customers/{id}", s.handleCustomerUpdate)
	s.mux.HandleFunc("POST /customers/{id}/delete", s.handleCustomerDelete)
	s.mux.HandleFunc("GET /settings", s.handleSettingsForm)
	s.mux.HandleFunc("POST /settings", s.handleSettingsSave)
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) render(w http.ResponseWriter, page string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.pages[page].ExecuteTemplate(w, "layout", data); err != nil {
		log.Printf("gitvoice: render %s: %v", page, err)
	}
}

func (s *Server) serverError(w http.ResponseWriter, err error) {
	log.Printf("gitvoice: %v", err)
	http.Error(w, "internal error: "+err.Error(), http.StatusInternalServerError)
}
