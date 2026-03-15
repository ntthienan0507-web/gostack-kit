package auth

import "context"

// Claims holds parsed token data passed downstream to handlers via context.
type Claims struct {
	UserID   string
	Email    string
	Role     string
	Metadata map[string]interface{}
}

// Provider is the pluggable auth interface.
// Switch implementations via AUTH_PROVIDER config (jwt | keycloak).
type Provider interface {
	// ValidateToken parses and validates a bearer token, returns Claims.
	ValidateToken(ctx context.Context, token string) (*Claims, error)

	// GenerateToken issues a new token (JWT local only).
	GenerateToken(userID, email, role string) (string, error)

	// RefreshToken issues a new token from a valid existing token.
	RefreshToken(ctx context.Context, token string) (string, error)
}
