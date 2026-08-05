package cli

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/szczursonn/online-activity-tracker/internal/cli/config"
	"github.com/szczursonn/online-activity-tracker/internal/logs/simpletext"
)

func newLogger(w io.Writer, debug bool, format config.LogFormat) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	var handler slog.Handler
	switch format {
	case config.LogFormatJSON:
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level: level,
		})
	case config.LogFormatText:
		handler = slog.NewTextHandler(w, &slog.HandlerOptions{
			Level: level,
		})
	case config.LogFormatSimpleText:
		handler = simpletext.NewHandler(w, &simpletext.HandlerOptions{
			Level: level,
		})
	default:
		panic(fmt.Errorf("invalid log format %q", format))
	}

	return slog.New(handler)
}
