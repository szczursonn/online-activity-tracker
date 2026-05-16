package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storediscord"
	"github.com/szczursonn/online-activity-tracker/internal/db/storesteam"
	"github.com/szczursonn/online-activity-tracker/internal/discord"
	"github.com/szczursonn/online-activity-tracker/internal/steam"
	"github.com/szczursonn/online-activity-tracker/internal/steam/storefrontapi"
	"github.com/szczursonn/online-activity-tracker/internal/steam/webapi"
	"github.com/szczursonn/online-activity-tracker/internal/viewer"
)

func main() {
	os.Exit(run())
}

type appConfig struct {
	Debug        bool   `toml:"debug"`
	DatabasePath string `toml:"database_path"`
	Steam        struct {
		Key                            string        `toml:"key"`
		PollInterval                   time.Duration `toml:"poll_interval"`
		AppExtraInfoStaleCheckInterval time.Duration `toml:"app_extra_info_stale_check_interval"`
		AppExtraInfoStaleTime          time.Duration `toml:"app_extra_info_stale_time"`
	} `toml:"steam"`
	Discord struct {
		Token string `toml:"token"`
	} `toml:"discord"`
	Viewer struct {
		ListenAddr string `toml:"listen_addr"`
	} `toml:"viewer"`
}

func printUsage() {
	fmt.Fprint(os.Stderr, `Subcommands:
  migrate                              Run database migrations
  vacuum                               Optimize the database (reduce size)
  vacuum <path>                        Create copy of the database
  steam enable <user-id>               Enable tracking for a Steam user
  steam disable <user-id>              Disable tracking for a Steam user
  discord enable <user-id> <guild-id>  Enable tracking for a Discord user
  discord disable <user-id>            Disable tracking for a Discord user
  run [<service>...]        Run services (steam | discord | view)

Flags:
  --config <path>   Config file path (default: oat.toml)
`)
}

func run() int {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "oat.toml", "Config file path")
	flag.Usage = printUsage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		return 1
	}

	var cfg appConfig
	_, err := toml.DecodeFile(cfgPath, &cfg)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: func() slog.Level {
			if cfg.Debug {
				return slog.LevelDebug
			}
			return slog.LevelInfo
		}(),
	}))
	slog.SetDefault(logger)
	defer os.Stdout.Sync()
	if err != nil {
		logger.Error("Failed to load config", slog.Any("err", err))
		return 1
	}

	ctx, cancelCtx := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelCtx()
	go func() {
		// ensure subsequent signals insta-kill the app
		<-ctx.Done()
		cancelCtx()
	}()

	database, err := db.Open(ctx, cfg.DatabasePath)
	if err != nil {
		logger.Error("Failed to open database", slog.Any("err", err))
		return 1
	}
	defer database.Close()

	switch args[0] {
	case "migrate":
		return runMigrations(ctx, logger, database)
	case "run":
		return runServices(ctx, cancelCtx, logger, database, &cfg, args[1:])
	case "vacuum":
		return runVacuum(ctx, logger, database, args[1:])
	case "steam":
		return runSteamSubcommand(ctx, logger, database, args[1:])
	case "discord":
		return runDiscordSubcommand(ctx, logger, database, args[1:])
	default:
		logger.Error("Unknown subcommand", slog.String("subcommand", args[0]))
		return 1
	}
}

func runMigrations(ctx context.Context, logger *slog.Logger, database *db.DB) int {
	logger.Info("Running migrations...")
	if err := database.RunMigrations(ctx); err != nil {
		logger.Error("Failed to run migrations", slog.Any("err", err))
		return 1
	}

	logger.Info("Done!")
	return 0
}

func runServices(ctx context.Context, cancelCtx context.CancelFunc, logger *slog.Logger, database *db.DB, cfg *appConfig, services []string) int {
	wantSteam, wantDiscord, wantView, err := parseServices(services)
	if err != nil {
		logger.Error("Invalid services", slog.Any("err", err))
		return 1
	}

	var wg sync.WaitGroup
	var isErr atomic.Bool

	if wantSteam {
		wg.Go(func() {
			defer logger.Debug("Steam tracker done")
			defer cancelCtx()

			webAPI, err := webapi.NewClient(&webapi.ClientOptions{Key: cfg.Steam.Key})
			if err != nil {
				logger.Error("Failed to create steam web api client", slog.Any("err", err))
				isErr.Store(true)
				return
			}

			tracker, err := steam.NewTracker(steam.TrackerOptions{
				Logger:                         logger,
				DB:                             database,
				WebAPI:                         webAPI,
				StorefrontAPI:                  storefrontapi.NewClient(&storefrontapi.ClientOptions{}),
				PollInterval:                   cfg.Steam.PollInterval,
				AppExtraInfoStaleCheckInterval: cfg.Steam.AppExtraInfoStaleCheckInterval,
				AppExtraInfoStaleTime:          cfg.Steam.AppExtraInfoStaleTime,
			})
			if err != nil {
				logger.Error("Failed to start steam tracker", slog.Any("err", err))
				isErr.Store(true)
				return
			}
			defer tracker.Shutdown()
			logger.Info("Started steam tracker")

			<-ctx.Done()
		})
	}

	if wantDiscord {
		wg.Go(func() {
			defer logger.Debug("Discord tracker done")
			defer cancelCtx()

			tracker, err := discord.NewTracker(ctx, discord.TrackerOptions{
				Logger: logger,
				DB:     database,
				Token:  cfg.Discord.Token,
			})
			if err != nil {
				logger.Error("Failed to start discord tracker", slog.Any("err", err))
				isErr.Store(true)
				return
			}
			logger.Info("Started discord tracker")

			select {
			case <-tracker.Done():
				isErr.Store(true)
			case <-ctx.Done():
				tracker.InitShutdown()
				<-tracker.Done()
			}
		})
	}

	if wantView {
		wg.Go(func() {
			defer logger.Debug("View server done")
			defer cancelCtx()

			srv, err := viewer.NewServer(&viewer.ServerOptions{
				Logger:     logger,
				DB:         database,
				ListenAddr: cfg.Viewer.ListenAddr,
			})
			if err != nil {
				logger.Error("Failed to start viewer server", slog.Any("err", err))
				isErr.Store(true)
				return
			}
			defer srv.Shutdown()

			go func() {
				if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					logger.Error("HTTP server failed", slog.Any("err", err))
					cancelCtx()
				}
			}()

			logger.Info("Started HTTP viewer server", slog.String("listenAddr", srv.ListenAddr()))
			<-ctx.Done()
		})
	}

	wg.Wait()

	if isErr.Load() {
		return 1
	}

	return 0
}

func parseServices(tokens []string) (steam bool, discord bool, view bool, err error) {
	for _, t := range tokens {
		switch t {
		case "steam":
			steam = true
		case "discord":
			discord = true
		case "view":
			view = true
		default:
			err = fmt.Errorf("unknown service %s (allowed: steam, discord, view)", t)
			return
		}
	}

	if !steam && !discord && !view {
		err = fmt.Errorf("at least one service required (allowed: steam, discord, view)")
	}

	return
}

func runVacuum(ctx context.Context, logger *slog.Logger, database *db.DB, args []string) int {
	if len(args) == 0 {
		logger.Info("Running VACUUM...")
		if err := database.Vacuum(ctx); err != nil {
			logger.Error("Vacuum failed", slog.Any("err", err))
			return 1
		}

		if err := database.CheckpointTruncate(ctx); err != nil {
			logger.Error("Checkpoint failed", slog.Any("err", err))
			return 1
		}
	} else {
		logger.Info("Running VACUUM INTO...", slog.String("destination", args[0]))
		if err := database.VacuumInto(ctx, args[0]); err != nil {
			logger.Error("Vacuum into failed", slog.Any("err", err))
			return 1
		}
	}

	logger.Info("Done")
	return 0
}

func runSteamSubcommand(ctx context.Context, logger *slog.Logger, database *db.DB, args []string) int {
	if len(args) == 0 {
		logger.Error("Missing action for steam subcommand (expected: enable | disable)")
		return 1
	}
	switch args[0] {
	case "enable":
		userID, ok := parseSingleUserID(logger, "steam enable", args[1:])
		if !ok {
			return 1
		}
		return runEnableSteam(ctx, logger, database, userID)
	case "disable":
		userID, ok := parseSingleUserID(logger, "steam disable", args[1:])
		if !ok {
			return 1
		}
		return runDisableSteam(ctx, logger, database, userID)
	default:
		logger.Error("Unknown steam action", slog.String("action", args[0]))
		return 1
	}
}

func runDiscordSubcommand(ctx context.Context, logger *slog.Logger, database *db.DB, args []string) int {
	if len(args) == 0 {
		logger.Error("Missing action for discord subcommand (expected: enable | disable)")
		return 1
	}
	switch args[0] {
	case "enable":
		rest := args[1:]
		if len(rest) != 2 {
			logger.Error("discord enable expects exactly 2 args: <user-id> <guild-id>", slog.Int("got", len(rest)))
			return 1
		}
		userID, err := strconv.ParseUint(rest[0], 10, 64)
		if err != nil {
			logger.Error("Invalid user-id", slog.String("value", rest[0]), slog.Any("err", err))
			return 1
		}
		guildID, err := strconv.ParseUint(rest[1], 10, 64)
		if err != nil {
			logger.Error("Invalid guild-id", slog.String("value", rest[1]), slog.Any("err", err))
			return 1
		}
		return runEnableDiscord(ctx, logger, database, userID, guildID)
	case "disable":
		userID, ok := parseSingleUserID(logger, "discord disable", args[1:])
		if !ok {
			return 1
		}
		return runDisableDiscord(ctx, logger, database, userID)
	default:
		logger.Error("Unknown discord action", slog.String("action", args[0]))
		return 1
	}
}

func parseSingleUserID(logger *slog.Logger, cmdLabel string, args []string) (uint64, bool) {
	if len(args) != 1 {
		logger.Error(cmdLabel+" expects exactly 1 arg: <user-id>", slog.Int("got", len(args)))
		return 0, false
	}
	userID, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		logger.Error("Invalid user-id", slog.String("value", args[0]), slog.Any("err", err))
		return 0, false
	}
	return userID, true
}

func runEnableSteam(ctx context.Context, logger *slog.Logger, database *db.DB, userID uint64) int {
	if err := database.WriteTx(ctx, func(tx *db.Tx) error {
		return storesteam.InsertOrEnableUser(tx, userID)
	}); err != nil {
		logger.Error("Failed to enable steam user", slog.Any("err", err))
		return 1
	}

	logger.Info("Enabled steam user", slog.Uint64("userID", userID))
	return 0
}

func runDisableSteam(ctx context.Context, logger *slog.Logger, database *db.DB, userID uint64) int {
	if err := database.WriteTx(ctx, func(tx *db.Tx) error {
		return storesteam.DisableUser(tx, userID, time.Now())
	}); err != nil {
		logger.Error("Failed to disable steam user", slog.Any("err", err))
		return 1
	}

	logger.Info("Disabled steam user", slog.Uint64("userID", userID))
	return 0
}

func runEnableDiscord(ctx context.Context, logger *slog.Logger, database *db.DB, userID, guildID uint64) int {
	if err := database.WriteTx(ctx, func(tx *db.Tx) error {
		return storediscord.InsertOrEnableUser(tx, userID, guildID, time.Now())
	}); err != nil {
		logger.Error("Failed to enable discord user", slog.Any("err", err))
		return 1
	}

	logger.Info("Enabled discord user", slog.Uint64("userID", userID), slog.Uint64("presenceGuildID", guildID))
	return 0
}

func runDisableDiscord(ctx context.Context, logger *slog.Logger, database *db.DB, userID uint64) int {
	if err := database.WriteTx(ctx, func(tx *db.Tx) error {
		return storediscord.DisableUser(tx, userID, time.Now())
	}); err != nil {
		logger.Error("Failed to disable discord user", slog.Any("err", err))
		return 1
	}

	logger.Info("Disabled discord user", slog.Uint64("userID", userID))
	return 0
}
