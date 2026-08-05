package viewer

import (
	"cmp"
	"embed"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/szczursonn/online-activity-tracker/internal/db"
)

//go:embed templates/*.html
var templatesFS embed.FS

const (
	discordLogoURL = "https://upload.wikimedia.org/wikipedia/fr/8/80/Logo_Discord_2015.png"
	steamLogoURL   = "https://upload.wikimedia.org/wikipedia/commons/c/c1/Steam_Logo.png"
)

type navLink struct {
	Href    string
	Label   string
	IconURL string
	IconAlt string
}

var navLinks = []navLink{
	discordUsersNavLink,
	discordGuildsNavLink,
	steamUsersNavLink,
	steamAppsNavLink,
}

var templateFuncs = template.FuncMap{
	"navLinks": func() []navLink {
		return navLinks
	},
}

const layoutTemplateFileName = "_layout.html"

func mustParseTemplate(fileNames ...string) *template.Template {
	patterns := make([]string, 0, len(fileNames)+1)
	patterns = append(patterns, "templates/"+layoutTemplateFileName)
	for _, fileName := range fileNames {
		patterns = append(patterns, "templates/"+fileName)
	}

	return template.Must(template.New(layoutTemplateFileName).Funcs(templateFuncs).ParseFS(templatesFS, patterns...))
}

func displayNameStrOrID(name string, id uint64) string {
	return cmp.Or(name, strconv.FormatUint(id, 10))
}

func newPageHandler(s *Server, tmpl *template.Template, getPageData func(tx *db.Tx) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.servePage(tmpl, w, r, getPageData)
	}
}

func newPageHandlerWithParams[P any](s *Server, tmpl *template.Template, extractParams func(r *http.Request) (P, error), getPageData func(tx *db.Tx, params P) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params, err := extractParams(r)
		if err != nil {
			respondBadRequest(w)
			return
		}

		s.servePage(tmpl, w, r, func(tx *db.Tx) (any, error) {
			return getPageData(tx, params)
		})
	}
}

func (s *Server) servePage(tmpl *template.Template, w http.ResponseWriter, r *http.Request, getPageData func(tx *db.Tx) (any, error)) {
	var pageData any

	if getPageData != nil {
		if err := s.db.ReadTx(r.Context(), func(tx *db.Tx) (err error) {
			pageData, err = getPageData(tx)
			return
		}); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				respondNotFound(w)
				return
			}

			if r.Context().Err() == nil {
				s.logger.Error("Failed to get page data", slog.String("path", r.URL.Path), slog.Any("err", err))
			}
			respondInternalServerError(w)
			return
		}
	}

	if err := tmpl.Execute(w, pageData); err != nil {
		s.logger.Error("Failed to serve page template", slog.String("path", r.URL.Path), slog.Any("err", err))
	}
}
