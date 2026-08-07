package web

import (
	"net/http"
	"strings"

	"github.com/janmarkuslanger/gitvoice/internal/company"
)

func (s *Server) handleSettingsForm(w http.ResponseWriter, r *http.Request) {
	comp, err := s.store.Company()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, "settings.html", map[string]any{
		"Company": comp,
		"Saved":   r.URL.Query().Get("saved") != "",
	})
}

func (s *Server) handleSettingsSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "parse form: "+err.Error(), http.StatusBadRequest)
		return
	}
	c := company.Company{
		Name:              strings.TrimSpace(r.PostFormValue("name")),
		Address:           strings.TrimSpace(r.PostFormValue("address")),
		Email:             strings.TrimSpace(r.PostFormValue("email")),
		Phone:             strings.TrimSpace(r.PostFormValue("phone")),
		TaxNumber:         strings.TrimSpace(r.PostFormValue("tax_number")),
		VATID:             strings.TrimSpace(r.PostFormValue("vat_id")),
		IBAN:              strings.TrimSpace(r.PostFormValue("iban")),
		BIC:               strings.TrimSpace(r.PostFormValue("bic")),
		BankName:          strings.TrimSpace(r.PostFormValue("bank_name")),
		SmallBusiness:     r.PostFormValue("small_business") != "",
		SmallBusinessNote: strings.TrimSpace(r.PostFormValue("small_business_note")),
	}
	if err := s.store.SaveCompany(c); err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}
