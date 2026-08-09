package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/janmarkuslanger/gitvoice/internal/invoice"
	"github.com/janmarkuslanger/gitvoice/internal/invoicing"
	"github.com/janmarkuslanger/gitvoice/internal/money"
)

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	invoices, err := s.svc.Invoices()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "list.html", map[string]any{"Invoices": invoices})
}

func (s *Server) handleView(w http.ResponseWriter, r *http.Request) {
	inv, err := s.svc.Invoice(r.PathValue("number"))
	if errors.Is(err, invoicing.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	comp, err := s.svc.Company()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "view.html", map[string]any{
		"Invoice":  inv,
		"Company":  comp,
		"Warnings": invoicing.ComplianceWarnings(inv, comp),
	})
}

// invoiceForm assembles the render data for the invoice form, loading the
// customer master data for the customer select.
func (s *Server) invoiceForm(inv invoice.Invoice, isNew bool, action, errMsg string) (map[string]any, error) {
	customers, err := s.svc.Customers()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"Invoice":   inv,
		"Statuses":  invoice.Statuses,
		"Customers": customers,
		"IsNew":     isNew,
		"Action":    action,
		"Error":     errMsg,
	}, nil
}

func (s *Server) renderInvoiceForm(w http.ResponseWriter, r *http.Request, inv invoice.Invoice, isNew bool, action, errMsg string) {
	data, err := s.invoiceForm(inv, isNew, action, errMsg)
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "form.html", data)
}

func (s *Server) handleNewForm(w http.ResponseWriter, r *http.Request) {
	inv, err := s.svc.NewDraft()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.renderInvoiceForm(w, r, inv, true, "/invoices", "")
}

func (s *Server) handleEditForm(w http.ResponseWriter, r *http.Request) {
	number := r.PathValue("number")
	inv, err := s.svc.Invoice(number)
	if errors.Is(err, invoicing.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.renderInvoiceForm(w, r, inv, false, "/invoices/"+url.PathEscape(number), "")
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	inv, err := parseInvoiceForm(r)
	if err == nil {
		err = s.svc.CreateInvoice(inv)
	}
	if err != nil {
		s.renderInvoiceForm(w, r, inv, true, "/invoices", err.Error())
		return
	}
	http.Redirect(w, r, "/invoices/"+url.PathEscape(inv.Number), http.StatusSeeOther)
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	oldNumber := r.PathValue("number")
	inv, err := parseInvoiceForm(r)
	if err == nil {
		err = s.svc.UpdateInvoice(oldNumber, inv)
	}
	if errors.Is(err, invoicing.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.renderInvoiceForm(w, r, inv, false, "/invoices/"+url.PathEscape(oldNumber), err.Error())
		return
	}
	http.Redirect(w, r, "/invoices/"+url.PathEscape(inv.Number), http.StatusSeeOther)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	err := s.svc.DeleteInvoice(r.PathValue("number"))
	if errors.Is(err, invoicing.ErrNotFound) {
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
		Number:             strings.TrimSpace(r.PostFormValue("number")),
		Date:               r.PostFormValue("date"),
		DueDate:            r.PostFormValue("due_date"),
		Status:             invoice.Status(r.PostFormValue("status")),
		Currency:           strings.TrimSpace(r.PostFormValue("currency")),
		ServiceDate:        r.PostFormValue("service_date"),
		ServicePeriodStart: r.PostFormValue("service_period_start"),
		ServicePeriodEnd:   r.PostFormValue("service_period_end"),
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
		rate, err := money.ParseDecimal(raw)
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
			q, err := money.ParseDecimal(quantities[i])
			if err != nil {
				errs = append(errs, fmt.Errorf("item %d: invalid quantity %q", i+1, quantities[i]))
			}
			item.Quantity = q
		}
		if i < len(prices) {
			cents, err := money.ParseCents(prices[i])
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
