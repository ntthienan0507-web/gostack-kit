package grpcserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ntthienan0507-web/gostack-kit/pkg/auth"
)

type mockAuthProvider struct {
	claims *auth.Claims
	err    error
}

func (m *mockAuthProvider) ValidateToken(_ context.Context, _ string) (*auth.Claims, error) {
	return m.claims, m.err
}
func (m *mockAuthProvider) GenerateToken(_, _, _ string) (string, error) { return "", nil }
func (m *mockAuthProvider) RefreshToken(_ context.Context, _ string) (string, error) {
	return "", nil
}

var testInfo = &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
var noopHandler = func(_ context.Context, _ any) (any, error) { return "ok", nil }

func TestRecoveryInterceptor_NoPanic(t *testing.T) {
	interceptor := RecoveryInterceptor(zap.NewNop())
	resp, err := interceptor(context.Background(), nil, testInfo, noopHandler)

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestRecoveryInterceptor_CatchesPanic(t *testing.T) {
	interceptor := RecoveryInterceptor(zap.NewNop())
	panicHandler := func(_ context.Context, _ any) (any, error) {
		panic("test panic")
	}

	resp, err := interceptor(context.Background(), nil, testInfo, panicHandler)

	assert.Nil(t, resp)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestLoggingInterceptor(t *testing.T) {
	interceptor := LoggingInterceptor(zap.NewNop())
	resp, err := interceptor(context.Background(), nil, testInfo, noopHandler)

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestAuthInterceptor_ValidToken(t *testing.T) {
	provider := &mockAuthProvider{
		claims: &auth.Claims{UserID: "user-1", Email: "a@b.com", Role: "admin"},
	}
	interceptor := AuthInterceptor(provider)

	md := metadata.Pairs("authorization", "Bearer valid-token")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	handler := func(ctx context.Context, req any) (any, error) {
		claims, ok := ClaimsFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, "user-1", claims.UserID)
		return "ok", nil
	}

	resp, err := interceptor(ctx, nil, testInfo, handler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestAuthInterceptor_MissingToken(t *testing.T) {
	provider := &mockAuthProvider{}
	interceptor := AuthInterceptor(provider)

	_, err := interceptor(context.Background(), nil, testInfo, noopHandler)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestAuthInterceptor_SkipMethod(t *testing.T) {
	provider := &mockAuthProvider{}
	interceptor := AuthInterceptor(provider, "/test.Service/Method")

	resp, err := interceptor(context.Background(), nil, testInfo, noopHandler)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestAuthInterceptor_InvalidFormat(t *testing.T) {
	provider := &mockAuthProvider{}
	interceptor := AuthInterceptor(provider)

	md := metadata.Pairs("authorization", "Basic abc123")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, nil, testInfo, noopHandler)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestRequireRoleInterceptor_Allowed(t *testing.T) {
	interceptor := RequireRoleInterceptor("admin")

	ctx := context.WithValue(context.Background(), claimsKey{}, &auth.Claims{Role: "admin"})
	resp, err := interceptor(ctx, nil, testInfo, noopHandler)

	require.NoError(t, err)
	assert.Equal(t, "ok", resp)
}

func TestRequireRoleInterceptor_Denied(t *testing.T) {
	interceptor := RequireRoleInterceptor("admin")

	ctx := context.WithValue(context.Background(), claimsKey{}, &auth.Claims{Role: "user"})
	_, err := interceptor(ctx, nil, testInfo, noopHandler)

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestRequireRoleInterceptor_NoClaims(t *testing.T) {
	interceptor := RequireRoleInterceptor("admin")

	_, err := interceptor(context.Background(), nil, testInfo, noopHandler)

	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}
