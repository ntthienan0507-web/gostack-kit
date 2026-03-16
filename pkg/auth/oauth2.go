package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
)

type oauth2Provider struct {
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
	provider     *oidc.Provider
}

// NewOAuth2Provider creates an OAuth2/OIDC provider that validates ID tokens
// via OIDC discovery from the issuer URL.
func NewOAuth2Provider(ctx context.Context, cfg *config.Config) (Provider, error) {
	provider, err := oidc.NewProvider(ctx, cfg.OAuth2IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery from %s: %w", cfg.OAuth2IssuerURL, err)
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.OAuth2ClientID,
	})

	oauth2Config := &oauth2.Config{
		ClientID:     cfg.OAuth2ClientID,
		ClientSecret: cfg.OAuth2ClientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	return &oauth2Provider{
		verifier:     verifier,
		oauth2Config: oauth2Config,
		provider:     provider,
	}, nil
}

func (p *oauth2Provider) ValidateToken(ctx context.Context, tokenStr string) (*Claims, error) {
	idToken, err := p.verifier.Verify(ctx, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("oauth2 verify token: %w", err)
	}

	var tokenClaims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Role  string `json:"role"`
	}
	if err := idToken.Claims(&tokenClaims); err != nil {
		return nil, fmt.Errorf("oauth2 parse claims: %w", err)
	}

	return &Claims{
		UserID: idToken.Subject,
		Email:  tokenClaims.Email,
		Role:   tokenClaims.Role,
	}, nil
}

func (p *oauth2Provider) GenerateToken(_, _, _ string) (string, error) {
	return "", fmt.Errorf("token generation not supported with OAuth2 provider; use OAuth2 authorization flow")
}

func (p *oauth2Provider) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	tokenSource := p.oauth2Config.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	newToken, err := tokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("oauth2 refresh: %w", err)
	}

	idToken, ok := newToken.Extra("id_token").(string)
	if !ok || idToken == "" {
		return newToken.AccessToken, nil
	}
	return idToken, nil
}
