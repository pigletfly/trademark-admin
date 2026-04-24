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
	"github.com/google/uuid"

	api "github.com/pigletfly/trademark-admin/apps/api"
	"github.com/pigletfly/trademark-admin/apps/api/internal/auth"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/audit"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/config"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/httpx"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/logger"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/database"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		_, _ = os.Stderr.WriteString("config error: " + err.Error() + "\n")
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel, cfg.AppEnv)

	// Run pending migrations.
	mig, err := migrator.New(api.Migrations, "migrations", cfg.DatabaseURL)
	if err != nil {
		log.Error("migrator init", "error", err)
		os.Exit(1)
	}
	if err := mig.Up(); err != nil {
		log.Error("migrate up", "error", err)
		os.Exit(1)
	}
	_ = mig.Close()
	log.Info("migrations applied")

	// Open GORM handle.
	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Error("open database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close(db) }()

	// Build auth service.
	authRepo := auth.NewRepository(db)
	authSvc := auth.NewService(auth.ServiceConfig{
		Repo:          authRepo,
		AccessSecret:  []byte(cfg.JWTAccessSecret),
		RefreshSecret: []byte(cfg.JWTRefreshSecret),
		AccessTTL:     cfg.JWTAccessTTL,
		RefreshTTL:    cfg.JWTRefreshTTL,
	})

	// Bootstrap admin if requested and users table is empty.
	if cfg.BootstrapAdminEmail != "" && cfg.BootstrapAdminPassword != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := authSvc.Bootstrap(ctx, cfg.BootstrapAdminEmail, cfg.BootstrapAdminPassword, "Bootstrap Admin"); err != nil {
			cancel()
			log.Error("bootstrap admin", "error", err)
			os.Exit(1)
		}
		cancel()
		log.Info("bootstrap admin ensured", "email", cfg.BootstrapAdminEmail)
	} else {
		log.Warn("BOOTSTRAP_ADMIN_EMAIL/PASSWORD not set; skipping initial admin creation")
	}

	authHandler := auth.NewHandler(auth.HandlerConfig{
		Service:      authSvc,
		CookieSecure: cfg.CookieSecure,
		AccessTTL:    cfg.JWTAccessTTL,
		RefreshTTL:   cfg.JWTRefreshTTL,
	})

	if cfg.AppEnv != "development" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/health", httpx.Health(db))

	// API v1 groups.
	v1 := router.Group("/api/v1")
	public := v1.Group("")
	authed := v1.Group("")
	authed.Use(auth.RequireAuth([]byte(cfg.JWTAccessSecret)), auth.CSRF())

	auth.RegisterRoutes(public, authed, authHandler)

	// Audit plumbing
	auditRepo := audit.NewRepository(db)
	auditMW := audit.Middleware(auditRepo, func(c *gin.Context) (uuid.UUID, bool) {
		u := auth.CurrentUser(c)
		if u.ID == uuid.Nil {
			return uuid.Nil, false
		}
		return u.ID, true
	}, log)

	// Admin routes require auth + role=admin + CSRF + audit middleware
	adminGroup := v1.Group("")
	adminGroup.Use(auth.RequireAuth([]byte(cfg.JWTAccessSecret)),
		auth.RequireRole("admin"),
		auth.CSRF(),
		auditMW,
	)
	adminUserHandler := auth.NewAdminHandler(auth.NewAdminService(authRepo))
	auth.RegisterAdminRoutes(adminGroup, adminUserHandler)
	audit.RegisterAdminRoutes(adminGroup, audit.NewAdminHandler(auditRepo))

	srv := &http.Server{
		Addr:              cfg.HTTPListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

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
