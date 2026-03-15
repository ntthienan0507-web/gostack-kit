package app

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
	"github.com/ntthienan0507-web/gostack-kit/pkg/external/elasticsearch"
)

func init() { registerOptionalService(initElasticsearch) }

func initElasticsearch(_ context.Context, cfg *config.Config, logger *zap.Logger, s *Services) error {
	if cfg.ElasticURLs == "" {
		return nil
	}
	es, err := elasticsearch.New(elasticsearch.Config{
		URLs:     cfg.ElasticURLList(),
		Username: cfg.ElasticUsername,
		Password: cfg.ElasticPassword,
		APIKey:   cfg.ElasticAPIKey,
	}, logger)
	if err != nil {
		return fmt.Errorf("init elasticsearch: %w", err)
	}
	s.register("elasticsearch", es)
	logger.Info("elasticsearch client initialized")
	return nil
}

// Elasticsearch returns the Elasticsearch client, or nil if not configured.
func (s *Services) Elasticsearch() *elasticsearch.Client {
	v, _ := s.lookup("elasticsearch").(*elasticsearch.Client)
	return v
}
