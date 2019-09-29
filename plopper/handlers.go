package plopper

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/husio/plopper/lith"
)

func NewHTTPApplication(plops PlopStore, auth *lith.Client, authUI string) http.Handler {
	withAuth := lith.AuthMiddleware(auth)

	mux := http.NewServeMux()
	mux.Handle("/", withAuth(&listPlopsHandler{plops: plops}))

	http.Handle("/accounts/", http.StripPrefix("/accounts/", revproxy(authUI)))
	// Static files require "/pub/" statics.
	http.Handle("/pub/", http.StripPrefix("/pub/", revproxy(authUI)))

	mux.Handle("/create", withAuth(&requireLoginMiddleware{
		loginURL: "/accounts/login/",
		next:     &createPlopHandler{plops: plops},
	}))
	mux.Handle("/plop/", http.StripPrefix("/plop/", &showPlopHandler{plops: plops}))
	return mux
}

const (
	plopsPerPage          = 50
	plopPaginationDateFmt = "2006-01-02_15-04-05"
)

type requireLoginMiddleware struct {
	next     http.Handler
	loginURL string
}

func (m requireLoginMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	account, ok := lith.CurrentAccount(r.Context())
	if !ok {
		dest := m.loginURL + "?next=" + url.QueryEscape(r.URL.Path)
		http.Redirect(w, r, dest, http.StatusSeeOther)
		return
	}

	if !contains(account.Permissions, "plop:create") {
		renderFail(w, http.StatusForbidden, `"plop:create" permission is required.`)
		return
	}

	m.next.ServeHTTP(w, r)
}

func contains(collection []string, element string) bool {
	for _, s := range collection {
		if s == element {
			return true
		}
	}
	return false
}

type showPlopHandler struct {
	plops PlopStore
}

func (h *showPlopHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, err := hex.DecodeString(r.URL.Path)
	if err != nil {
		renderStd(w, http.StatusNotFound)
		return
	}

	switch plop, err := h.plops.Plop(r.Context(), id); {
	case err == nil:
		render(w, "show-plop", plop)
	case errors.Is(err, ErrNotFound):
		renderStd(w, http.StatusNotFound)
	default:
		log.Printf("cannot get plop %q: %s", id, err)
		renderStd(w, http.StatusInternalServerError)
	}
}

type listPlopsHandler struct {
	plops PlopStore
}

func (h *listPlopsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	olderThan := time.Now().UTC()
	isNewest := true
	query := r.URL.Query()
	if raw := query.Get("olderThan"); raw != "" {
		if t, err := time.Parse(plopPaginationDateFmt, raw); err == nil {
			olderThan = t
			isNewest = false
		}
	}

	plops, err := h.plops.ListPlops(r.Context(), olderThan, plopsPerPage)
	if err != nil {
		log.Printf("cannot list plops: %s", err)
		renderStd(w, http.StatusInternalServerError)
		return
	}

	var nextPage string
	if len(plops) == plopsPerPage {
		nextPage = plops[plopsPerPage-1].CreatedAt.Format(plopPaginationDateFmt)
	}

	account, _ := lith.CurrentAccount(r.Context())

	render(w, "list-plops", struct {
		Plops    []*Plop
		Account  *lith.AccountSession
		IsNewest bool
		NextPage string
	}{
		Plops:    plops,
		Account:  account,
		IsNewest: isNewest,
		NextPage: nextPage,
	})
}

type createPlopHandler struct {
	plops PlopStore
}

func (h createPlopHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	account, ok := lith.CurrentAccount(r.Context())
	if !ok {
		renderFail(w, http.StatusUnauthorized, "Not logged in.")
		return
	}

	if r.Method != "POST" {
		renderStd(w, http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		renderFail(w, http.StatusBadRequest, fmt.Sprintf("cannot parse form: %s", err))
		return
	}

	content := r.Form.Get("content")
	switch n := len(strings.TrimSpace(content)); {
	case n < 3:
		renderFail(w, http.StatusBadRequest, "Content must be more than 3 characters")
		return
	case n > 1024:
		renderFail(w, http.StatusBadRequest, "Content must be more less than 1024 characters")
		return
	}

	if _, err := h.plops.Create(r.Context(), account.AccountID, content); err != nil {
		log.Printf("cannot create a plop: %s", err)
		renderStd(w, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func render(w http.ResponseWriter, templateName string, context interface{}) {
	var b bytes.Buffer

	if err := tmpl.ExecuteTemplate(&b, templateName, context); err != nil {
		const code = http.StatusInternalServerError
		http.Error(w, http.StatusText(code), code)
		log.Printf("cannot render %q template: %v", templateName, err)
		return
	}
	_, _ = b.WriteTo(w)
}

func renderStd(w http.ResponseWriter, code int) {
	render(w, "std", http.StatusText(code))
}

func renderFail(w http.ResponseWriter, code int, description string) {
	var b bytes.Buffer

	context := struct {
		Description string
		Code        int
	}{
		Description: description,
		Code:        code,
	}
	if err := tmpl.ExecuteTemplate(&b, "fail", context); err != nil {
		log.Printf("cannot render fail template: %v", err)
		renderStd(w, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(code)
	_, _ = b.WriteTo(w)
}

var (
	//go:embed template.html
	tmplRaw string
	tmpl    = template.Must(template.New("").Parse(tmplRaw))
)
