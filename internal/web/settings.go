package web

import (
	"net/http"
	"strings"

	"github.com/janmarkuslanger/gitvoice/internal/company"
	"github.com/janmarkuslanger/gitvoice/internal/i18n"
)

func (s *Server) handleSettingsForm(w http.ResponseWriter, r *http.Request) {
	comp, err := s.svc.Company()
	if err != nil {
		s.serverError(w, err)
		return
	}
	s.render(w, r, "settings.html", map[string]any{
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
		Company:           strings.TrimSpace(r.PostFormValue("company")),
		FirstName:         strings.TrimSpace(r.PostFormValue("first_name")),
		LastName:          strings.TrimSpace(r.PostFormValue("last_name")),
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
	// Unsupported codes are dropped: the profile then falls back to English.
	if lang, ok := i18n.Parse(r.PostFormValue("language")); ok {
		c.Language = lang
	}
	if err := s.svc.SaveCompany(c); err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}
