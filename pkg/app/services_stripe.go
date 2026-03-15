package app

import (
	"context"

	"go.uber.org/zap"

	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
	"github.com/ntthienan0507-web/gostack-kit/pkg/external/stripe"
)

func init() { registerOptionalService(initStripe) }

func initStripe(_ context.Context, cfg *config.Config, logger *zap.Logger, s *Services) error {
	if cfg.StripeSecretKey == "" {
		return nil
	}
	s.register("stripe", stripe.New(stripe.Config{
		BaseURL:   cfg.StripeURL,
		SecretKey: cfg.StripeSecretKey,
	}, logger))
	logger.Info("stripe client initialized")
	return nil
}

// Stripe returns the Stripe client, or nil if not configured.
func (s *Services) Stripe() *stripe.Client {
	v, _ := s.lookup("stripe").(*stripe.Client)
	return v
}
