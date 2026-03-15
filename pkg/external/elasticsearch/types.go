package elasticsearch

import "encoding/json"

// IndexRequest represents a request to index a document.
type IndexRequest struct {
	Index string `json:"index"`
	ID    string `json:"id,omitempty"`
	Body  any    `json:"body"`
}

// SearchRequest represents a search query against an index.
type SearchRequest struct {
	Index string          `json:"index"`
	Query json.RawMessage `json:"query"`
	From  int             `json:"from,omitempty"`
	Size  int             `json:"size,omitempty"`
}

// SearchResult represents the result of a search query.
type SearchResult struct {
	Total int               `json:"total"`
	Hits  []json.RawMessage `json:"hits"`
}

// BulkItem represents a single item in a bulk operation.
type BulkItem struct {
	Action string `json:"action"` // "index", "create", "update", "delete"
	Index  string `json:"index"`
	ID     string `json:"id,omitempty"`
	Body   any    `json:"body,omitempty"`
}
