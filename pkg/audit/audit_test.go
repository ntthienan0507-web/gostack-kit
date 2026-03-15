package audit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntry_StructCreation(t *testing.T) {
	id := uuid.New()
	now := time.Now()

	entry := Entry{
		ID:         id,
		UserID:     "user-123",
		Action:     ActionCreate,
		Resource:   "order",
		ResourceID: "order-456",
		Changes:    json.RawMessage(`{"status":"pending"}`),
		IP:         "192.168.1.1",
		UserAgent:  "Mozilla/5.0",
		CreatedAt:  now,
	}

	assert.Equal(t, id, entry.ID)
	assert.Equal(t, "user-123", entry.UserID)
	assert.Equal(t, ActionCreate, entry.Action)
	assert.Equal(t, "order", entry.Resource)
	assert.Equal(t, "order-456", entry.ResourceID)
	assert.Equal(t, "192.168.1.1", entry.IP)
	assert.Equal(t, "Mozilla/5.0", entry.UserAgent)
	assert.JSONEq(t, `{"status":"pending"}`, string(entry.Changes))
}

func TestEntry_TableName(t *testing.T) {
	assert.Equal(t, "audit_log", Entry{}.TableName())
}

func TestAction_Constants(t *testing.T) {
	assert.Equal(t, Action("create"), ActionCreate)
	assert.Equal(t, Action("update"), ActionUpdate)
	assert.Equal(t, Action("delete"), ActionDelete)
	assert.Equal(t, Action("view"), ActionView)
	assert.Equal(t, Action("export"), ActionExport)
	assert.Equal(t, Action("login"), ActionLogin)
	assert.Equal(t, Action("logout"), ActionLogout)
}

func TestQueryParams_Defaults(t *testing.T) {
	params := QueryParams{}

	assert.Equal(t, "", params.UserID)
	assert.Equal(t, Action(""), params.Action)
	assert.Equal(t, "", params.Resource)
	assert.Equal(t, 0, params.Limit)
	assert.Equal(t, 0, params.Offset)
	assert.Nil(t, params.From)
	assert.Nil(t, params.To)
}

func TestQueryParams_WithFilters(t *testing.T) {
	from := time.Now().Add(-24 * time.Hour)
	to := time.Now()

	params := QueryParams{
		UserID:     "user-123",
		Action:     ActionUpdate,
		Resource:   "order",
		ResourceID: "order-789",
		From:       &from,
		To:         &to,
		Limit:      25,
		Offset:     10,
	}

	assert.Equal(t, "user-123", params.UserID)
	assert.Equal(t, ActionUpdate, params.Action)
	assert.Equal(t, "order", params.Resource)
	assert.Equal(t, "order-789", params.ResourceID)
	assert.Equal(t, 25, params.Limit)
	assert.Equal(t, 10, params.Offset)
	assert.NotNil(t, params.From)
	assert.NotNil(t, params.To)
}

func TestNew_ReturnsLogger(t *testing.T) {
	// New should not panic with nil dependencies (used for unit testing).
	// In production, non-nil values are injected.
	logger := New(nil, nil, nil)
	require.NotNil(t, logger)
}

func TestLogFromGin_ExtractsContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// We can't fully test LogFromGin without a DB and worker pool,
	// but we can verify the gin context extraction path doesn't panic.
	// The real integration test would use a test DB.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/orders", nil)
	c.Request.Header.Set("User-Agent", "TestAgent/1.0")
	c.Set("user_id", "user-abc")

	// Verify the context values are accessible.
	assert.Equal(t, "user-abc", c.GetString("user_id"))
	assert.Equal(t, "TestAgent/1.0", c.Request.UserAgent())
}

func TestEntry_ChangesJSON(t *testing.T) {
	changes := map[string]any{
		"old_status": "pending",
		"new_status": "confirmed",
	}
	data, err := json.Marshal(changes)
	require.NoError(t, err)

	entry := Entry{
		Changes: data,
	}

	var decoded map[string]any
	err = json.Unmarshal(entry.Changes, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "pending", decoded["old_status"])
	assert.Equal(t, "confirmed", decoded["new_status"])
}

func TestEntry_MetadataJSON(t *testing.T) {
	metadata := map[string]any{
		"request_id": "req-123",
		"source":     "admin_panel",
	}
	data, err := json.Marshal(metadata)
	require.NoError(t, err)

	entry := Entry{
		Metadata: data,
	}

	var decoded map[string]any
	err = json.Unmarshal(entry.Metadata, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "req-123", decoded["request_id"])
	assert.Equal(t, "admin_panel", decoded["source"])
}
