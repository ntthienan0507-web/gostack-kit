package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	db "github.com/chungnguyen/go-api-template/db/sqlc"
	"github.com/chungnguyen/go-api-template/internal/auth"
	"github.com/chungnguyen/go-api-template/internal/config"
	"github.com/chungnguyen/go-api-template/internal/database"
	"github.com/chungnguyen/go-api-template/internal/middleware"
	usermodule "github.com/chungnguyen/go-api-template/internal/module/user"
)

// App is the DI container. All dependencies live here — NO globals.
type App struct {
	cfg          *config.Config
	logger       *zap.Logger
	pool         *pgxpool.Pool
	store        *database.Store
	queries      *db.Queries
	authProvider auth.Provider
	router       *gin.Engine
}

// New wires all dependencies and returns a ready App.
func New(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*App, error) {
	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("init db pool: %w", err)
	}

	store := database.NewStore(pool)
	queries := db.New(pool)

	authProvider, err := auth.NewProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("init auth: %w", err)
	}

	a := &App{
		cfg:          cfg,
		logger:       logger,
		pool:         pool,
		store:        store,
		queries:      queries,
		authProvider: authProvider,
	}

	a.setupRouter()
	return a, nil
}

func (a *App) setupRouter() {
	if a.cfg.ServerMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(
		middleware.Recovery(a.logger),
		middleware.RequestLogger(a.logger),
		middleware.CORS("*"),
	)

	api := r.Group("/api/v1")

	// Health check (no auth required)
	api.GET("/healthz", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	a.registerModules(api)

	r.NoRoute(func(ctx *gin.Context) {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error":             404,
			"error_description": "Route not found",
		})
	})

	a.router = r
}

func (a *App) registerModules(api *gin.RouterGroup) {
	// User module — reference CRUD implementation
	userRepo := usermodule.NewRepository(a.queries)
	userSvc := usermodule.NewService(userRepo, a.logger)
	userHandler := usermodule.NewHandler(userSvc, a.logger)
	usermodule.RegisterRoutes(api, userHandler, a.authProvider)

	// Add more modules here following the same pattern:
	// fooRepo := foo.NewRepository(a.queries)
	// fooSvc  := foo.NewService(fooRepo, a.logger)
	// fooH    := foo.NewHandler(fooSvc, a.logger)
	// foo.RegisterRoutes(api, fooH, a.authProvider)
}

// Run starts the HTTP server.
func (a *App) Run() error {
	addr := fmt.Sprintf(":%d", a.cfg.ServerPort)
	a.logger.Info("starting server", zap.String("addr", addr))
	return a.router.Run(addr)
}

// Shutdown cleanly releases resources.
func (a *App) Shutdown() {
	a.logger.Info("shutting down")
	a.pool.Close()
	_ = a.logger.Sync()
}
