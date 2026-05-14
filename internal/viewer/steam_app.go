package viewer

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storesteam"
)

func (s *Server) handleSteamApp(w http.ResponseWriter, r *http.Request) {
	appID, err := strconv.ParseUint(r.PathValue("appID"), 10, 64)
	if err != nil {
		respondBadRequest(w)
		return
	}

	var pageData *storesteam.GetAppPageDataResult

	if err := s.db.ReadTx(r.Context(), func(tx *db.Tx) (err error) {
		pageData, err = storesteam.GetAppPageData(tx, appID)
		return
	}); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			respondNotFound(w)
			return
		}

		if r.Context().Err() == nil {
			s.logger.Error("Failed to get data for steam app template", slog.Any("err", err))
		}
		respondInternalServerError(w)
		return
	}

	if err := s.steamAppTemplate.Execute(w, &pageData); err != nil {
		s.logger.Error("Failed to serve steam app template", slog.Any("err", err))
	}
}
