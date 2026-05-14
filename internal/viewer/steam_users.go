package viewer

import (
	"log/slog"
	"net/http"

	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storesteam"
)

func (s *Server) handleSteamUsers(w http.ResponseWriter, r *http.Request) {
	var pageData *storesteam.GetUsersPageDataResult

	if err := s.db.ReadTx(r.Context(), func(tx *db.Tx) (err error) {
		pageData, err = storesteam.GetUsersPageData(tx)
		return
	}); err != nil {
		if r.Context().Err() == nil {
			s.logger.Error("Failed to get data for steam users template", slog.Any("err", err))
		}
		respondInternalServerError(w)
		return
	}

	if err := s.steamUsersTemplate.Execute(w, pageData); err != nil {
		s.logger.Error("Failed to serve steam users template", slog.Any("err", err))
	}
}
