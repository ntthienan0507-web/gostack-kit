package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	elastic "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"go.uber.org/zap"
)

// Config holds Elasticsearch client configuration.
type Config struct {
	URLs     []string // cluster endpoint URLs
	Username string
	Password string
	APIKey   string
}

// Client wraps the Elasticsearch client with convenient operations.
type Client struct {
	es     *elastic.Client
	logger *zap.Logger
}

// New creates a new Elasticsearch client and pings the cluster to verify connectivity.
func New(cfg Config, logger *zap.Logger) (*Client, error) {
	esCfg := elastic.Config{
		Addresses: cfg.URLs,
		Username:  cfg.Username,
		Password:  cfg.Password,
	}
	if cfg.APIKey != "" {
		esCfg.APIKey = cfg.APIKey
	}

	es, err := elastic.NewClient(esCfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnFailed, err)
	}

	// Ping to verify connectivity
	resp, err := es.Ping()
	if err != nil {
		return nil, fmt.Errorf("%w: ping failed: %v", ErrConnFailed, err)
	}
	resp.Body.Close()

	if resp.IsError() {
		return nil, fmt.Errorf("%w: ping returned %s", ErrConnFailed, resp.Status())
	}

	logger.Info("elasticsearch connected", zap.Strings("urls", cfg.URLs))

	return &Client{
		es:     es,
		logger: logger,
	}, nil
}

// Index indexes a document in the specified index.
func (c *Client) Index(ctx context.Context, req IndexRequest) error {
	body, err := json.Marshal(req.Body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	indexReq := esapi.IndexRequest{
		Index:      req.Index,
		DocumentID: req.ID,
		Body:       bytes.NewReader(body),
	}

	resp, err := indexReq.Do(ctx, c.es)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrIndexFailed, err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("%w: %s", ErrIndexFailed, resp.Status())
	}

	return nil
}

// Search executes a search query against an index.
func (c *Client) Search(ctx context.Context, req SearchRequest) (*SearchResult, error) {
	query := buildSearchBody(req)

	resp, err := c.es.Search(
		c.es.Search.WithContext(ctx),
		c.es.Search.WithIndex(req.Index),
		c.es.Search.WithBody(bytes.NewReader(query)),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSearchFailed, err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, fmt.Errorf("%w: %s", ErrSearchFailed, resp.Status())
	}

	// Parse the Elasticsearch response
	var esResp struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", ErrSearchFailed, err)
	}

	hits := make([]json.RawMessage, len(esResp.Hits.Hits))
	for i, h := range esResp.Hits.Hits {
		hits[i] = h.Source
	}

	return &SearchResult{
		Total: esResp.Hits.Total.Value,
		Hits:  hits,
	}, nil
}

// Delete removes a document from the specified index.
func (c *Client) Delete(ctx context.Context, index, id string) error {
	resp, err := c.es.Delete(index, id,
		c.es.Delete.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDeleteFailed, err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("%w: %s", ErrDeleteFailed, resp.Status())
	}

	return nil
}

// Bulk performs a bulk operation with multiple items.
func (c *Client) Bulk(ctx context.Context, items []BulkItem) error {
	if len(items) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, item := range items {
		meta := map[string]any{
			item.Action: map[string]any{
				"_index": item.Index,
			},
		}
		if item.ID != "" {
			meta[item.Action].(map[string]any)["_id"] = item.ID
		}

		metaLine, err := json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("marshal bulk meta: %w", err)
		}
		buf.Write(metaLine)
		buf.WriteByte('\n')

		// delete action has no body
		if item.Action != "delete" && item.Body != nil {
			bodyLine, err := json.Marshal(item.Body)
			if err != nil {
				return fmt.Errorf("marshal bulk body: %w", err)
			}
			buf.Write(bodyLine)
			buf.WriteByte('\n')
		}
	}

	resp, err := c.es.Bulk(
		strings.NewReader(buf.String()),
		c.es.Bulk.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBulkFailed, err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("%w: %s", ErrBulkFailed, resp.Status())
	}

	// Check for item-level errors
	var bulkResp struct {
		Errors bool `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bulkResp); err != nil {
		return fmt.Errorf("%w: decode response: %v", ErrBulkFailed, err)
	}
	if bulkResp.Errors {
		return ErrBulkFailed.WithDetail("one or more bulk items failed")
	}

	return nil
}

// buildSearchBody constructs the Elasticsearch search body from a SearchRequest.
func buildSearchBody(req SearchRequest) []byte {
	body := map[string]any{
		"query": json.RawMessage(req.Query),
	}
	if req.From > 0 {
		body["from"] = req.From
	}
	if req.Size > 0 {
		body["size"] = req.Size
	}

	data, _ := json.Marshal(body)
	return data
}
