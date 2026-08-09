package web

import (
	"net/http"
	"strings"

	"github.com/janmarkuslanger/gitvoice/internal/i18n"
)

const langCookie = "lang"

// lang resolves the UI language for a request: the switcher cookie wins,
// then the profile default, then the application fallback.
func (s *Server) lang(r *http.Request) string {
	if c, err := r.Cookie(langCookie); err == nil {
		if l, ok := i18n.Parse(c.Value); ok {
			return l
		}
	}
	if comp, err := s.svc.Company(); err == nil {
		if l, ok := i18n.Parse(comp.Language); ok {
			return l
		}
	}
	return i18n.Fallback
}

// handleLangSwitch stores the chosen language in a cookie and returns to
// the page the switcher was clicked on.
func (s *Server) handleLangSwitch(w http.ResponseWriter, r *http.Request) {
	lang, ok := i18n.Parse(r.PathValue("code"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     langCookie,
		Value:    lang,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		SameSite: http.SameSiteLaxMode,
	})
	// Only same-site paths: anything else could be an open redirect.
	back := r.URL.Query().Get("back")
	if !strings.HasPrefix(back, "/") || strings.HasPrefix(back, "//") {
		back = "/"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}
