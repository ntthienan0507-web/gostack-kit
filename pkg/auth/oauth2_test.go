package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
)

func newFakeOIDCServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var serverURL string

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 serverURL,
			"authorization_endpoint": serverURL + "/authorize",
			"token_endpoint":         serverURL + "/token",
			"userinfo_endpoint":      serverURL + "/userinfo",
			"jwks_uri":               serverURL + "/jwks",
		})
	})

	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"keys":[]}`))
	})

	srv := httptest.NewServer(mux)
	serverURL = srv.URL
	t.Cleanup(srv.Close)
	return srv
}

func TestOAuth2_NewProvider_Success(t *testing.T) {
	srv := newFakeOIDCServer(t)

	p, err := NewOAuth2Provider(context.Background(), &config.Config{
		OAuth2IssuerURL:    srv.URL,
		OAuth2ClientID:     "test-client",
		OAuth2ClientSecret: "test-secret",
	})

	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestOAuth2_NewProvider_BadIssuer(t *testing.T) {
	_, err := NewOAuth2Provider(context.Background(), &config.Config{
		OAuth2IssuerURL: "http://127.0.0.1:1/nonexistent",
		OAuth2ClientID:  "test-client",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "oidc discovery")
}

func TestOAuth2_GenerateToken_Unsupported(t *testing.T) {
	srv := newFakeOIDCServer(t)

	p, err := NewOAuth2Provider(context.Background(), &config.Config{
		OAuth2IssuerURL:    srv.URL,
		OAuth2ClientID:     "test-client",
		OAuth2ClientSecret: "test-secret",
	})
	require.NoError(t, err)

	token, err := p.GenerateToken("user-1", "a@b.com", "admin")

	assert.Empty(t, token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestOAuth2_ValidateToken_InvalidToken(t *testing.T) {
	srv := newFakeOIDCServer(t)

	p, err := NewOAuth2Provider(context.Background(), &config.Config{
		OAuth2IssuerURL:    srv.URL,
		OAuth2ClientID:     "test-client",
		OAuth2ClientSecret: "test-secret",
	})
	require.NoError(t, err)

	claims, err := p.ValidateToken(context.Background(), "invalid-token")

	assert.Nil(t, claims)
	assert.Error(t, err)
}
