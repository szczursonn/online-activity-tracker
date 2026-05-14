package viewer

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/szczursonn/online-activity-tracker/internal/db"
)

const errPrefix = "viewer: "

//go:embed templates/*.html
var templatesFS embed.FS

type ServerOptions struct {
	Logger     *slog.Logger
	DB         *db.DB
	ListenAddr string
}

func (opts *ServerOptions) validateAndApplyDefaults() error {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	if opts.DB == nil {
		return fmt.Errorf(errPrefix + "missing db")
	}

	if opts.ListenAddr == "" {
		opts.ListenAddr = ":8080"
	}

	return nil
}

type Server struct {
	logger *slog.Logger
	db     *db.DB
	srv    *http.Server

	homeTemplate         *template.Template
	steamUsersTemplate   *template.Template
	steamUserTemplate    *template.Template
	steamAppTemplate     *template.Template
	discordUsersTemplate *template.Template
	discordUserTemplate  *template.Template
}

func NewServer(opts *ServerOptions) (*Server, error) {
	if err := opts.validateAndApplyDefaults(); err != nil {
		return nil, err
	}

	s := &Server{
		db:     opts.DB,
		logger: opts.Logger.With(slog.String("module", "viewer")),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleHome)
	mux.HandleFunc("GET /steam/users", s.handleSteamUsers)
	mux.HandleFunc("GET /steam/users/{userID}", s.handleSteamUser)
	mux.HandleFunc("GET /steam/apps/{appID}", s.handleSteamApp)
	mux.HandleFunc("GET /discord/users", s.handleDiscordUsers)
	mux.HandleFunc("GET /discord/users/{userID}", s.handleDiscordUser)

	s.srv = &http.Server{
		Addr:              opts.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      30 * time.Second,
		ErrorLog:          slog.NewLogLogger(s.logger.Handler(), slog.LevelWarn),
	}

	var err error
	s.homeTemplate, err = template.ParseFS(templatesFS, "templates/_layout.html", "templates/home.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse home html template: %w", err)
	}

	s.steamUsersTemplate, err = template.ParseFS(templatesFS, "templates/_layout.html", "templates/steam_users.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse steam users html template: %w", err)
	}

	s.steamUserTemplate, err = template.ParseFS(templatesFS, "templates/_layout.html", "templates/steam_user.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse steam user html template: %w", err)
	}

	s.steamAppTemplate, err = template.ParseFS(templatesFS, "templates/_layout.html", "templates/steam_app.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse steam app html template: %w", err)
	}

	s.discordUsersTemplate, err = template.ParseFS(templatesFS, "templates/_layout.html", "templates/discord_users.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse discord users html template: %w", err)
	}

	s.discordUserTemplate, err = template.ParseFS(templatesFS, "templates/_layout.html", "templates/discord_user.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse discord user html template: %w", err)
	}

	return s, nil
}

func (s *Server) ListenAndServe() error {
	return s.srv.ListenAndServe()
}

func (s *Server) ListenAddr() string {
	return s.srv.Addr
}

func (s *Server) Shutdown() {
	s.srv.Shutdown(context.Background())
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
