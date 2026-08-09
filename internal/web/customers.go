package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/janmarkuslanger/gitvoice/internal/customer"
	"github.com/janmarkuslanger/gitvoice/internal/invoicing"
)

// customerFormData assembles the render data for the customer form.
func customerFormData(c customer.Customer, isNew bool, action, errMsg string) map[string]any {
	return map[string]any{
		"Customer": c,
		"IsNew":    isNew,
		"Action":   action,
		"Error":    errMsg,
	}
}

func (s *Server) handleCustomers(w http.ResponseWriter, r *http.Request) {
	customers, err := s.svc.Customers()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "customers.html", map[string]any{"Customers": customers})
}

func (s *Server) handleCustomerNewForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, "customer_form.html", customerFormData(customer.Customer{}, true, "/customers", ""))
}

func (s *Server) handleCustomerEditForm(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := s.svc.Customer(id)
	if errors.Is(err, invoicing.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "customer_form.html", customerFormData(c, false, "/customers/"+url.PathEscape(id), ""))
}

func (s *Server) handleCustomerCreate(w http.ResponseWriter, r *http.Request) {
	c, err := parseCustomerForm(r)
	if err == nil {
		_, err = s.svc.CreateCustomer(c)
	}
	if err != nil {
		s.render(w, r, "customer_form.html", customerFormData(c, true, "/customers", err.Error()))
		return
	}
	http.Redirect(w, r, "/customers", http.StatusSeeOther)
}

func (s *Server) handleCustomerUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := parseCustomerForm(r)
	// The form carries no ID; fill it in so a re-render on error still shows
	// which customer is being edited.
	c.ID = id
	if err == nil {
		_, err = s.svc.UpdateCustomer(id, c)
	}
	if errors.Is(err, invoicing.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.render(w, r, "customer_form.html", customerFormData(c, false, "/customers/"+url.PathEscape(id), err.Error()))
		return
	}
	http.Redirect(w, r, "/customers", http.StatusSeeOther)
}

func (s *Server) handleCustomerDelete(w http.ResponseWriter, r *http.Request) {
	err := s.svc.DeleteCustomer(r.PathValue("id"))
	if errors.Is(err, invoicing.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/customers", http.StatusSeeOther)
}

// parseCustomerForm reads the posted fields. The ID is not among them: it is
// derived on create and fixed afterwards, so the form never carries one.
// Validation is left to the service, which knows the ID it assigned.
func parseCustomerForm(r *http.Request) (customer.Customer, error) {
	if err := r.ParseForm(); err != nil {
		return customer.Customer{}, fmt.Errorf("parse form: %w", err)
	}
	c := customer.Customer{
		Company:   strings.TrimSpace(r.PostFormValue("company")),
		FirstName: strings.TrimSpace(r.PostFormValue("first_name")),
		LastName:  strings.TrimSpace(r.PostFormValue("last_name")),
		Address:   strings.TrimSpace(r.PostFormValue("address")),
		Email:     strings.TrimSpace(r.PostFormValue("email")),
		Phone:     strings.TrimSpace(r.PostFormValue("phone")),
		VATID:     strings.TrimSpace(r.PostFormValue("vat_id")),
		Notes:     strings.TrimSpace(r.PostFormValue("notes")),
	}
	return c, nil
}
