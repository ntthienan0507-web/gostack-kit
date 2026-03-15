package app

import (
	"context"

	"go.uber.org/zap"

	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
	"github.com/ntthienan0507-web/gostack-kit/pkg/external/icewarp"
)

func init() { registerOptionalService(initIceWarp) }

func initIceWarp(_ context.Context, cfg *config.Config, logger *zap.Logger, s *Services) error {
	if cfg.IceWarpURL == "" || cfg.IceWarpUsername == "" {
		return nil
	}
	s.register("icewarp", icewarp.New(icewarp.Config{
		URL:      cfg.IceWarpURL,
		Username: cfg.IceWarpUsername,
		Password: cfg.IceWarpPassword,
		From:     cfg.IceWarpFrom,
	}, logger))
	logger.Info("icewarp client initialized")
	return nil
}

// IceWarp returns the IceWarp client, or nil if not configured.
func (s *Services) IceWarp() *icewarp.Client {
	v, _ := s.lookup("icewarp").(*icewarp.Client)
	return v
}
