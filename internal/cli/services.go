package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/szczursonn/online-activity-tracker/internal/cli/config"
	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/discord"
	"github.com/szczursonn/online-activity-tracker/internal/steam"
	"github.com/szczursonn/online-activity-tracker/internal/steam/storefrontapi"
	"github.com/szczursonn/online-activity-tracker/internal/steam/webapi"
	"github.com/szczursonn/online-activity-tracker/internal/viewer"
)

const serviceLogKey = "service"

const (
	serviceNameSteam   = "steam"
	serviceNameDiscord = "discord"
	serviceNameViewer  = "viewer"
)

type service struct {
	logger *slog.Logger
	run    func(ctx context.Context) error
	close  func()
}

func runServicesCommand(ctx context.Context, logger *slog.Logger, database *db.DB, cfg *config.Config, serviceNames []string) StopReason {
	enablement, err := parseServiceNames(serviceNames)
	if err != nil {
		logger.Error("Invalid service names", slog.Any("err", err))
		return StopReasonBadArgsError
	}

	services, err := setupServices(ctx, logger, database, cfg, enablement)
	if err != nil {
		logger.Error("Failed to setup services", slog.Any("err", err))
		return StopReasonSetupError
	}

	return runServices(ctx, services)
}

func setupServices(ctx context.Context, logger *slog.Logger, database *db.DB, cfg *config.Config, enablement servicesEnablement) ([]service, error) {
	ctx, cancelCtx := context.WithCancel(ctx)
	defer cancelCtx()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var successfulServices []service
	var errs []error

	addResult := func(svc service, err error) {
		mu.Lock()
		defer mu.Unlock()

		if err != nil {
			errs = append(errs, err)
			cancelCtx()
			return
		}

		successfulServices = append(successfulServices, svc)
	}

	if enablement.steamTracker {
		wg.Go(func() {
			addResult(setupSteamTrackerService(logger.With(slog.String(serviceLogKey, serviceNameSteam)), database, cfg.Steam))
		})
	}

	if enablement.discordTracker {
		wg.Go(func() {
			addResult(setupDiscordTrackerService(ctx, logger.With(slog.String(serviceLogKey, serviceNameDiscord)), database, cfg.Discord))
		})
	}

	if enablement.viewer {
		wg.Go(func() {
			addResult(setupViewerServerService(logger.With(slog.String(serviceLogKey, serviceNameViewer)), database, cfg.Viewer))
		})
	}

	wg.Wait()

	if len(errs) > 0 {
		closeServices(successfulServices)
		return nil, errors.Join(errs...)
	}

	return successfulServices, nil
}

func setupSteamTrackerService(logger *slog.Logger, database *db.DB, cfg config.Steam) (service, error) {
	webAPI, err := webapi.NewClient(&webapi.ClientOptions{Key: cfg.Key})
	if err != nil {
		return service{}, fmt.Errorf("creating steam web api client: %w", err)
	}

	tracker, err := steam.NewTracker(steam.TrackerOptions{
		Logger:                         logger,
		DB:                             database,
		WebAPI:                         webAPI,
		StorefrontAPI:                  storefrontapi.NewClient(&storefrontapi.ClientOptions{}),
		PollInterval:                   cfg.PollInterval,
		AppExtraInfoStaleCheckInterval: cfg.AppExtraInfoStaleCheckInterval,
		AppExtraInfoStaleTime:          cfg.AppExtraInfoStaleTime,
	})
	if err != nil {
		return service{}, fmt.Errorf("creating steam tracker: %w", err)
	}

	return service{
		logger: logger,
		run: func(ctx context.Context) error {
			tracker.Run(ctx)
			return nil
		},
	}, nil
}

func setupDiscordTrackerService(ctx context.Context, logger *slog.Logger, database *db.DB, cfg config.Discord) (service, error) {
	tracker, err := discord.NewTracker(ctx, discord.TrackerOptions{
		Logger: logger,
		DB:     database,
		Token:  cfg.Token,
	})
	if err != nil {
		return service{}, fmt.Errorf("creating discord tracker: %w", err)
	}

	return service{
		logger: logger,
		run:    tracker.Run,
		close:  tracker.Close,
	}, nil
}

func setupViewerServerService(logger *slog.Logger, database *db.DB, cfg config.Viewer) (service, error) {
	srv, err := viewer.NewServer(&viewer.ServerOptions{
		Logger:     logger,
		DB:         database,
		ListenAddr: cfg.ListenAddr,
	})
	if err != nil {
		return service{}, fmt.Errorf("creating viewer server: %w", err)
	}
	logger.Info("Claimed address", slog.String("listenAddr", srv.ListenAddr()))

	return service{
		logger: logger,
		run:    srv.Run,
		close:  srv.Close,
	}, nil
}

// runServices runs every service until they are all done, the first one to finish stops the rest
func runServices(ctx context.Context, services []service) StopReason {
	ctx, cancelCtx := context.WithCancel(ctx)
	defer cancelCtx()

	var wg sync.WaitGroup
	var isErr atomic.Bool

	for _, svc := range services {
		wg.Go(func() {
			defer svc.logger.Debug("Service stopped")
			defer cancelCtx()

			svc.logger.Info("Service started")
			if err := svc.run(ctx); err != nil {
				svc.logger.Error("Service failed", slog.Any("err", err))
				isErr.Store(true)
			}
		})
	}

	wg.Wait()

	if isErr.Load() {
		return StopReasonServicesUnexpectedShutdown
	}

	return StopReasonServicesShutdown
}

func closeServices(services []service) {
	for _, svc := range services {
		if svc.close != nil {
			svc.close()
		}
	}
}

type servicesEnablement struct {
	steamTracker   bool
	discordTracker bool
	viewer         bool
}

func parseServiceNames(serviceNames []string) (parsed servicesEnablement, err error) {
	for _, serviceName := range serviceNames {
		switch serviceName {
		case serviceNameSteam:
			parsed.steamTracker = true
		case serviceNameDiscord:
			parsed.discordTracker = true
		case serviceNameViewer:
			parsed.viewer = true
		default:
			err = fmt.Errorf("unknown service %q", serviceName)
			return
		}
	}

	if !parsed.steamTracker && !parsed.discordTracker && !parsed.viewer {
		err = fmt.Errorf("at least one service required")
	}

	return
}
