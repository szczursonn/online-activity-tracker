package steam

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storesteam"
	"github.com/szczursonn/online-activity-tracker/internal/steam/storefrontapi"
	"github.com/szczursonn/online-activity-tracker/internal/steam/webapi"
)

const errPrefix = "steam: "

type TrackerOptions struct {
	Logger                         *slog.Logger
	DB                             *db.DB
	WebAPI                         *webapi.Client
	StorefrontAPI                  *storefrontapi.Client
	PollInterval                   time.Duration
	AppExtraInfoStaleCheckInterval time.Duration
	AppExtraInfoStaleTime          time.Duration
}

func (opts *TrackerOptions) validateAndApplyDefaults() error {
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}

	if opts.DB == nil {
		return fmt.Errorf(errPrefix + "missing db")
	}

	if opts.WebAPI == nil {
		return fmt.Errorf(errPrefix + "missing web api client")
	}

	if opts.StorefrontAPI == nil {
		return fmt.Errorf(errPrefix + "missing storefront api client")
	}

	if opts.PollInterval <= 0 {
		return fmt.Errorf(errPrefix + "missing poll interval")
	}

	if opts.AppExtraInfoStaleCheckInterval <= 0 {
		return fmt.Errorf(errPrefix + "missing app extra info stale check interval")
	}

	if opts.AppExtraInfoStaleTime <= 0 {
		return fmt.Errorf(errPrefix + "missing app extra info stale time")
	}

	return nil
}

type Tracker struct {
	logger                         *slog.Logger
	db                             *db.DB
	webAPI                         *webapi.Client
	storefrontAPI                  *storefrontapi.Client
	pollInterval                   time.Duration
	appExtraInfoStaleCheckInterval time.Duration
	appExtraInfoStaleTime          time.Duration

	hasHeartbeatContinuity bool
}

func NewTracker(opts TrackerOptions) (*Tracker, error) {
	if err := opts.validateAndApplyDefaults(); err != nil {
		return nil, err
	}

	return &Tracker{
		logger:                         opts.Logger,
		db:                             opts.DB,
		webAPI:                         opts.WebAPI,
		storefrontAPI:                  opts.StorefrontAPI,
		pollInterval:                   opts.PollInterval,
		appExtraInfoStaleCheckInterval: opts.AppExtraInfoStaleCheckInterval,
		appExtraInfoStaleTime:          opts.AppExtraInfoStaleTime,
	}, nil
}

func (t *Tracker) Run(ctx context.Context) {
	var workerWg sync.WaitGroup
	workerWg.Go(func() {
		t.playerSummariesPollWorker(ctx)
	})
	workerWg.Go(func() {
		t.appDetailsPollWorker(ctx)
	})
	workerWg.Wait()
}

func (t *Tracker) playerSummariesPollWorker(ctx context.Context) {
	ticker := time.NewTicker(t.pollInterval)
	defer ticker.Stop()

	for {
		t.doPlayerSummariesPoll(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (t *Tracker) doPlayerSummariesPoll(pollCtx context.Context) {
	t.logger.Debug("Polling player summaries")
	ctx, cancelCtx := context.WithTimeout(pollCtx, time.Minute)
	defer cancelCtx()

	if err := func() error {
		var enabledUserIDs []uint64
		if err := t.db.ReadTx(ctx, func(tx *db.Tx) (err error) {
			enabledUserIDs, err = storesteam.GetEnabledUserIDs(tx)
			return
		}); err != nil {
			return fmt.Errorf("in pre-poll tx: %w", err)
		}

		if len(enabledUserIDs) == 0 {
			return nil
		}

		playerSummariesByUserID, err := t.webAPI.FetchPlayerSummaries(ctx, enabledUserIDs)
		pollTimestamp := time.Now()
		if err != nil {
			return fmt.Errorf("fetching player summaries: %w", err)
		}

		if err := t.db.WriteTx(ctx, func(tx *db.Tx) error {
			heartbeatTs, err := storesteam.GetHeartbeat(tx)
			if err != nil {
				return err
			}

			if !t.hasHeartbeatContinuity {
				// if last run was unsuccessful (or it's the first run), close all existing sessions (because there was a period of time in which player summaries were not monitored)
				if err := storesteam.CloseAllSessions(tx, heartbeatTs); err != nil {
					return err
				}
			}

			newPersonaStateByUserID := make(map[uint64]*int, len(playerSummariesByUserID))
			newAppIDByUserID := make(map[uint64]*uint64, len(playerSummariesByUserID))

			for _, userID := range enabledUserIDs {
				playerSummary, hasPlayerSummary := playerSummariesByUserID[userID]
				if hasPlayerSummary {
					newPersonaStateByUserID[userID] = &playerSummary.PersonaState
					newAppIDByUserID[userID] = &playerSummary.GameID

					if err := storesteam.UpdateUserExtraInfo(tx, userID, playerSummary.PersonaName, playerSummary.ProfileURL, playerSummary.AvatarURL, pollTimestamp); err != nil {
						return err
					}
				} else {
					newPersonaStateByUserID[userID] = nil
					newAppIDByUserID[userID] = nil
				}

			}

			if err := storesteam.SyncPersonaStateSessions(tx, heartbeatTs, pollTimestamp, newPersonaStateByUserID); err != nil {
				return err
			}

			if err := storesteam.SyncAppSessions(tx, heartbeatTs, pollTimestamp, newAppIDByUserID); err != nil {
				return err
			}

			if err := storesteam.UpsertHeartbeat(tx, pollTimestamp); err != nil {
				return err
			}

			return nil
		}); err != nil {
			return fmt.Errorf("in post-poll tx: %w", err)
		}

		return nil
	}(); err != nil {
		t.hasHeartbeatContinuity = false
		t.logger.Error("Failed to poll player summaries", slog.Any("err", fmt.Errorf(errPrefix+"%w", err)))
	} else {
		t.hasHeartbeatContinuity = true
		t.logger.Debug("Successfully polled player summaries")
	}
}

func (t *Tracker) appDetailsPollWorker(ctx context.Context) {
	ticker := time.NewTicker(t.appExtraInfoStaleCheckInterval)
	defer ticker.Stop()

	for {
		t.doAppDetailsRefresh(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (t *Tracker) doAppDetailsRefresh(refreshCtx context.Context) {
	t.logger.Debug("Refreshing app details")
	ctx, cancelCtx := context.WithTimeout(refreshCtx, time.Minute)
	defer cancelCtx()

	if err := func() error {
		var appID uint64
		if err := t.db.ReadTx(ctx, func(tx *db.Tx) (err error) {
			appID, err = storesteam.GetLeastUpdatedExtraInfoAppID(tx, time.Now().Add(-t.appExtraInfoStaleTime))
			return
		}); err != nil && !errors.Is(err, db.ErrNotFound) {
			return fmt.Errorf("in read tx: %w", err)
		}

		if appID == 0 {
			t.logger.Debug("No app to refresh")
			return nil
		}

		appDetail, err := t.storefrontAPI.FetchAppDetails(ctx, appID)
		if err != nil {
			return fmt.Errorf("fetching app details for app id %d: %w", appID, err)
		}
		fetchTimestamp := time.Now()

		if err := t.db.WriteTx(ctx, func(tx *db.Tx) error {
			if err := storesteam.UpdateAppExtraInfo(tx, appDetail.ID, appDetail.Name, appDetail.HeaderImageURL, fetchTimestamp); err != nil {
				return err
			}

			return nil
		}); err != nil {
			return fmt.Errorf("in write tx: %w", err)
		}

		return nil
	}(); err != nil {
		t.logger.Error("Failed to refresh app details", slog.Any("err", err))
	} else {
		t.logger.Debug("Successfully refreshed app details")
	}
}
