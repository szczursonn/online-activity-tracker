package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/szczursonn/online-activity-tracker/internal/cli"
)

func main() {
	os.Exit(run())
}

func run() (exitCode int) {
	ctx, cancelCtx := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelCtx()
	go func() {
		// ensure subsequent signals insta-kill the app
		<-ctx.Done()
		cancelCtx()
	}()

	switch cli.Run(ctx, os.Args[1:], os.Stderr) {
	case cli.StopReasonServicesShutdown, cli.StopReasonManageDone:
		exitCode = 0
	case cli.StopReasonSetupError, cli.StopReasonBadArgsError, cli.StopReasonManageError:
		exitCode = 1
	case cli.StopReasonServicesUnexpectedShutdown:
		exitCode = 2
	default:
		exitCode = 1
	}

	return
}
