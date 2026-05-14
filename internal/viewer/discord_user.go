package viewer

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storediscord"
)

func (s *Server) handleDiscordUser(w http.ResponseWriter, r *http.Request) {
	userID, err := strconv.ParseUint(r.PathValue("userID"), 10, 64)
	if err != nil {
		respondBadRequest(w)
		return
	}

	var pageData *storediscord.GetUserPageDataResult

	if err := s.db.ReadTx(r.Context(), func(tx *db.Tx) (err error) {
		pageData, err = storediscord.GetUserPageData(tx, userID)
		return
	}); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			respondNotFound(w)
			return
		}

		if r.Context().Err() == nil {
			s.logger.Error("Failed to get data for discord user template", slog.Any("err", err))
		}
		respondInternalServerError(w)
		return
	}

	if err := s.discordUserTemplate.Execute(w, &pageData); err != nil {
		s.logger.Error("Failed to serve discord user template", slog.Any("err", err))
	}
}
