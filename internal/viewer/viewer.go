package viewer

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/szczursonn/online-activity-tracker/internal/db"
)

const errPrefix = "viewer: "

//go:embed static
var staticFS embed.FS

type ServerOptions struct {
	Logger     *slog.Logger
	DB         *db.DB
	ListenAddr string
}

func (opts *ServerOptions) validateAndApplyDefaults() error {
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}

	if opts.DB == nil {
		return fmt.Errorf(errPrefix + "missing db")
	}

	if opts.ListenAddr == "" {
		return fmt.Errorf(errPrefix + "missing listen addr")
	}

	return nil
}

type Server struct {
	logger   *slog.Logger
	db       *db.DB
	srv      *http.Server
	listener net.Listener
}

func NewServer(opts *ServerOptions) (*Server, error) {
	if err := opts.validateAndApplyDefaults(); err != nil {
		return nil, err
	}

	s := &Server{
		db:     opts.DB,
		logger: opts.Logger,
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))

	registerHomePage(s, mux)
	registerDiscordUsersPage(s, mux)
	registerDiscordUserPage(s, mux)
	registerDiscordGuildsPage(s, mux)
	registerDiscordGuildPage(s, mux)
	registerSteamUsersPage(s, mux)
	registerSteamUserPage(s, mux)
	registerSteamAppsPage(s, mux)
	registerSteamAppPage(s, mux)

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		respondNotFound(w)
	})

	s.srv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      30 * time.Second,
		ErrorLog:          slog.NewLogLogger(s.logger.Handler(), slog.LevelWarn),
	}

	var err error
	s.listener, err = net.Listen("tcp", opts.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf(errPrefix+"listening on %q: %w", opts.ListenAddr, err)
	}

	return s, nil
}

func (s *Server) Close() {
	s.srv.Shutdown(context.Background())
	s.listener.Close()
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.srv.Serve(s.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf(errPrefix+"serving: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return nil
	}
}

func (s *Server) ListenAddr() string {
	return s.listener.Addr().String()
}

func respondInternalServerError(w http.ResponseWriter) {
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

func respondBadRequest(w http.ResponseWriter) {
	http.Error(w, "Bad Request", http.StatusBadRequest)
}

func respondNotFound(w http.ResponseWriter) {
	http.Error(w, "Not Found", http.StatusNotFound)
}
