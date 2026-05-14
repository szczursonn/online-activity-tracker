package viewer

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storesteam"
)

func (s *Server) handleSteamUser(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseUint(r.PathValue("userID"), 10, 64)
	if err != nil {
		respondBadRequest(w)
		return
	}

	var pageData *storesteam.GetUserPageDataResult

	if err := s.db.ReadTx(r.Context(), func(tx *db.Tx) (err error) {
		pageData, err = storesteam.GetUserPageData(tx, userID)
		return
	}); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			respondNotFound(w)
			return
		}

		if r.Context().Err() == nil {
			s.logger.Error("Failed to get data for steam user template", slog.Any("err", err))
		}
		respondInternalServerError(w)
		return
	}

	if err := s.steamUserTemplate.Execute(w, &pageData); err != nil {
		s.logger.Error("Failed to serve steam user template", slog.Any("err", err))
	}
}
