package httpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ntthienan0507-web/gostack-kit/pkg/retry"
)

func testClient(url string, opts ...func(*Config)) *Client {
	cfg := Config{BaseURL: url, Timeout: 5 * time.Second}
	for _, opt := range opts {
		opt(&cfg)
	}
	return New(cfg, zap.NewNop())
}

// --- Auth headers ---

func TestClient_SetsAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := testClient(srv.URL, func(cfg *Config) { cfg.APIKey = "secret-key" })
	_, err := c.Get(context.Background(), "/test")

	require.NoError(t, err)
	assert.Equal(t, "Bearer secret-key", gotAuth)
}

func TestClient_DoesNotOverrideExplicitAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := testClient(srv.URL, func(cfg *Config) { cfg.APIKey = "default-key" })

	req, _ := http.NewRequest("GET", srv.URL+"/test", nil)
	req.Header.Set("Authorization", "Bearer override-key")
	_, err := c.Do(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "Bearer override-key", gotAuth)
}

func TestClient_CustomHeaders(t *testing.T) {
	var gotTenant string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenant = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := testClient(srv.URL, func(cfg *Config) {
		cfg.Headers = map[string]string{"X-Tenant-ID": "tenant-42"}
	})
	_, err := c.Get(context.Background(), "/test")

	require.NoError(t, err)
	assert.Equal(t, "tenant-42", gotTenant)
}

// --- GET ---

func TestClient_Get_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/users", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"name": "john"})
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	resp, err := c.Get(context.Background(), "/api/users")

	require.NoError(t, err)

	var data map[string]string
	err = DecodeJSON(resp, &data)
	require.NoError(t, err)
	assert.Equal(t, "john", data["name"])
}

// --- POST ---

func TestClient_Post_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "john", body["name"])

		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]string{"id": "123"})
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	resp, err := c.Post(context.Background(), "/api/users", map[string]string{"name": "john"})

	require.NoError(t, err)
	assert.Equal(t, 201, resp.StatusCode)

	var data map[string]string
	err = DecodeJSON(resp, &data)
	require.NoError(t, err)
	assert.Equal(t, "123", data["id"])
}

// --- PUT ---

func TestClient_Put_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	resp, err := c.Put(context.Background(), "/api/users/1", map[string]string{"name": "jane"})

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

// --- DELETE ---

func TestClient_Delete_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(204)
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	resp, err := c.Delete(context.Background(), "/api/users/1")

	require.NoError(t, err)
	assert.Equal(t, 204, resp.StatusCode)
	resp.Body.Close()
}

// --- DecodeJSON error on non-2xx ---

func TestDecodeJSON_ErrorOn4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error": "not found"}`))
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	resp, err := c.Get(context.Background(), "/missing")
	require.NoError(t, err)

	var data map[string]string
	err = DecodeJSON(resp, &data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "upstream returned 404")
}

// --- Retry on 503 ---

func TestClient_RetriesOn503(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer srv.Close()

	c := testClient(srv.URL, func(cfg *Config) {
		cfg.Retry = &retry.Config{
			MaxRetries: 3,
			BaseDelay:  1 * time.Millisecond,
			MaxDelay:   10 * time.Millisecond,
			Multiplier: 1.0,
		}
	})

	resp, err := c.Get(context.Background(), "/flaky")

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 3, attempts)
	resp.Body.Close()
}

// --- Context cancellation ---

func TestClient_RespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := c.Get(ctx, "/slow")
	assert.Error(t, err)
}

// --- Redirect strips auth header on cross-origin ---

func TestClient_StripsAuthOnCrossOriginRedirect(t *testing.T) {
	var gotAuth string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer target.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/redirected", http.StatusTemporaryRedirect)
	}))
	defer origin.Close()

	c := testClient(origin.URL, func(cfg *Config) { cfg.APIKey = "secret" })
	resp, err := c.Get(context.Background(), "/start")

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Empty(t, gotAuth, "auth header should be stripped on cross-origin redirect")
	resp.Body.Close()
}

// --- URL building ---

func TestClient_URLBuilding(t *testing.T) {
	tests := []struct {
		base     string
		path     string
		expected string
	}{
		{"https://api.example.com", "/users", "https://api.example.com/users"},
		{"https://api.example.com/", "/users", "https://api.example.com/users"},
		{"https://api.example.com", "users", "https://api.example.com/users"},
		{"https://api.example.com/v1", "/users", "https://api.example.com/v1/users"},
	}

	for _, tt := range tests {
		c := &Client{baseURL: strings.TrimRight(tt.base, "/")}
		assert.Equal(t, tt.expected, c.url(tt.path))
	}
}
