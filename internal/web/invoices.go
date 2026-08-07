package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/janmarkuslanger/gitvoice/internal/customer"
	"github.com/janmarkuslanger/gitvoice/internal/invoice"
	"github.com/janmarkuslanger/gitvoice/internal/store"
)

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	invoices, err := s.store.List()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, "list.html", map[string]any{"Invoices": invoices})
}

func (s *Server) handleView(w http.ResponseWriter, r *http.Request) {
	inv, err := s.store.Get(r.PathValue("number"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	comp, err := s.store.Company()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, "view.html", map[string]any{"Invoice": inv, "Company": comp})
}

type formData struct {
	Invoice   invoice.Invoice
	Statuses  []invoice.Status
	Customers []customer.Customer
	IsNew     bool
	Action    string
	Error     string
}

// invoiceForm assembles the render data for the invoice form, loading the
// customer master data for the customer select.
func (s *Server) invoiceForm(inv invoice.Invoice, isNew bool, action, errMsg string) (formData, error) {
	customers, err := s.store.ListCustomers()
	if err != nil {
		return formData{}, err
	}
	return formData{
		Invoice:   inv,
		Statuses:  invoice.Statuses,
		Customers: customers,
		IsNew:     isNew,
		Action:    action,
		Error:     errMsg,
	}, nil
}

func (s *Server) renderInvoiceForm(w http.ResponseWriter, inv invoice.Invoice, isNew bool, action, errMsg string) {
	data, err := s.invoiceForm(inv, isNew, action, errMsg)
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, "form.html", data)
}

func (s *Server) handleNewForm(w http.ResponseWriter, r *http.Request) {
	comp, err := s.store.Company()
	if err != nil {
		s.serverError(w, err)
		return
	}
	inv := invoice.Invoice{Status: invoice.StatusDraft, Currency: "EUR"}
	// Snapshot the tax defaults from the profile; the form can override them.
	inv.SmallBusiness = comp.SmallBusiness
	if !comp.SmallBusiness {
		inv.TaxRatePercent = 19
	}
	s.renderInvoiceForm(w, inv, true, "/invoices", "")
}

func (s *Server) handleEditForm(w http.ResponseWriter, r *http.Request) {
	number := r.PathValue("number")
	inv, err := s.store.Get(number)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.renderInvoiceForm(w, inv, false, "/invoices/"+url.PathEscape(number), "")
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	inv, err := parseInvoiceForm(r)
	if err == nil {
		var exists bool
		exists, err = s.store.Exists(inv.Number)
		if err == nil && exists {
			err = fmt.Errorf("invoice %s already exists", inv.Number)
		}
	}
	if err == nil {
		err = s.store.Save(inv)
	}
	if err != nil {
		s.renderInvoiceForm(w, inv, true, "/invoices", err.Error())
		return
	}
	http.Redirect(w, r, "/invoices/"+url.PathEscape(inv.Number), http.StatusSeeOther)
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	oldNumber := r.PathValue("number")
	if ok, err := s.store.Exists(oldNumber); err != nil || !ok {
		http.NotFound(w, r)
		return
	}
	inv, err := parseInvoiceForm(r)
	if err == nil {
		err = s.store.Save(inv)
	}
	if err != nil {
		s.renderInvoiceForm(w, inv, false, "/invoices/"+url.PathEscape(oldNumber), err.Error())
		return
	}
	// The number doubles as the filename: renaming means save new, drop old.
	if inv.Number != oldNumber {
		if err := s.store.Delete(oldNumber); err != nil && !errors.Is(err, store.ErrNotFound) {
			s.serverError(w, err)
			return
		}
	}
	http.Redirect(w, r, "/invoices/"+url.PathEscape(inv.Number), http.StatusSeeOther)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	err := s.store.Delete(r.PathValue("number"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func parseInvoiceForm(r *http.Request) (invoice.Invoice, error) {
	if err := r.ParseForm(); err != nil {
		return invoice.Invoice{}, fmt.Errorf("parse form: %w", err)
	}
	inv := invoice.Invoice{
		Number:   strings.TrimSpace(r.PostFormValue("number")),
		Date:     r.PostFormValue("date"),
		DueDate:  r.PostFormValue("due_date"),
		Status:   invoice.Status(r.PostFormValue("status")),
		Currency: strings.TrimSpace(r.PostFormValue("currency")),
		Customer: invoice.Customer{
			Company:   strings.TrimSpace(r.PostFormValue("customer_company")),
			FirstName: strings.TrimSpace(r.PostFormValue("customer_first_name")),
			LastName:  strings.TrimSpace(r.PostFormValue("customer_last_name")),
			Address:   strings.TrimSpace(r.PostFormValue("customer_address")),
			Email:     strings.TrimSpace(r.PostFormValue("customer_email")),
			VATID:     strings.TrimSpace(r.PostFormValue("customer_vat_id")),
		},
		SmallBusiness: r.PostFormValue("small_business") != "",
		Notes:         strings.TrimSpace(r.PostFormValue("notes")),
	}
	descriptions := r.PostForm["item_description"]
	quantities := r.PostForm["item_quantity"]
	prices := r.PostForm["item_price"]
	var errs []error
	if raw := strings.TrimSpace(r.PostFormValue("tax_rate")); raw != "" && !inv.SmallBusiness {
		rate, err := strconv.ParseFloat(normalizeDecimal(raw), 64)
		if err != nil {
			errs = append(errs, fmt.Errorf("invalid tax rate %q", raw))
		}
		inv.TaxRatePercent = rate
	}
	for i, desc := range descriptions {
		desc = strings.TrimSpace(desc)
		if desc == "" {
			continue
		}
		item := invoice.Item{Description: desc}
		if i < len(quantities) {
			q, err := strconv.ParseFloat(normalizeDecimal(quantities[i]), 64)
			if err != nil {
				errs = append(errs, fmt.Errorf("item %d: invalid quantity %q", i+1, quantities[i]))
			}
			item.Quantity = q
		}
		if i < len(prices) {
			cents, err := ParseCents(prices[i])
			if err != nil {
				errs = append(errs, fmt.Errorf("item %d: invalid price %q", i+1, prices[i]))
			}
			item.UnitPriceCents = cents
		}
		inv.Items = append(inv.Items, item)
	}
	if err := errors.Join(errs...); err != nil {
		return inv, err
	}
	return inv, inv.Validate()
}
