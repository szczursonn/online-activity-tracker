package cli

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/szczursonn/online-activity-tracker/internal/db"
)

func runManage(ctx context.Context, logger *slog.Logger, database *db.DB, args []string) StopReason {
	if len(args) == 0 {
		logger.Error("Missing subcommand for manage (expected: db | steam | discord)")
		return StopReasonBadArgsError
	}

	switch args[0] {
	case "db":
		return runManageDB(ctx, logger, database, args[1:])
	case "steam":
		return runManageSteam(ctx, logger, database, args[1:])
	case "discord":
		return runManageDiscord(ctx, logger, database, args[1:])
	default:
		logger.Error("Unknown manage subcommand", slog.String("subcommand", args[0]))
		return StopReasonBadArgsError
	}
}

func parseSingleUserID(logger *slog.Logger, cmdLabel string, args []string) (uint64, bool) {
	if len(args) != 1 {
		logger.Error(cmdLabel+" expects exactly 1 arg: <user-id>", slog.Int("got", len(args)))
		return 0, false
	}

	return parseID(logger, "user-id", args[0])
}

func parseID(logger *slog.Logger, name string, value string) (uint64, bool) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		logger.Error("Invalid "+name, slog.String("value", value), slog.Any("err", err))
		return 0, false
	}

	return id, true
}
