package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/janmarkuslanger/gitvoice/internal/customer"
	"github.com/janmarkuslanger/gitvoice/internal/store"
)

type customerFormData struct {
	Customer customer.Customer
	IsNew    bool
	Action   string
	Error    string
}

func (s *Server) handleCustomers(w http.ResponseWriter, r *http.Request) {
	customers, err := s.store.ListCustomers()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, "customers.html", map[string]any{"Customers": customers})
}

func (s *Server) handleCustomerNewForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, "customer_form.html", customerFormData{
		IsNew:  true,
		Action: "/customers",
	})
}

func (s *Server) handleCustomerEditForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := s.store.GetCustomer(id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, "customer_form.html", customerFormData{
		Customer: c,
		Action:   "/customers/" + url.PathEscape(id),
	})
}

func (s *Server) handleCustomerCreate(w http.ResponseWriter, r *http.Request) {
	c, err := parseCustomerForm(r)
	if err == nil {
		var exists bool
		exists, err = s.store.CustomerExists(c.ID)
		if err == nil && exists {
			err = fmt.Errorf("customer %s already exists", c.ID)
		}
	}
	if err == nil {
		err = s.store.SaveCustomer(c)
	}
	if err != nil {
		s.render(w, "customer_form.html", customerFormData{
			Customer: c, IsNew: true, Action: "/customers", Error: err.Error(),
		})
		return
	}
	http.Redirect(w, r, "/customers", http.StatusSeeOther)
}

func (s *Server) handleCustomerUpdate(w http.ResponseWriter, r *http.Request) {
	oldID := r.PathValue("id")
	if ok, err := s.store.CustomerExists(oldID); err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	c, err := parseCustomerForm(r)
	if err == nil {
		err = s.store.SaveCustomer(c)
	}
	if err != nil {
		s.render(w, "customer_form.html", customerFormData{
			Customer: c, Action: "/customers/" + url.PathEscape(oldID), Error: err.Error(),
		})
		return
	}
	// The ID doubles as the filename: renaming means save new, drop old.
	if c.ID != oldID {
		if err := s.store.DeleteCustomer(oldID); err != nil && !errors.Is(err, store.ErrNotFound) {
			s.serverError(w, err)
			return
		}
	}
	http.Redirect(w, r, "/customers", http.StatusSeeOther)
}

func (s *Server) handleCustomerDelete(w http.ResponseWriter, r *http.Request) {
	err := s.store.DeleteCustomer(r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/customers", http.StatusSeeOther)
}

func parseCustomerForm(r *http.Request) (customer.Customer, error) {
	if err := r.ParseForm(); err != nil {
		return customer.Customer{}, fmt.Errorf("parse form: %w", err)
	}
	c := customer.Customer{
		ID:        strings.TrimSpace(r.PostFormValue("id")),
		Company:   strings.TrimSpace(r.PostFormValue("company")),
		FirstName: strings.TrimSpace(r.PostFormValue("first_name")),
		LastName:  strings.TrimSpace(r.PostFormValue("last_name")),
		Address:   strings.TrimSpace(r.PostFormValue("address")),
		Email:     strings.TrimSpace(r.PostFormValue("email")),
		Phone:     strings.TrimSpace(r.PostFormValue("phone")),
		VATID:     strings.TrimSpace(r.PostFormValue("vat_id")),
		Notes:     strings.TrimSpace(r.PostFormValue("notes")),
	}
	return c, c.Validate()
}
