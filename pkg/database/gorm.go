package database

import (
	"fmt"

	"github.com/ntthienan0507-web/go-api-template/pkg/config"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// NewGormDB creates a GORM database connection using the same DSN.
// Schema is managed by goose migrations — GORM does NOT run AutoMigrate.
func NewGormDB(cfg *config.Config, logger *zap.Logger) (*gorm.DB, error) {
	logLevel := gormlogger.Silent
	if cfg.ServerMode == "debug" {
		logLevel = gormlogger.Info
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("open gorm connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(int(cfg.DBMaxConns))
	sqlDB.SetMaxIdleConns(int(cfg.DBMinConns))

	logger.Info("gorm database connected")
	return db, nil
}
