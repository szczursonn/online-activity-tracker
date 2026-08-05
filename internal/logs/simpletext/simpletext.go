package simpletext

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"time"
	"unicode"
)

type HandlerOptions struct {
	Level slog.Leveler
}

func (opts *HandlerOptions) applyDefaults() {
	if opts.Level == nil {
		opts.Level = slog.LevelInfo
	}
}

type handler struct {
	w         io.Writer
	mu        *sync.Mutex
	level     slog.Leveler
	startTime time.Time

	preformattedAttrs []byte
	groupPrefix       string
}

func NewHandler(w io.Writer, opts *HandlerOptions) slog.Handler {
	opts.applyDefaults()

	return &handler{
		w:         w,
		mu:        &sync.Mutex{},
		level:     opts.Level,
		startTime: time.Now(),
	}
}

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *handler) Handle(_ context.Context, record slog.Record) error {
	recordTime := record.Time
	if recordTime.IsZero() {
		recordTime = time.Now()
	}

	buf := make([]byte, 0, 256)
	buf = append(buf, '[')
	buf = append(buf, recordTime.Sub(h.startTime).Round(time.Millisecond).String()...)
	buf = append(buf, "] ["...)
	buf = append(buf, record.Level.String()...)
	buf = append(buf, "] "...)
	buf = append(buf, record.Message...)
	buf = append(buf, h.preformattedAttrs...)
	record.Attrs(func(attr slog.Attr) bool {
		buf = appendAttr(buf, h.groupPrefix, attr)
		return true
	})
	buf = append(buf, '\n')

	h.mu.Lock()
	defer h.mu.Unlock()

	_, err := h.w.Write(buf)
	return err
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	newHandler := h.clone()
	for _, attr := range attrs {
		newHandler.preformattedAttrs = appendAttr(newHandler.preformattedAttrs, h.groupPrefix, attr)
	}

	return newHandler
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	newHandler := h.clone()
	newHandler.groupPrefix += name + "."

	return newHandler
}

func (h *handler) clone() *handler {
	newHandler := *h
	newHandler.preformattedAttrs = make([]byte, len(h.preformattedAttrs))
	copy(newHandler.preformattedAttrs, h.preformattedAttrs)

	return &newHandler
}

func appendAttr(buf []byte, groupPrefix string, attr slog.Attr) []byte {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return buf
	}

	if attr.Value.Kind() == slog.KindGroup {
		groupAttrs := attr.Value.Group()
		if len(groupAttrs) == 0 {
			return buf
		}

		if attr.Key != "" {
			groupPrefix += attr.Key + "."
		}
		for _, groupAttr := range groupAttrs {
			buf = appendAttr(buf, groupPrefix, groupAttr)
		}

		return buf
	}

	buf = append(buf, ' ')
	buf = append(buf, groupPrefix...)
	buf = append(buf, attr.Key...)
	buf = append(buf, '=')

	value := attr.Value.String()
	if needsQuoting(value) {
		return strconv.AppendQuote(buf, value)
	}

	return append(buf, value...)
}

func needsQuoting(value string) bool {
	if value == "" {
		return true
	}

	for _, r := range value {
		if r == '"' || r == '=' || unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return true
		}
	}

	return false
}
