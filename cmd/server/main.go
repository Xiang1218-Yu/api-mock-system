// Command server is the single entrypoint for the api-mock platform.
// It constructs a signal-cancellable context and hands control to app.Run.
// Keeping main tiny means the composition root (package app) is the only
// place that knows how the system is wired.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"api-mock-system/internal/app"

	"go.uber.org/zap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil {
		// Best-effort log; the logger may not be constructed yet.
		if l, e := zap.NewProduction(); e == nil {
			l.Fatal("server failed", zap.Error(err))
		} else {
			os.Exit(1)
		}
	}
}
