package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"gorm.io/gorm"

	db "github.com/ntthienan0507-web/go-api-template/db/sqlc"
	"github.com/ntthienan0507-web/go-api-template/internal/auth"
	"github.com/ntthienan0507-web/go-api-template/internal/config"
	"github.com/ntthienan0507-web/go-api-template/internal/database"
	"github.com/ntthienan0507-web/go-api-template/internal/middleware"
	usermodule "github.com/ntthienan0507-web/go-api-template/internal/module/user"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "github.com/ntthienan0507-web/go-api-template/docs"
)

// App is the DI container. All dependencies live here — NO globals.
type App struct {
	cfg          *config.Config
	logger       *zap.Logger
	pool         *pgxpool.Pool // used by SQLC driver
	gormDB       *gorm.DB     // used by GORM driver
	store        *database.Store
	queries      *db.Queries
	authProvider auth.Provider
	router       *gin.Engine
}

// New wires all dependencies and returns a ready App.
// DB_DRIVER config selects between SQLC (pgxpool) and GORM.
func New(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*App, error) {
	a := &App{
		cfg:    cfg,
		logger: logger,
	}

	// Database — init based on driver selection
	switch cfg.DBDriver {
	case "gorm":
		gormDB, err := database.NewGormDB(cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("init gorm: %w", err)
		}
		a.gormDB = gormDB
		logger.Info("using GORM database driver")

	default: // "sqlc" (default)
		pool, err := database.NewPool(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("init db pool: %w", err)
		}
		a.pool = pool
		a.store = database.NewStore(pool)
		a.queries = db.New(pool)
		logger.Info("using SQLC database driver")
	}

	// Auth
	authProvider, err := auth.NewProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("init auth: %w", err)
	}
	a.authProvider = authProvider

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

	api.GET("/healthz", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	a.registerModules(api)

	// Swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.NoRoute(func(ctx *gin.Context) {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error":             404,
			"error_description": "Route not found",
		})
	})

	a.router = r
}

func (a *App) registerModules(api *gin.RouterGroup) {
	// User module — repository selection based on DB_DRIVER
	var userRepo usermodule.Repository
	switch a.cfg.DBDriver {
	case "gorm":
		userRepo = usermodule.NewGORMRepository(a.gormDB)
	default:
		userRepo = usermodule.NewSQLCRepository(a.queries)
	}

	userSvc := usermodule.NewService(userRepo, a.logger)
	userHandler := usermodule.NewHandler(userSvc, a.logger)
	usermodule.RegisterRoutes(api, userHandler, a.authProvider)

	// Add more modules following the same pattern:
	// var fooRepo foo.Repository
	// switch a.cfg.DBDriver {
	// case "gorm":
	//     fooRepo = foo.NewGORMRepository(a.gormDB)
	// default:
	//     fooRepo = foo.NewSQLCRepository(a.queries)
	// }
	// fooSvc := foo.NewService(fooRepo, a.logger)
	// fooH   := foo.NewHandler(fooSvc, a.logger)
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
	if a.pool != nil {
		a.pool.Close()
	}
	if a.gormDB != nil {
		if sqlDB, err := a.gormDB.DB(); err == nil {
			sqlDB.Close()
		}
	}
	_ = a.logger.Sync()
}
