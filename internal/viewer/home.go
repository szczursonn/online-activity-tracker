package viewer

import (
	"log/slog"
	"net/http"
)

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		respondNotFound(w)
		return
	}

	if err := s.homeTemplate.Execute(w, nil); err != nil {
		s.logger.Error("Failed to serve home template", slog.Any("err", err))
	}
}
