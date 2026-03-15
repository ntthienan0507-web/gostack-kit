package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ntthienan0507-web/go-api-template/pkg/config"
)

func newTestJWTProvider() Provider {
	return NewJWTProvider(&config.Config{
		JWTSecret: "test-secret-key-for-testing",
		JWTExpiry: time.Hour,
	})
}

func TestJWT_GenerateToken(t *testing.T) {
	p := newTestJWTProvider()

	token, err := p.GenerateToken("user-123", "user@example.com", "admin")

	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestJWT_ValidateToken_Success(t *testing.T) {
	p := newTestJWTProvider()
	ctx := context.Background()

	token, err := p.GenerateToken("user-123", "user@example.com", "admin")
	require.NoError(t, err)

	claims, err := p.ValidateToken(ctx, token)

	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.UserID)
	assert.Equal(t, "user@example.com", claims.Email)
	assert.Equal(t, "admin", claims.Role)
}

func TestJWT_ValidateToken_InvalidToken(t *testing.T) {
	p := newTestJWTProvider()
	ctx := context.Background()

	claims, err := p.ValidateToken(ctx, "invalid.token.here")

	assert.Nil(t, claims)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestJWT_ValidateToken_WrongSecret(t *testing.T) {
	p1 := newTestJWTProvider()
	p2 := NewJWTProvider(&config.Config{
		JWTSecret: "different-secret",
		JWTExpiry: time.Hour,
	})
	ctx := context.Background()

	token, err := p1.GenerateToken("user-123", "user@example.com", "admin")
	require.NoError(t, err)

	claims, err := p2.ValidateToken(ctx, token)

	assert.Nil(t, claims)
	assert.Error(t, err)
}

func TestJWT_ValidateToken_Expired(t *testing.T) {
	p := NewJWTProvider(&config.Config{
		JWTSecret: "test-secret",
		JWTExpiry: -time.Hour, // already expired
	})
	ctx := context.Background()

	token, err := p.GenerateToken("user-123", "user@example.com", "admin")
	require.NoError(t, err)

	claims, err := p.ValidateToken(ctx, token)

	assert.Nil(t, claims)
	assert.Error(t, err)
}

func TestJWT_RefreshToken_Success(t *testing.T) {
	p := newTestJWTProvider()
	ctx := context.Background()

	original, err := p.GenerateToken("user-123", "user@example.com", "admin")
	require.NoError(t, err)

	refreshed, err := p.RefreshToken(ctx, original)

	require.NoError(t, err)
	assert.NotEmpty(t, refreshed)

	// Verify refreshed token has same claims
	claims, err := p.ValidateToken(ctx, refreshed)
	require.NoError(t, err)
	assert.Equal(t, "user-123", claims.UserID)
	assert.Equal(t, "user@example.com", claims.Email)
	assert.Equal(t, "admin", claims.Role)
}

func TestJWT_RefreshToken_InvalidToken(t *testing.T) {
	p := newTestJWTProvider()
	ctx := context.Background()

	_, err := p.RefreshToken(ctx, "bad-token")

	assert.Error(t, err)
}

func TestJWT_GenerateToken_DifferentUsersGetDifferentTokens(t *testing.T) {
	p := newTestJWTProvider()

	t1, err := p.GenerateToken("user-1", "a@b.com", "user")
	require.NoError(t, err)
	t2, err := p.GenerateToken("user-2", "c@d.com", "admin")
	require.NoError(t, err)

	assert.NotEqual(t, t1, t2)
}
