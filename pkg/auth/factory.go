package auth

import (
	"fmt"

	"github.com/ntthienan0507-web/go-api-template/pkg/config"
)

// NewProvider selects the auth provider based on AUTH_PROVIDER config.
func NewProvider(cfg *config.Config) (Provider, error) {
	switch cfg.AuthProvider {
	case "jwt":
		return NewJWTProvider(cfg), nil
	case "keycloak":
		return NewKeycloakProvider(cfg), nil
	default:
		return nil, fmt.Errorf("unknown AUTH_PROVIDER %q: must be jwt or keycloak", cfg.AuthProvider)
	}
}
