package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"

	"github.com/szczursonn/online-activity-tracker/internal/cli/config"
	"github.com/szczursonn/online-activity-tracker/internal/db"
)

type StopReason int

const (
	StopReasonSetupError StopReason = iota
	StopReasonBadArgsError
	StopReasonServicesShutdown
	StopReasonServicesUnexpectedShutdown
	StopReasonManageDone
	StopReasonManageError
)

const defaultConfigFilePath = "oat.toml"

func Run(ctx context.Context, args []string, w io.Writer) StopReason {
	stopReason := run(ctx, args, w)

	if stopReason == StopReasonBadArgsError {
		fmt.Fprint(w, `Commands:
  manage db migrate                           Run database migrations
  manage db status                            Show applied and pending migrations
  manage db vacuum                            Optimize the database (reduce size)
  manage db vacuum <path>                     Create copy of the database
  manage steam enable <user-id>               Enable tracking for a Steam user
  manage steam disable <user-id>              Disable tracking for a Steam user
  manage discord enable <user-id> <guild-id>  Enable tracking for a Discord user
  manage discord disable <user-id>            Disable tracking for a Discord user
  run <service>...                            Run services (steam | discord | viewer)

Flags:
  --config <path>   Config file path (default: oat.toml)
`)
	}

	return stopReason
}

func run(ctx context.Context, args []string, w io.Writer) StopReason {
	const (
		commandManage = "manage"
		commandRun    = "run"
	)

	flagSet := flag.NewFlagSet("oat", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	// usage is printed by Run
	flagSet.Usage = func() {}
	var configFilePath string
	flagSet.StringVar(&configFilePath, "config", defaultConfigFilePath, "Config file path")

	if err := flagSet.Parse(args); err != nil {
		return StopReasonBadArgsError
	}

	commandArgs := flagSet.Args()
	if len(commandArgs) == 0 {
		return StopReasonBadArgsError
	}

	// command has to be known before the config is loaded, it decides which log format is used
	command := commandArgs[0]
	if command != commandManage && command != commandRun {
		return StopReasonBadArgsError
	}

	cfg, err := config.LoadTOMLFile(configFilePath)
	if err != nil {
		fmt.Fprintf(w, "Failed to load config file %q: %s\n", configFilePath, err)
		return StopReasonSetupError
	}

	logFormat := cfg.Logs.Format
	if command == commandManage {
		logFormat = cfg.Logs.ManageFormat
	}
	logger := newLogger(w, cfg.Logs.Debug, logFormat)

	logger.Debug("Config loaded", slog.Any("config", cfg))

	logger.Debug("Opening database...")
	database, err := db.Open(ctx, cfg.Database.Path)
	if err != nil {
		logger.Error("Failed to open database", slog.Any("err", err))
		return StopReasonSetupError
	}
	defer database.Close()
	logger.Debug("Database opened")

	if command == commandManage {
		return runManage(ctx, logger, database, commandArgs[1:])
	}

	return runServicesCommand(ctx, logger, database, cfg, commandArgs[1:])
}
