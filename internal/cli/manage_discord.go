package cli

import (
	"context"
	"log/slog"
	"time"

	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storediscord"
)

func runManageDiscord(ctx context.Context, logger *slog.Logger, database *db.DB, args []string) StopReason {
	if len(args) == 0 {
		logger.Error("Missing action for manage discord (expected: enable | disable)")
		return StopReasonBadArgsError
	}

	switch args[0] {
	case "enable":
		return runManageDiscordEnable(ctx, logger, database, args[1:])
	case "disable":
		return runManageDiscordDisable(ctx, logger, database, args[1:])
	default:
		logger.Error("Unknown manage discord action", slog.String("action", args[0]))
		return StopReasonBadArgsError
	}
}

func runManageDiscordEnable(ctx context.Context, logger *slog.Logger, database *db.DB, args []string) StopReason {
	if len(args) != 2 {
		logger.Error("manage discord enable expects exactly 2 args: <user-id> <guild-id>", slog.Int("got", len(args)))
		return StopReasonBadArgsError
	}

	userID, ok := parseID(logger, "user-id", args[0])
	if !ok {
		return StopReasonBadArgsError
	}

	guildID, ok := parseID(logger, "guild-id", args[1])
	if !ok {
		return StopReasonBadArgsError
	}

	if err := database.WriteTx(ctx, func(tx *db.Tx) error {
		return storediscord.InsertOrEnableUser(tx, userID, guildID, time.Now())
	}); err != nil {
		logger.Error("Failed to enable discord user", slog.Any("err", err))
		return StopReasonManageError
	}

	logger.Info("Enabled discord user", slog.Uint64("userID", userID), slog.Uint64("presenceGuildID", guildID))
	return StopReasonManageDone
}

func runManageDiscordDisable(ctx context.Context, logger *slog.Logger, database *db.DB, args []string) StopReason {
	userID, ok := parseSingleUserID(logger, "manage discord disable", args)
	if !ok {
		return StopReasonBadArgsError
	}

	if err := database.WriteTx(ctx, func(tx *db.Tx) error {
		return storediscord.DisableUser(tx, userID, time.Now())
	}); err != nil {
		logger.Error("Failed to disable discord user", slog.Any("err", err))
		return StopReasonManageError
	}

	logger.Info("Disabled discord user", slog.Uint64("userID", userID))
	return StopReasonManageDone
}
