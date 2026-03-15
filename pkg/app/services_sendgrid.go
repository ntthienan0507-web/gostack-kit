package app

import (
	"context"

	"go.uber.org/zap"

	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
	"github.com/ntthienan0507-web/gostack-kit/pkg/external/sendgrid"
)

func init() { registerOptionalService(initSendGrid) }

func initSendGrid(_ context.Context, cfg *config.Config, logger *zap.Logger, s *Services) error {
	if cfg.SendGridAPIKey == "" {
		return nil
	}
	s.register("sendgrid", sendgrid.New(sendgrid.Config{
		BaseURL:   cfg.SendGridURL,
		APIKey:    cfg.SendGridAPIKey,
		FromEmail: cfg.SendGridFrom,
	}, logger))
	logger.Info("sendgrid client initialized")
	return nil
}

// SendGrid returns the SendGrid client, or nil if not configured.
func (s *Services) SendGrid() *sendgrid.Client {
	v, _ := s.lookup("sendgrid").(*sendgrid.Client)
	return v
}
