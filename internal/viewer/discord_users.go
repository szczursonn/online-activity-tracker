package viewer

import (
	"log/slog"
	"net/http"

	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storediscord"
)

func (s *Server) handleDiscordUsers(w http.ResponseWriter, r *http.Request) {
	var pageData *storediscord.GetUsersPageDataResult

	if err := s.db.ReadTx(r.Context(), func(tx *db.Tx) (err error) {
		pageData, err = storediscord.GetUsersPageData(tx)
		return
	}); err != nil {
		if r.Context().Err() == nil {
			s.logger.Error("Failed to get data for discord users template", slog.Any("err", err))
		}
		respondInternalServerError(w)
		return
	}

	if err := s.discordUsersTemplate.Execute(w, pageData); err != nil {
		s.logger.Error("Failed to serve discord users template", slog.Any("err", err))
	}
}
