package discord

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/bot/handlers"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storediscord"
	"github.com/szczursonn/online-activity-tracker/internal/logs/loglevellimit"
)

const errPrefix = "discord: "

type TrackerOptions struct {
	Logger *slog.Logger
	DB     *db.DB
	Token  string
}

func (opts *TrackerOptions) applyDefaultsAndValidate() error {
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}

	if opts.DB == nil {
		return fmt.Errorf(errPrefix + "missing db")
	}

	if opts.Token == "" {
		return fmt.Errorf(errPrefix + "missing token")
	}

	return nil
}

type Tracker struct {
	logger *slog.Logger
	db     *db.DB
	client *bot.Client

	taskQueue     chan func()
	hasContinuity bool

	ctx                        context.Context
	cancelCtx                  context.CancelFunc
	gatewayClosedPermanentlyCh chan error
}

func NewTracker(ctx context.Context, opts TrackerOptions) (*Tracker, error) {
	if err := opts.applyDefaultsAndValidate(); err != nil {
		return nil, err
	}

	t := &Tracker{
		logger:                     opts.Logger,
		db:                         opts.DB,
		taskQueue:                  make(chan func(), 500),
		gatewayClosedPermanentlyCh: make(chan error, 1),
	}
	t.ctx, t.cancelCtx = context.WithCancel(context.Background())

	// disgo's join/available/ready events only carry guild metadata, event though GUILD_CREATE has more stuff
	gatewayHandlers := handlers.GetGatewayHandlers()
	defaultGuildCreateHandler := gatewayHandlers[gateway.EventTypeGuildCreate]
	gatewayHandlers[gateway.EventTypeGuildCreate] = bot.NewGatewayEventHandler(gateway.EventTypeGuildCreate,
		func(client *bot.Client, sequenceNumber int, shardID int, event gateway.EventGuildCreate) {
			t.handleGuildCreateGatewayEvent(&event.GatewayGuild)
			defaultGuildCreateHandler.HandleGatewayEvent(client, sequenceNumber, shardID, event)
		},
	)

	var err error
	t.client, err = disgo.New(opts.Token,
		bot.WithLogger(
			slog.New(loglevellimit.NewLevelLimitHandler(opts.Logger.Handler(), slog.LevelWarn)).With(slog.String("module", "disgo")),
		),
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuilds,
				gateway.IntentGuildMembers,
				gateway.IntentGuildPresences,
				gateway.IntentGuildVoiceStates,
			),
			gateway.WithAutoReconnect(true),
			gateway.WithCloseHandler(func(_ gateway.Gateway, err error, _ bool) {
				select {
				case t.gatewayClosedPermanentlyCh <- fmt.Errorf(errPrefix+"gateway closed and cannot reconnect: %w", err):
				default:
				}
			}),
		),
		bot.WithEventManagerConfigOpts(bot.WithGatewayHandlers(gatewayHandlers)),
		bot.WithEventListeners(&events.ListenerAdapter{
			OnReady:                 t.handleReadyEvent,
			OnGuildLeave:            t.handleGuildLeaveEvent,
			OnGuildUnavailable:      t.handleGuildUnavailableEvent,
			OnGuildMemberLeave:      t.handleGuildMemberLeaveEvent,
			OnPresenceUpdate:        t.handlePresenceUpdateEvent,
			OnGuildVoiceStateUpdate: t.handleVoiceStateUpdateEvent,
		}),
	)
	if err != nil {
		t.cancelCtx()
		return nil, fmt.Errorf(errPrefix+"creating disgo client: %w", err)
	}

	if err := t.client.OpenGateway(ctx); err != nil {
		t.cancelCtx()
		t.client.Close(ctx)
		return nil, fmt.Errorf(errPrefix+"opening discord gateway: %w", err)
	}

	return t, nil
}

func (t *Tracker) Close() {
	t.cancelCtx()
	t.client.Close(context.Background())
}

func (t *Tracker) Run(ctx context.Context) error {
	heartbeatTicker := time.NewTicker(3 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-t.gatewayClosedPermanentlyCh:
			return err
		case <-heartbeatTicker.C:
			if t.client.Gateway.Status() == gateway.StatusReady {
				ts := time.Now()
				t.enqueueTask(func() { t.applyPeriodicHeartbeatUpdate(ts) })
			}
		case task := <-t.taskQueue:
			task()
		}
	}
}

func (t *Tracker) enqueueTask(task func()) {
	select {
	case t.taskQueue <- task:
	case <-t.ctx.Done():
	}
}

func (t *Tracker) applyPeriodicHeartbeatUpdate(ts time.Time) {
	if !t.hasContinuity {
		return
	}

	if err := t.db.WriteTx(t.ctx, func(tx *db.Tx) error {
		return storediscord.UpsertHeartbeat(tx, ts)
	}); err != nil {
		t.logger.Error("Periodic heartbeat update failed", slog.Any("err", err))
	}
}

func discordOnlineStatusToInt(status discord.OnlineStatus) int {
	switch status {
	case discord.OnlineStatusOnline:
		return 0
	case discord.OnlineStatusDND:
		return 1
	case discord.OnlineStatusIdle:
		return 2
	default:
		return 3
	}
}
