package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// tokenContextKey is the context key for storing forwarded tokens.
type tokenContextKey struct{}

// TokenSource provides tokens dynamically for HTTP requests.
// Implementations handle static keys, token forwarding, OAuth2 client_credentials, etc.
type TokenSource interface {
	// Token returns the current token string. Implementations should handle
	// caching and refresh internally.
	Token(ctx context.Context) (string, error)
}

// --- StaticToken ---

// staticToken is a TokenSource that always returns the same token.
type staticToken struct {
	token string
}

// NewStaticToken creates a TokenSource that always returns the given token.
func NewStaticToken(token string) TokenSource {
	return &staticToken{token: token}
}

// Token returns the static token.
func (s *staticToken) Token(_ context.Context) (string, error) {
	return s.token, nil
}

// --- TokenForwarder ---

// tokenForwarder extracts the token from the request context.
// Use this to forward the caller's JWT to downstream services.
type tokenForwarder struct{}

// NewTokenForwarder creates a TokenSource that extracts tokens from context.
func NewTokenForwarder() TokenSource {
	return &tokenForwarder{}
}

// Token extracts the token from context.
func (t *tokenForwarder) Token(ctx context.Context) (string, error) {
	token, ok := TokenFromContext(ctx)
	if !ok {
		return "", fmt.Errorf("no token in context")
	}
	return token, nil
}

// ContextWithToken stores a token in the context for forwarding.
func ContextWithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenContextKey{}, token)
}

// TokenFromContext retrieves a forwarded token from the context.
func TokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(tokenContextKey{}).(string)
	if ok && token == "" {
		return "", false
	}
	return token, ok
}

// --- ClientCredentials ---

// ClientCredentialsConfig holds OAuth2 client_credentials configuration.
type ClientCredentialsConfig struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	ExtraParams  map[string]string
}

// clientCredentials implements TokenSource using OAuth2 client_credentials grant.
// It caches the token and refreshes it before expiry.
type clientCredentials struct {
	cfg         ClientCredentialsConfig
	mu          sync.Mutex
	cachedToken string
	expiresAt   time.Time
	httpClient  *http.Client
}

// NewClientCredentials creates a TokenSource using OAuth2 client_credentials grant.
func NewClientCredentials(cfg ClientCredentialsConfig) TokenSource {
	return &clientCredentials{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Token returns a cached token or fetches a new one if expired.
func (cc *clientCredentials) Token(ctx context.Context) (string, error) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// Return cached token if still valid (with 30s buffer)
	if cc.cachedToken != "" && time.Now().Add(30*time.Second).Before(cc.expiresAt) {
		return cc.cachedToken, nil
	}

	// Fetch new token
	token, expiresIn, err := cc.fetchToken(ctx)
	if err != nil {
		return "", err
	}

	cc.cachedToken = token
	cc.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)

	return token, nil
}

// fetchToken performs the OAuth2 client_credentials grant.
func (cc *clientCredentials) fetchToken(ctx context.Context) (string, int, error) {
	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {cc.cfg.ClientID},
		"client_secret": {cc.cfg.ClientSecret},
	}
	if len(cc.cfg.Scopes) > 0 {
		data.Set("scope", strings.Join(cc.cfg.Scopes, " "))
	}
	for k, v := range cc.cfg.ExtraParams {
		data.Set(k, v)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cc.cfg.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := cc.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", 0, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", 0, fmt.Errorf("decode token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", 0, fmt.Errorf("empty access_token in response")
	}

	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}
