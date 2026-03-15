// Package feature provides a simple feature flag system backed by config/env.
//
// Flags are plain bools loaded from environment variables (via mapstructure).
// The Manager supports runtime updates, flag checks by name, and gin middleware
// for maintenance mode and per-feature gating.
//
// Usage:
//
//	flags := feature.Flags{EnableBetaAPI: true}
//	fm := feature.New(flags, logger)
//
//	// Check in code
//	if fm.IsEnabled("beta_api") { ... }
//
//	// Middleware: block all traffic in maintenance mode
//	router.Use(fm.MaintenanceMiddleware())
//
//	// Middleware: gate a route behind a flag
//	router.GET("/beta/feature", fm.RequireFlag("beta_api"), handler)
package feature

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Flags holds all feature flags. Loaded from config/env.
// Add new flags as fields — they're just bools.
type Flags struct {
	EnableWebSocket bool `mapstructure:"FF_WEBSOCKET" json:"websocket"`
	EnableBetaAPI   bool `mapstructure:"FF_BETA_API" json:"beta_api"`
	EnableKafka     bool `mapstructure:"FF_KAFKA" json:"kafka"`
	MaintenanceMode bool `mapstructure:"FF_MAINTENANCE" json:"maintenance"`
}

// flagLookup maps json tag names to a getter function.
// This avoids reflection at runtime — just a simple map lookup.
func flagLookup(f *Flags) map[string]bool {
	return map[string]bool{
		"websocket":   f.EnableWebSocket,
		"beta_api":    f.EnableBetaAPI,
		"kafka":       f.EnableKafka,
		"maintenance": f.MaintenanceMode,
	}
}

// Manager holds flags and provides check methods.
type Manager struct {
	flags  Flags
	logger *zap.Logger
	mu     sync.RWMutex
}

// New creates a Manager with the given flags.
func New(flags Flags, logger *zap.Logger) *Manager {
	logger.Info("feature flags loaded",
		zap.Bool("websocket", flags.EnableWebSocket),
		zap.Bool("beta_api", flags.EnableBetaAPI),
		zap.Bool("kafka", flags.EnableKafka),
		zap.Bool("maintenance", flags.MaintenanceMode),
	)
	return &Manager{flags: flags, logger: logger}
}

// IsEnabled checks if a named flag is enabled.
// Name must match the json tag (e.g. "beta_api", "websocket", "kafka", "maintenance").
// Returns false for unknown flag names.
func (m *Manager) IsEnabled(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	lookup := flagLookup(&m.flags)
	return lookup[name]
}

// All returns a copy of all flags (for admin API / debugging).
func (m *Manager) All() Flags {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.flags
}

// Update replaces flags at runtime (e.g. from admin API or config reload).
func (m *Manager) Update(flags Flags) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flags = flags
	m.logger.Info("feature flags updated",
		zap.Bool("websocket", flags.EnableWebSocket),
		zap.Bool("beta_api", flags.EnableBetaAPI),
		zap.Bool("kafka", flags.EnableKafka),
		zap.Bool("maintenance", flags.MaintenanceMode),
	)
}

// MaintenanceMiddleware returns a gin middleware that blocks requests when
// MaintenanceMode is true. Returns 503 with a maintenance message.
func (m *Manager) MaintenanceMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if m.IsEnabled("maintenance") {
			ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error_code":    http.StatusServiceUnavailable,
				"error_message": "common.maintenance",
				"error_detail":  "Service is under maintenance. Please try again later.",
			})
			return
		}
		ctx.Next()
	}
}

// RequireFlag returns middleware that checks a specific flag.
// If the flag is disabled, returns 404 (feature not available).
//
//	router.GET("/beta/feature", featureManager.RequireFlag("beta_api"), handler)
func (m *Manager) RequireFlag(name string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if !m.IsEnabled(name) {
			ctx.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error_code":    http.StatusNotFound,
				"error_message": "common.feature_not_available",
				"error_detail":  "This feature is not currently available.",
			})
			return
		}
		ctx.Next()
	}
}
