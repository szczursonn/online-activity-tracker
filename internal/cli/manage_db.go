package cli

import (
	"context"
	"log/slog"

	"github.com/szczursonn/online-activity-tracker/internal/db"
)

func runManageDB(ctx context.Context, logger *slog.Logger, database *db.DB, args []string) StopReason {
	if len(args) == 0 {
		logger.Error("Missing action for manage db (expected: migrate | status | vacuum)")
		return StopReasonBadArgsError
	}

	switch args[0] {
	case "migrate":
		return runManageDBMigrate(ctx, logger, database, args[1:])
	case "status":
		return runManageDBStatus(ctx, logger, database, args[1:])
	case "vacuum":
		return runManageDBVacuum(ctx, logger, database, args[1:])
	default:
		logger.Error("Unknown manage db action", slog.String("action", args[0]))
		return StopReasonBadArgsError
	}
}

func runManageDBMigrate(ctx context.Context, logger *slog.Logger, database *db.DB, args []string) StopReason {
	if len(args) != 0 {
		logger.Error("manage db migrate expects no args", slog.Int("got", len(args)))
		return StopReasonBadArgsError
	}

	if err := database.RunMigrations(ctx, db.MigrationHooks{
		OnStatus: func(status db.MigrationStatus) {
			if status.PendingCount == 0 {
				logger.Info("Database is up to date", slog.String("lastApplied", status.LastApplied))
				return
			}

			logger.Info("Running migrations...", slog.Int("pendingCount", status.PendingCount), slog.String("lastApplied", status.LastApplied))
		},
		OnStart: func(name string, left int) {
			logger.Info("Applying migration...", slog.String("name", name), slog.Int("left", left))
		},
		OnDone: func(name string) {
			logger.Debug("Migration applied", slog.String("name", name))
		},
	}); err != nil {
		logger.Error("Failed to run migrations", slog.Any("err", err))
		return StopReasonManageError
	}

	logger.Info("Done!")
	return StopReasonManageDone
}

func runManageDBStatus(ctx context.Context, logger *slog.Logger, database *db.DB, args []string) StopReason {
	if len(args) != 0 {
		logger.Error("manage db status expects no args", slog.Int("got", len(args)))
		return StopReasonBadArgsError
	}

	status, err := database.MigrationStatus(ctx)
	if err != nil {
		logger.Error("Failed to get migration status", slog.Any("err", err))
		return StopReasonManageError
	}

	if status.PendingCount == 0 {
		logger.Info("Database is up to date")
	} else {
		logger.Info("Database is out of date", slog.Int("pendingMigrations", status.PendingCount), slog.String("lastAppliedMigration", status.LastApplied))
	}

	return StopReasonManageDone
}

func runManageDBVacuum(ctx context.Context, logger *slog.Logger, database *db.DB, args []string) StopReason {
	if len(args) > 1 {
		logger.Error("manage db vacuum expects at most 1 arg: <path>", slog.Int("got", len(args)))
		return StopReasonBadArgsError
	}

	if len(args) == 0 {
		logger.Info("Running VACUUM...")
		if err := database.Vacuum(ctx); err != nil {
			logger.Error("Vacuum failed", slog.Any("err", err))
			return StopReasonManageError
		}

		logger.Debug("Running WAL checkpoint truncate...")
		if err := database.CheckpointTruncate(ctx); err != nil {
			logger.Error("Checkpoint failed", slog.Any("err", err))
			return StopReasonManageError
		}
	} else {
		logger.Info("Running VACUUM INTO...", slog.String("destination", args[0]))
		if err := database.VacuumInto(ctx, args[0]); err != nil {
			logger.Error("Vacuum into failed", slog.Any("err", err))
			return StopReasonManageError
		}
	}

	logger.Info("Done!")
	return StopReasonManageDone
}
