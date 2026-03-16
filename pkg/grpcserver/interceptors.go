package grpcserver

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/ntthienan0507-web/gostack-kit/pkg/auth"
)

type claimsKey struct{}

// ClaimsFromContext extracts auth Claims set by the auth interceptor.
func ClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	c, ok := ctx.Value(claimsKey{}).(*auth.Claims)
	return c, ok
}

// RecoveryInterceptor catches panics and returns Internal error.
func RecoveryInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("grpc panic recovered",
					zap.Any("error", r),
					zap.String("method", info.FullMethod),
				)
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}

// LoggingInterceptor logs each gRPC call with structured zap logging.
func LoggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)

		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		logger.Info("grpc request",
			zap.String("method", info.FullMethod),
			zap.String("code", code.String()),
			zap.Duration("latency", time.Since(start)),
		)
		return resp, err
	}
}

// AuthInterceptor validates bearer tokens from gRPC metadata.
// Skips methods listed in skipMethods (e.g. health checks).
func AuthInterceptor(provider auth.Provider, skipMethods ...string) grpc.UnaryServerInterceptor {
	skip := make(map[string]struct{}, len(skipMethods))
	for _, m := range skipMethods {
		skip[m] = struct{}{}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, ok := skip[info.FullMethod]; ok {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		token := strings.TrimPrefix(authHeader[0], "Bearer ")
		if token == authHeader[0] {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization format")
		}

		claims, err := provider.ValidateToken(ctx, token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		ctx = context.WithValue(ctx, claimsKey{}, claims)
		return handler(ctx, req)
	}
}

// RequireRoleInterceptor checks that the caller has one of the allowed roles.
// Must be chained after AuthInterceptor.
func RequireRoleInterceptor(roles ...string) grpc.UnaryServerInterceptor {
	roleSet := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		roleSet[r] = struct{}{}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		claims, ok := ClaimsFromContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "no claims in context")
		}
		if _, allowed := roleSet[claims.Role]; !allowed {
			return nil, status.Error(codes.PermissionDenied, "insufficient role")
		}
		return handler(ctx, req)
	}
}

// StreamRecoveryInterceptor catches panics in streaming RPCs.
func StreamRecoveryInterceptor(logger *zap.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("grpc stream panic recovered",
					zap.Any("error", r),
					zap.String("method", info.FullMethod),
				)
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(srv, ss)
	}
}

// StreamLoggingInterceptor logs streaming RPCs.
func StreamLoggingInterceptor(logger *zap.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)

		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		logger.Info("grpc stream",
			zap.String("method", info.FullMethod),
			zap.String("code", code.String()),
			zap.Duration("latency", time.Since(start)),
		)
		return err
	}
}

// StreamAuthInterceptor validates bearer tokens for streaming RPCs.
func StreamAuthInterceptor(provider auth.Provider, skipMethods ...string) grpc.StreamServerInterceptor {
	skip := make(map[string]struct{}, len(skipMethods))
	for _, m := range skipMethods {
		skip[m] = struct{}{}
	}

	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if _, ok := skip[info.FullMethod]; ok {
			return handler(srv, ss)
		}

		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok {
			return status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return status.Error(codes.Unauthenticated, "missing authorization header")
		}

		token := strings.TrimPrefix(authHeader[0], "Bearer ")
		if token == authHeader[0] {
			return status.Error(codes.Unauthenticated, "invalid authorization format")
		}

		claims, err := provider.ValidateToken(ss.Context(), token)
		if err != nil {
			return status.Error(codes.Unauthenticated, "invalid token")
		}

		wrapped := &authenticatedStream{
			ServerStream: ss,
			ctx:          context.WithValue(ss.Context(), claimsKey{}, claims),
		}
		return handler(srv, wrapped)
	}
}

type authenticatedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authenticatedStream) Context() context.Context { return s.ctx }
