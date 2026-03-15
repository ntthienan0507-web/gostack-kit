package auth

import (
	"context"
	"fmt"

	"github.com/Nerzal/gocloak/v13"

	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
)

type keycloakProvider struct {
	client       *gocloak.GoCloak
	realm        string
	clientID     string
	clientSecret string
}

// NewKeycloakProvider creates a Keycloak OIDC token validator.
func NewKeycloakProvider(cfg *config.Config) Provider {
	return &keycloakProvider{
		client:       gocloak.NewClient(cfg.KeycloakHost),
		realm:        cfg.KeycloakRealm,
		clientID:     cfg.KeycloakClientID,
		clientSecret: cfg.KeycloakClientSecret,
	}
}

func (p *keycloakProvider) ValidateToken(ctx context.Context, tokenStr string) (*Claims, error) {
	_, mapClaims, err := p.client.DecodeAccessToken(ctx, tokenStr, p.realm)
	if err != nil {
		return nil, fmt.Errorf("keycloak decode token: %w", err)
	}

	data := *mapClaims
	sub, _ := data["sub"].(string)
	email, _ := data["email"].(string)

	var role string
	if ra, ok := data["realm_access"].(map[string]interface{}); ok {
		if roles, ok := ra["roles"].([]interface{}); ok && len(roles) > 0 {
			role, _ = roles[0].(string)
		}
	}

	return &Claims{
		UserID:   sub,
		Email:    email,
		Role:     role,
		Metadata: data,
	}, nil
}

func (p *keycloakProvider) GenerateToken(_, _, _ string) (string, error) {
	return "", fmt.Errorf("token generation not supported with keycloak provider; use keycloak login flow")
}

func (p *keycloakProvider) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	jwt, err := p.client.RefreshToken(ctx, refreshToken, p.clientID, p.clientSecret, p.realm)
	if err != nil {
		return "", fmt.Errorf("keycloak refresh: %w", err)
	}
	return jwt.AccessToken, nil
}
