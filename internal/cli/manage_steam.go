package cli

import (
	"context"
	"log/slog"
	"time"

	"github.com/szczursonn/online-activity-tracker/internal/db"
	"github.com/szczursonn/online-activity-tracker/internal/db/storesteam"
)

func runManageSteam(ctx context.Context, logger *slog.Logger, database *db.DB, args []string) StopReason {
	if len(args) == 0 {
		logger.Error("Missing action for manage steam (expected: enable | disable)")
		return StopReasonBadArgsError
	}

	switch args[0] {
	case "enable":
		return runManageSteamEnable(ctx, logger, database, args[1:])
	case "disable":
		return runManageSteamDisable(ctx, logger, database, args[1:])
	default:
		logger.Error("Unknown manage steam action", slog.String("action", args[0]))
		return StopReasonBadArgsError
	}
}

func runManageSteamEnable(ctx context.Context, logger *slog.Logger, database *db.DB, args []string) StopReason {
	userID, ok := parseSingleUserID(logger, "manage steam enable", args)
	if !ok {
		return StopReasonBadArgsError
	}

	if err := database.WriteTx(ctx, func(tx *db.Tx) error {
		return storesteam.InsertOrEnableUser(tx, userID)
	}); err != nil {
		logger.Error("Failed to enable steam user", slog.Any("err", err))
		return StopReasonManageError
	}

	logger.Info("Enabled steam user", slog.Uint64("userID", userID))
	return StopReasonManageDone
}

func runManageSteamDisable(ctx context.Context, logger *slog.Logger, database *db.DB, args []string) StopReason {
	userID, ok := parseSingleUserID(logger, "manage steam disable", args)
	if !ok {
		return StopReasonBadArgsError
	}

	if err := database.WriteTx(ctx, func(tx *db.Tx) error {
		return storesteam.DisableUser(tx, userID, time.Now())
	}); err != nil {
		logger.Error("Failed to disable steam user", slog.Any("err", err))
		return StopReasonManageError
	}

	logger.Info("Disabled steam user", slog.Uint64("userID", userID))
	return StopReasonManageDone
}
