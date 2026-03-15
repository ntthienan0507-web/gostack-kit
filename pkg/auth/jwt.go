package auth

import (
	"context"
	"crypto/rsa"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
)

type jwtProvider struct {
	algorithm  string
	secret     []byte
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	keyID      string
	expiry     time.Duration
}

// NewJWTProvider creates a JWT provider supporting HS256 (default) or RS256.
func NewJWTProvider(cfg *config.Config) (Provider, error) {
	p := &jwtProvider{
		algorithm: cfg.JWTAlgorithm,
		expiry:    cfg.JWTExpiry,
		keyID:     cfg.JWTKeyID,
	}

	switch cfg.JWTAlgorithm {
	case "HS256", "":
		if cfg.JWTSecret == "" {
			return nil, fmt.Errorf("JWT_SECRET is required for HS256")
		}
		p.algorithm = "HS256"
		p.secret = []byte(cfg.JWTSecret)

	case "RS256":
		if cfg.JWTPrivateKeyFile == "" || cfg.JWTPublicKeyFile == "" {
			return nil, fmt.Errorf("JWT_PRIVATE_KEY_FILE and JWT_PUBLIC_KEY_FILE are required for RS256")
		}
		privPEM, err := os.ReadFile(cfg.JWTPrivateKeyFile)
		if err != nil {
			return nil, fmt.Errorf("read private key %s: %w", cfg.JWTPrivateKeyFile, err)
		}
		p.privateKey, err = jwt.ParseRSAPrivateKeyFromPEM(privPEM)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}

		pubPEM, err := os.ReadFile(cfg.JWTPublicKeyFile)
		if err != nil {
			return nil, fmt.Errorf("read public key %s: %w", cfg.JWTPublicKeyFile, err)
		}
		p.publicKey, err = jwt.ParseRSAPublicKeyFromPEM(pubPEM)
		if err != nil {
			return nil, fmt.Errorf("parse public key: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported JWT_ALGORITHM %q: must be HS256 or RS256", cfg.JWTAlgorithm)
	}

	return p, nil
}

type jwtClaims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
	Role  string `json:"role"`
}

func (p *jwtProvider) ValidateToken(_ context.Context, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (any, error) {
		switch p.algorithm {
		case "RS256":
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return p.publicKey, nil
		default:
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return p.secret, nil
		}
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

	var token *jwt.Token
	switch p.algorithm {
	case "RS256":
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		if p.keyID != "" {
			token.Header["kid"] = p.keyID
		}
		return token.SignedString(p.privateKey)
	default:
		token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		if p.keyID != "" {
			token.Header["kid"] = p.keyID
		}
		return token.SignedString(p.secret)
	}
}

func (p *jwtProvider) RefreshToken(_ context.Context, tokenStr string) (string, error) {
	claims, err := p.ValidateToken(context.Background(), tokenStr)
	if err != nil {
		return "", err
	}
	return p.GenerateToken(claims.UserID, claims.Email, claims.Role)
}
