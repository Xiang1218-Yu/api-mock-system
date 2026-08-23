// Package app is the composition root. It wires every dependency once at
// startup, starts background jobs, builds the HTTP router, and serves.
//
// This is the only place that knows the concrete wiring: the order of
// construction, which repository feeds which service, and which handlers the
// router receives. Changing a dependency's implementation means editing here
// (or in the implementation's own package), not scattered across the codebase.
package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"api-mock-system/internal/aggregatehandler"
	"api-mock-system/internal/aggregaterepo"
	"api-mock-system/internal/aggregateservice"
	"api-mock-system/internal/apihandler"
	"api-mock-system/internal/apirepo"
	"api-mock-system/internal/apiservice"
	"api-mock-system/internal/auth"
	"api-mock-system/internal/cache"
	"api-mock-system/internal/calllogrepo"
	"api-mock-system/internal/config"
	"api-mock-system/internal/dashboardhandler"
	"api-mock-system/internal/dashboardservice"
	"api-mock-system/internal/debughandler"
	"api-mock-system/internal/debugrepo"
	"api-mock-system/internal/debugservice"
	"api-mock-system/internal/dochandler"
	"api-mock-system/internal/docservice"
	"api-mock-system/internal/httpx"
	"api-mock-system/internal/logger"
	"api-mock-system/internal/mockdatarepo"
	"api-mock-system/internal/mockhandler"
	"api-mock-system/internal/mockservice"
	"api-mock-system/internal/projecthandler"
	"api-mock-system/internal/projectrepo"
	"api-mock-system/internal/projectservice"
	"api-mock-system/internal/router"
	"api-mock-system/internal/storage"
	"api-mock-system/internal/userhandler"
	"api-mock-system/internal/userrepo"
	"api-mock-system/internal/userservice"
	"api-mock-system/internal/web"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// App holds the running application's top-level handles for graceful shutdown.
type App struct {
	server *http.Server
	store  *storage.Store
	log    *zap.Logger
}

// Run loads config, wires everything, and blocks serving until ctx is done.
func Run(ctx context.Context) error {
	cfg := config.Load()

	log, err := buildLogger(cfg)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	log.Info("starting api-mock platform",
		zap.String("env", cfg.Env),
		zap.String("port", cfg.ServerPort),
	)

	store, err := storage.Open(ctx, cfg.DBDSN, log)
	if err != nil {
		return err
	}

	authn, err := auth.New(cfg.JWTSecret, cfg.JWTExpiry)
	if err != nil {
		return err
	}
	sharedCache := cache.New()

	// --- repositories (data access) ---
	userR := userrepo.New(store.DB)
	projectR := projectrepo.New(store.DB)
	apiR := apirepo.New(store.DB)
	aggR := aggregaterepo.New(store.DB)
	mockR := mockdatarepo.New(store.DB)
	debugR := debugrepo.New(store.DB)
	calllogR := calllogrepo.New(store.DB)

	// --- services (business logic) ---
	users := userservice.New(userR, authn)
	projects := projectservice.New(projectR)
	apis := apiservice.New(apiR, projects)
	mock := mockservice.New(apis, mockR, sharedCache, log)
	aggregates := aggregateservice.New(aggR, apis, projects, cfg.MockBaseURL, cfg.AggrTimeout, log)
	docs := docservice.New(projects, apis)
	debug := debugservice.New(apis, mock, debugR, log)
	dashboard := dashboardservice.New(store.DB, apiR, aggR, projects, calllogR)

	// --- handlers (HTTP) ---
	deps := router.Deps{
		Users:     userhandler.New(users),
		Projects:  projecthandler.New(projects, users),
		APIs:      apihandler.New(apis),
		Mock:      mockhandler.New(mock, calllogR),
		Aggregate: aggregatehandler.New(aggregates, calllogR),
		Docs:      dochandler.New(docs),
		Debug:     debughandler.New(debug),
		Dashboard: dashboardhandler.New(dashboard),
		StaticFS:  web.FS,
	}
	engine := router.New(deps, authn, router.RateConfig{RPS: cfg.RateRPS, Burst: cfg.RateBurst}, log)

	// --- background jobs: cache cleanup every minute ---
	go runCacheCleanup(ctx, sharedCache, log)

	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      engine,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	app := &App{server: srv, store: store, log: log}

	// Serve in the background so the main goroutine can wait on ctx.
	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		return app.Shutdown()
	case err := <-errCh:
		_ = app.Shutdown()
		return err
	}
}

// Shutdown gracefully stops the HTTP server and closes the DB.
func (a *App) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a.log.Info("shutting down")
	if err := a.server.Shutdown(ctx); err != nil {
		a.log.Warn("http shutdown error", zap.Error(err))
	}
	return a.store.Close()
}

// buildLogger constructs the zap logger from config. Kept here so app.Run owns
// the single logger construction site.
func buildLogger(cfg *config.Config) (*zap.Logger, error) {
	l, err := loggerBuild(cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	return l, nil
}

// runCacheCleanup periodically purges expired cache entries, standing in for
// the gocron data-cleanup task in the spec.
func runCacheCleanup(ctx context.Context, c *cache.Cache, log *zap.Logger) {
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n := c.Purge(); n > 0 {
				log.Debug("cache purged", zap.Int("removed", n))
			}
		}
	}
}

// httpx import is referenced indirectly via handlers; keep it explicit so the
// dependency is visible at the composition root.
var _ = httpx.OK
var _ = gin.New

// loggerBuild is split into a variable so tests can stub it; production uses
// the real logger package.
var loggerBuild = logger.New
