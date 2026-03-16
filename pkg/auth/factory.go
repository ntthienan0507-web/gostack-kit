package auth

import (
	"context"
	"fmt"

	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
)

// NewProvider selects the auth provider based on AUTH_PROVIDER config.
func NewProvider(ctx context.Context, cfg *config.Config) (Provider, error) {
	switch cfg.AuthProvider {
	case "jwt":
		return NewJWTProvider(cfg)
	case "keycloak":
		return NewKeycloakProvider(cfg), nil
	case "oauth2":
		return NewOAuth2Provider(ctx, cfg)
	case "saml":
		return NewSAMLProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown AUTH_PROVIDER %q: must be jwt, keycloak, oauth2, or saml", cfg.AuthProvider)
	}
}
