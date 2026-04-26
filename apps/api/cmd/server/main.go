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
	"github.com/pigletfly/trademark-admin/apps/api/internal/catalog"
	"github.com/pigletfly/trademark-admin/apps/api/internal/customer"
	"github.com/pigletfly/trademark-admin/apps/api/internal/dashboard"
	"github.com/pigletfly/trademark-admin/apps/api/internal/export"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/audit"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/config"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/httpx"
	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/logger"
	"github.com/pigletfly/trademark-admin/apps/api/internal/pricing"
	"github.com/pigletfly/trademark-admin/apps/api/internal/quotation"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/database"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/migrator"
	"github.com/pigletfly/trademark-admin/apps/api/pkg/seeder"
)

// pricingRepoAdapter bridges pricing.Repository.ListActive(ctx, ActiveFilter)
// into the pricingRepo interface that quotation.Service expects
// (ListActive(ctx, *uuid.UUID)).
type pricingRepoAdapter struct{ *pricing.Repository }

func (a pricingRepoAdapter) ListActive(ctx context.Context, countryID *uuid.UUID) ([]pricing.PricingEntry, error) {
	return a.Repository.ListActive(ctx, pricing.ActiveFilter{CountryID: countryID})
}

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

	// Seed catalog dictionaries (idempotent).
	{
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := seeder.Run(ctx, db, api.SeedFS, "seed/countries.json", "seed/nice_categories.json")
		cancel()
		if err != nil {
			log.Error("seed catalog", "error", err)
			os.Exit(1)
		}
		log.Info("catalog seeded")
	}

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

	// Build audit middleware first so both authed and adminGroup can chain it.
	auditRepo := audit.NewRepository(db)
	auditMW := audit.Middleware(auditRepo, func(c *gin.Context) (uuid.UUID, bool) {
		u := auth.CurrentUser(c)
		if u.ID == uuid.Nil {
			return uuid.Nil, false
		}
		return u.ID, true
	}, log)

	// Authenticated routes for any logged-in user.
	authed := v1.Group("")
	authed.Use(auth.RequireAuth([]byte(cfg.JWTAccessSecret)), auth.CSRF(), auditMW)
	auth.RegisterRoutes(public, authed, authHandler)

	// Catalog: read endpoints on authed; write endpoints on adminGroup below.
	catalogRepo := catalog.NewRepository(db)
	catalogSvc := catalog.NewService(catalogRepo)
	catalogHandler := catalog.NewHandler(catalogSvc)
	catalog.RegisterReadRoutes(authed, catalogHandler)

	// Customers — owner scoping handled inside the service.
	custRepo := customer.NewRepository(db)
	custSvc := customer.NewService(custRepo)
	custHandler := customer.NewHandler(custSvc)
	customer.RegisterRoutes(authed, custHandler)

	// Pricing entries — reviewer+admin read, admin write.
	pricingRepo := pricing.NewRepository(db)
	pricingSvc := pricing.NewService(pricingRepo)
	pricingHandler := pricing.NewHandler(pricingSvc)

	reviewerAdminGroup := v1.Group("")
	reviewerAdminGroup.Use(auth.RequireAuth([]byte(cfg.JWTAccessSecret)),
		auth.RequireRole("reviewer", "admin"),
		auth.CSRF(),
		auditMW,
	)
	pricing.RegisterReadRoutes(reviewerAdminGroup, pricingHandler)

	// Admin-only routes: auth + role=admin + CSRF + audit.
	adminGroup := v1.Group("")
	adminGroup.Use(auth.RequireAuth([]byte(cfg.JWTAccessSecret)),
		auth.RequireRole("admin"),
		auth.CSRF(),
		auditMW,
	)
	adminUserHandler := auth.NewAdminHandler(auth.NewAdminService(authRepo))
	auth.RegisterAdminRoutes(adminGroup, adminUserHandler)
	audit.RegisterAdminRoutes(adminGroup, audit.NewAdminHandler(auditRepo))
	catalog.RegisterAdminRoutes(adminGroup, catalogHandler)
	pricing.RegisterAdminRoutes(adminGroup, pricingHandler)

	// Quotations — any authed user creates/lists own; reviewer+admin
	// sees all + approves/rejects. Salesperson ownership is enforced
	// inside the handler layer.
	quotRepo := quotation.NewRepository(db)
	quotSvc := quotation.NewService(quotRepo, pricingRepoAdapter{pricingRepo}, custRepo)
	quotHandler := quotation.NewHandler(quotSvc)
	quotation.RegisterAuthedRoutes(authed, quotHandler)
	quotation.RegisterReviewerRoutes(reviewerAdminGroup, quotHandler)

	// Export — any authed user may download their own; reviewer/admin
	// may download any. Only approved quotations are exportable.
	exportRepo := export.NewRepository(db)
	exportStorage := export.NewStorage(cfg.ExportStorageRoot)
	gotenbergClient := export.NewGotenberg(cfg.GotenbergURL)
	exportSvc := export.NewService(exportRepo, exportStorage, gotenbergClient, cfg.ExportTTL)
	exportSigner := export.NewSigner([]byte(cfg.ExportSigningSecret))
	exportHandler := export.NewHandler(quotSvc, custSvc, catalogRepo, exportSvc, exportSigner)
	export.RegisterRoutes(authed, exportHandler)            // legacy GET export.docx
	export.RegisterAuthedRoutes(authed, exportHandler)      // POST /quotations/:id/export
	export.RegisterPublicRoutes(public, exportHandler)      // GET /exports/:id/download (token-auth)

	// Dashboard — any authed user. Scope (self vs firm) is decided
	// inside the service based on role.
	dashSvc := dashboard.NewService(quotRepo, custRepo)
	dashHandler := dashboard.NewHandler(dashSvc)
	dashboard.RegisterRoutes(authed, dashHandler)

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
