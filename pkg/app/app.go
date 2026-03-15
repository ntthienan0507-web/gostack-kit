package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"

	db "github.com/ntthienan0507-web/gostack-kit/db/sqlc"
	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
	"github.com/ntthienan0507-web/gostack-kit/pkg/async"
	"github.com/ntthienan0507-web/gostack-kit/pkg/auth"
	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
	"github.com/ntthienan0507-web/gostack-kit/pkg/database"
	"github.com/ntthienan0507-web/gostack-kit/pkg/middleware"
	usermodule "github.com/ntthienan0507-web/gostack-kit/modules/user"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "github.com/ntthienan0507-web/gostack-kit/docs"
)

type App struct {
	cfg          *config.Config
	logger       *zap.Logger
	pool         *pgxpool.Pool
	gormDB       *gorm.DB
	store        *database.Store
	queries      *db.Queries
	Redis        *redis.Client
	authProvider auth.Provider
	router       *gin.Engine
	server       *http.Server
	Workers      *async.WorkerPool
	Services     *Services
}

func New(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*App, error) {
	a := &App{cfg: cfg, logger: logger}

	switch cfg.DBDriver {
	case "gorm":
		gormDB, err := database.NewGormDB(cfg, logger)
		if err != nil { return nil, fmt.Errorf("init gorm: %w", err) }
		a.gormDB = gormDB
	default:
		pool, err := database.NewPool(ctx, cfg)
		if err != nil { return nil, fmt.Errorf("init db pool: %w", err) }
		a.pool = pool
		a.store = database.NewStore(pool)
		a.queries = db.New(pool)
	}

	if cfg.RedisURL != "" {
		rdb, err := database.NewRedis(ctx, cfg, logger)
		if err != nil { return nil, fmt.Errorf("init redis: %w", err) }
		a.Redis = rdb
	}

	authProvider, err := auth.NewProvider(cfg)
	if err != nil { return nil, fmt.Errorf("init auth: %w", err) }
	a.authProvider = authProvider

	a.Workers = async.NewWorkerPool(cfg.WorkerCount, cfg.WorkerQueueSize, logger)

	services, err := initServices(ctx, cfg, logger)
	if err != nil { return nil, fmt.Errorf("init services: %w", err) }
	a.Services = services

	a.setupRouter()
	return a, nil
}

func (a *App) setupRouter() {
	if a.cfg.ServerMode == "release" { gin.SetMode(gin.ReleaseMode) }

	r := gin.New()
	r.Use(middleware.Recovery(a.logger), middleware.RequestLogger(a.logger), middleware.CORS("*"))
	if a.cfg.ServerMode != "release" { r.Use(middleware.ResponseAudit(a.logger)) }

	api := r.Group("/api/v1")
	api.GET("/healthz", func(ctx *gin.Context) { ctx.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	api.GET("/readyz", a.ReadinessHandler())
	a.registerModules(api)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.NoRoute(func(ctx *gin.Context) { apperror.Respond(ctx, apperror.ErrRouteNotFound) })
	a.router = r
}

func (a *App) registerModules(api *gin.RouterGroup) {
	// User module
	var userRepo usermodule.Repository
	switch a.cfg.DBDriver {
	case "gorm":  userRepo = usermodule.NewGORMRepository(a.gormDB)
	default:      userRepo = usermodule.NewSQLCRepository(a.queries)
	}
	userSvc := usermodule.NewService(userRepo, a.logger)
	userCtrl := usermodule.NewController(userSvc, a.logger)
	usermodule.NewRoutes(userCtrl).Register(api, a.authProvider)
}

func (a *App) Run(ctx context.Context) error {
	a.server = &http.Server{Addr: fmt.Sprintf(":%d", a.cfg.ServerPort), Handler: a.router, ReadHeaderTimeout: 10 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("starting server", zap.String("addr", a.server.Addr))
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed { errCh <- err }
		close(errCh)
	}()
	select {
	case err := <-errCh: return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return a.server.Shutdown(shutdownCtx)
	}
}

type DBPinger interface { Ping(ctx context.Context) error }

func (a *App) ReadinessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var p DBPinger
		if a.pool != nil { p = a.pool } else if a.gormDB != nil { p = &gormPinger{a.gormDB} }
		ReadinessCheck(c, p, a.Redis)
	}
}

func ReadinessCheck(c *gin.Context, dbPinger DBPinger, rdb *redis.Client) {
	ctx := c.Request.Context()
	checks := gin.H{}
	ready := true
	if dbPinger == nil { checks["database"] = "skipped"
	} else if err := dbPinger.Ping(ctx); err != nil { checks["database"] = "failed: " + err.Error(); ready = false
	} else { checks["database"] = "ok" }
	if rdb == nil { checks["redis"] = "skipped"
	} else if err := rdb.Ping(ctx).Err(); err != nil { checks["redis"] = "failed: " + err.Error(); ready = false
	} else { checks["redis"] = "ok" }
	status := http.StatusOK; if !ready { status = http.StatusServiceUnavailable }
	statusText := "ready"; if !ready { statusText = "not_ready" }
	c.JSON(status, gin.H{"status": statusText, "checks": checks})
}

type gormPinger struct{ db *gorm.DB }
func (g *gormPinger) Ping(ctx context.Context) error { s, err := g.db.DB(); if err != nil { return err }; return s.PingContext(ctx) }

func (a *App) Shutdown() {
	a.logger.Info("shutting down")
	if a.Workers != nil { a.Workers.Shutdown() }
	if a.Services != nil { a.Services.shutdown(a.logger) }
	if a.Redis != nil { a.Redis.Close() }
	if a.pool != nil { a.pool.Close() }
	if a.gormDB != nil { if s, err := a.gormDB.DB(); err == nil { s.Close() } }
	_ = a.logger.Sync()
}
