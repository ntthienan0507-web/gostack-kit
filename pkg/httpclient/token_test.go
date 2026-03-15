package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- StaticToken ---

func TestStaticToken(t *testing.T) {
	ts := NewStaticToken("my-api-key")

	token, err := ts.Token(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "my-api-key", token)
}

func TestStaticToken_AlwaysSameValue(t *testing.T) {
	ts := NewStaticToken("fixed")

	t1, _ := ts.Token(context.Background())
	t2, _ := ts.Token(context.Background())

	assert.Equal(t, t1, t2)
}

// --- TokenForwarder ---

func TestTokenForwarder_Success(t *testing.T) {
	ctx := ContextWithToken(context.Background(), "user-jwt-token")
	ts := NewTokenForwarder()

	token, err := ts.Token(ctx)

	require.NoError(t, err)
	assert.Equal(t, "user-jwt-token", token)
}

func TestTokenForwarder_NoTokenInContext(t *testing.T) {
	ts := NewTokenForwarder()

	_, err := ts.Token(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no token in context")
}

func TestContextWithToken_RoundTrip(t *testing.T) {
	ctx := ContextWithToken(context.Background(), "abc123")

	token, ok := TokenFromContext(ctx)

	assert.True(t, ok)
	assert.Equal(t, "abc123", token)
}

func TestTokenFromContext_EmptyString(t *testing.T) {
	ctx := ContextWithToken(context.Background(), "")

	_, ok := TokenFromContext(ctx)

	assert.False(t, ok)
}

// --- ClientCredentials ---

func TestClientCredentials_FetchesToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		assert.Equal(t, http.MethodPost, r.Method)

		r.ParseForm()
		assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		assert.Equal(t, "my-client", r.Form.Get("client_id"))
		assert.Equal(t, "my-secret", r.Form.Get("client_secret"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "fresh-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	cc := NewClientCredentials(ClientCredentialsConfig{
		TokenURL:     srv.URL + "/oauth/token",
		ClientID:     "my-client",
		ClientSecret: "my-secret",
	})

	token, err := cc.Token(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "fresh-token", token)
}

func TestClientCredentials_CachesToken(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "cached-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	cc := NewClientCredentials(ClientCredentialsConfig{
		TokenURL:     srv.URL + "/oauth/token",
		ClientID:     "c",
		ClientSecret: "s",
	})

	// Call Token() 5 times — should only hit the server once
	for i := 0; i < 5; i++ {
		token, err := cc.Token(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "cached-token", token)
	}

	assert.Equal(t, int32(1), calls.Load())
}

func TestClientCredentials_RefreshesExpiredToken(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "token-" + fmt.Sprint(n),
			"token_type":   "Bearer",
			"expires_in":   1, // expires in 1 second
		})
	}))
	defer srv.Close()

	cc := NewClientCredentials(ClientCredentialsConfig{
		TokenURL:     srv.URL + "/oauth/token",
		ClientID:     "c",
		ClientSecret: "s",
	})

	// First call
	token1, err := cc.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "token-1", token1)

	// Force expiry (token expires_in=1s, buffer=30s, so it's already "expired")
	// The 30s buffer means expires_in=1 is immediately considered expired
	token2, err := cc.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "token-2", token2)
}

func TestClientCredentials_WithScopes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		assert.Equal(t, "read write", r.Form.Get("scope"))

		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "scoped-token",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	cc := NewClientCredentials(ClientCredentialsConfig{
		TokenURL:     srv.URL + "/oauth/token",
		ClientID:     "c",
		ClientSecret: "s",
		Scopes:       []string{"read", "write"},
	})

	token, err := cc.Token(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "scoped-token", token)
}

func TestClientCredentials_WithExtraParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		assert.Equal(t, "https://api.example.com", r.Form.Get("audience"))

		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "audience-token",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	cc := NewClientCredentials(ClientCredentialsConfig{
		TokenURL:     srv.URL + "/oauth/token",
		ClientID:     "c",
		ClientSecret: "s",
		ExtraParams:  map[string]string{"audience": "https://api.example.com"},
	})

	token, err := cc.Token(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "audience-token", token)
}

func TestClientCredentials_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error": "invalid_client"}`))
	}))
	defer srv.Close()

	cc := NewClientCredentials(ClientCredentialsConfig{
		TokenURL:     srv.URL + "/oauth/token",
		ClientID:     "bad",
		ClientSecret: "creds",
	})

	_, err := cc.Token(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

func TestClientCredentials_EmptyAccessToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	cc := NewClientCredentials(ClientCredentialsConfig{
		TokenURL:     srv.URL + "/oauth/token",
		ClientID:     "c",
		ClientSecret: "s",
	})

	_, err := cc.Token(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty access_token")
}

func TestClientCredentials_ThreadSafe(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(10 * time.Millisecond) // simulate latency
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "concurrent-token",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	cc := NewClientCredentials(ClientCredentialsConfig{
		TokenURL:     srv.URL + "/oauth/token",
		ClientID:     "c",
		ClientSecret: "s",
	})

	// Launch 20 goroutines simultaneously
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := cc.Token(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, "concurrent-token", token)
		}()
	}
	wg.Wait()

	// Mutex ensures only a few actual HTTP calls (not 20)
	assert.LessOrEqual(t, calls.Load(), int32(3))
}

// --- Integration: Client with TokenSource ---

func TestClient_WithTokenSource(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := testClient(srv.URL, func(cfg *Config) {
		cfg.TokenSource = NewStaticToken("dynamic-token")
	})
	_, err := c.Get(context.Background(), "/test")

	require.NoError(t, err)
	assert.Equal(t, "Bearer dynamic-token", gotAuth)
}

func TestClient_TokenSourceOverridesAPIKey(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := testClient(srv.URL, func(cfg *Config) {
		cfg.APIKey = "should-be-ignored"
		cfg.TokenSource = NewStaticToken("from-token-source")
	})
	_, err := c.Get(context.Background(), "/test")

	require.NoError(t, err)
	assert.Equal(t, "Bearer from-token-source", gotAuth)
}

func TestClient_WithTokenForwarder(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := testClient(srv.URL, func(cfg *Config) {
		cfg.TokenSource = NewTokenForwarder()
	})

	// Simulate middleware storing user's token in context
	ctx := ContextWithToken(context.Background(), "user-bearer-token")
	_, err := c.Get(ctx, "/downstream")

	require.NoError(t, err)
	assert.Equal(t, "Bearer user-bearer-token", gotAuth)
}

func TestClient_NoAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := testClient(srv.URL) // no APIKey, no TokenSource
	_, err := c.Get(context.Background(), "/public")

	require.NoError(t, err)
	assert.Empty(t, gotAuth)
}
