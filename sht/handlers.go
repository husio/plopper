package sht

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"
)

func NewHTTPApplication(store PlopStore) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", listPlopsHandler(store))
	mux.Handle("/create", createPlopHandler(store))
	mux.Handle("/plop/", http.StripPrefix("/plop/", showPlopHandler(store)))
	return mux
}

const (
	plopsPerPage          = 50
	plopPaginationDateFmt = "2006-01-02_15-04-05"
)

func showPlopHandler(store PlopStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := hex.DecodeString(r.URL.Path)
		if err != nil {
			renderStd(w, http.StatusNotFound)
			return
		}

		switch plop, err := store.Plop(r.Context(), id); {
		case err == nil:
			render(w, "show-plop", plop)
		case errors.Is(err, ErrNotFound):
			renderStd(w, http.StatusNotFound)
		default:
			log.Printf("cannot get plop %q: %s", id, err)
			renderStd(w, http.StatusInternalServerError)
		}
	}
}

func listPlopsHandler(store PlopStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		olderThan := time.Now().UTC()
		query := r.URL.Query()
		if raw := query.Get("olderThan"); raw != "" {
			if t, err := time.Parse(plopPaginationDateFmt, raw); err == nil {
				olderThan = t
			}
		}

		plops, err := store.ListPlops(r.Context(), olderThan, plopsPerPage)
		if err != nil {
			log.Printf("cannot list plops: %s", err)
			renderStd(w, http.StatusInternalServerError)
			return
		}

		var nextPage string
		if len(plops) == plopsPerPage {
			nextPage = plops[plopsPerPage-1].CreatedAt.Format(plopPaginationDateFmt)
		}

		render(w, "list-plops", struct {
			Plops    []*Plop
			NextPage string
		}{
			Plops:    plops,
			NextPage: nextPage,
		})
	}
}

func createPlopHandler(store PlopStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		if _, err := store.Create(r.Context(), content); err != nil {
			log.Printf("cannot create a plop: %s", err)
			renderStd(w, http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
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

var tmpl = template.Must(template.New("").Parse(`
{{- define "header" -}}
<!doctype html>
<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/normalize/8.0.1/normalize.min.css" integrity="sha256-l85OmPOjvil/SOvVt3HnSSjzF1TUMyT9eV0c2BzEGzU=" crossorigin="anonymous" />
<style>{{template "main.css"}}</style>
{{end}}

{{- define "footer" -}}
{{- end}}


{{- define "fail" -}}
	{{- template "header" -}}
	<div>
		{{.Description}}
	</div>
	{{- template "footer" -}}
{{- end}}


{{define "std"}}
	{{- template "header" -}}
	{{.}}
	{{- template "footer" -}}
{{end}}


{{define "list-plops"}}
	{{- template "header"}}

	<h1>
		Welcome to Shitter!
		<small>
			<a href="https://www.youtube.com/watch?v=SbbNf0TEh8g" target="_blank">What is shitter?</a>
		</small>
	</h1>

	<form class="create-plop" action="/create" method="POST">
		<textarea name="content" placeholder="Write your plop here." required minlength="3" maxlength="1024" pattern=".{3,1024}"></textarea>
		<button>Publish</button>
	</form>

	{{range .Plops}}
		{{template "render-plop" .}}
	{{else}}
		No plops
	{{end}}

	<a href="/">Show newest plops</a>
	{{if .NextPage}}
		<a href="/?olderThan={{.NextPage}}">Show older plops</a>
	{{end}}


	{{- template "footer" -}}
{{end}}


{{define "show-plop"}}
	{{- template "header"}}
	{{- template "render-plop" . -}}
	<a href="/">Show newest plops</a>
	{{- template "footer" -}}
{{end}}

{{define "render-plop"}}
	<div class="plop" id="plop-{{.ID}}">
		<div class="created-at" title="{{.CreatedAt }}">
			<a href="/plop/{{.ID}}">{{.CreatedAt.Format "02 Jan 2006"}}</a>
		</div>
		<div class="content">{{.Content}}</div>
	</div>
{{end}}


{{define "main.css"}}
* 		{ box-sizing: border-box; }
body 		{ max-width: 600px; margin: 0 auto; }
a 		{ color: #2881D6; text-decoration: none; }
a:hover         { color: #D62847; }
a:visited       { color: #6B28D6; }
h1 small        { font-size: 40%; }

form.create-plop 		{ margin: 20px 0; }
form.create-plop textarea 	{ width: 100%; padding: 8px; min-height: 4em; }
form.create-plop button         { margin: 4px 0; }

.plop 			{ border: 1px solid #ddd; padding: 10px; margin: 10px 0; border-radius: 3px; position: relative; }
.plop .created-at 	{ font-size: 80%; position: absolute; top: 4px; right: 6px; }
.plop .content 		{ padding-top: 0.8em; white-space: pre-line; }
{{end}}
`))
