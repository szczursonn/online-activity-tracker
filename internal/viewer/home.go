package viewer

import (
	"net/http"
)

var homeTemplate = mustParseTemplate("home.html")

func registerHomePage(s *Server, mux *http.ServeMux) {
	mux.Handle("GET /{$}", newPageHandler(s, homeTemplate, nil))
}
