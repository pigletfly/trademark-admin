package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/config"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/httpx"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/logger"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// logger not ready yet; print to stderr
		_, _ = os.Stderr.WriteString("config error: " + err.Error() + "\n")
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel, cfg.AppEnv)

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Error("open database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close(db) }()

	if cfg.AppEnv != "development" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/health", httpx.Health(db))

	srv := &http.Server{
		Addr:              cfg.HTTPListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown
	idle := make(chan struct{})
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		log.Info("shutdown requested")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Error("graceful shutdown", "error", err)
		}
		close(idle)
	}()

	log.Info("api listening", "addr", cfg.HTTPListenAddr, "env", cfg.AppEnv)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("listen", "error", err)
		os.Exit(1)
	}
	<-idle
	log.Info("api stopped")
}
