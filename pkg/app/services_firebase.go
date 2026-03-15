package app

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
	"github.com/ntthienan0507-web/gostack-kit/pkg/external/firebase"
)

func init() { registerOptionalService(initFirebase) }

func initFirebase(ctx context.Context, cfg *config.Config, logger *zap.Logger, s *Services) error {
	if cfg.FirebaseCredentialsFile == "" {
		return nil
	}
	fb, err := firebase.New(ctx, firebase.Config{
		CredentialsFile: cfg.FirebaseCredentialsFile,
		ProjectID:       cfg.FirebaseProjectID,
	}, logger)
	if err != nil {
		return fmt.Errorf("init firebase: %w", err)
	}
	s.register("firebase", fb)
	logger.Info("firebase client initialized")
	return nil
}

// Firebase returns the Firebase client, or nil if not configured.
func (s *Services) Firebase() *firebase.Client {
	v, _ := s.lookup("firebase").(*firebase.Client)
	return v
}
