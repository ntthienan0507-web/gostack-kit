package elasticsearch

import (
	"net/http"

	"github.com/ntthienan0507-web/go-api-template/pkg/apperror"
)

// Sentinel errors for Elasticsearch operations.
var (
	ErrIndexFailed  = apperror.New(http.StatusBadGateway, "elasticsearch.index_failed", "Failed to index document")
	ErrSearchFailed = apperror.New(http.StatusBadGateway, "elasticsearch.search_failed", "Failed to execute search query")
	ErrDeleteFailed = apperror.New(http.StatusBadGateway, "elasticsearch.delete_failed", "Failed to delete document")
	ErrBulkFailed   = apperror.New(http.StatusBadGateway, "elasticsearch.bulk_failed", "Bulk operation failed")
	ErrConnFailed   = apperror.New(http.StatusBadGateway, "elasticsearch.conn_failed", "Failed to connect to Elasticsearch")
)
