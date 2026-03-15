package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
)

type jwtProvider struct {
	secret []byte
	expiry time.Duration
}

// NewJWTProvider creates a local HMAC-signed JWT provider.
func NewJWTProvider(cfg *config.Config) Provider {
	return &jwtProvider{
		secret: []byte(cfg.JWTSecret),
		expiry: cfg.JWTExpiry,
	}
}

type jwtClaims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (p *jwtProvider) ValidateToken(_ context.Context, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return p.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return &Claims{
		UserID: claims.Subject,
		Email:  claims.Email,
		Role:   claims.Role,
	}, nil
}

func (p *jwtProvider) GenerateToken(userID, email, role string) (string, error) {
	claims := &jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(p.expiry)),
		},
		Email: email,
		Role:  role,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(p.secret)
}

func (p *jwtProvider) RefreshToken(_ context.Context, tokenStr string) (string, error) {
	claims, err := p.ValidateToken(context.Background(), tokenStr)
	if err != nil {
		return "", err
	}
	return p.GenerateToken(claims.UserID, claims.Email, claims.Role)
}
