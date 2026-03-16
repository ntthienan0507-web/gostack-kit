package grpcserver

import (
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/ntthienan0507-web/gostack-kit/pkg/auth"
	"github.com/ntthienan0507-web/gostack-kit/pkg/config"
)

// Server wraps a gRPC server with its listener.
type Server struct {
	GRPCServer *grpc.Server
	Health     *health.Server
	listener   net.Listener
	logger     *zap.Logger
}

// New creates a gRPC server with standard interceptors (recovery, logging, auth).
func New(cfg *config.Config, logger *zap.Logger, authProvider auth.Provider) (*Server, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return nil, fmt.Errorf("grpc listen :%d: %w", cfg.GRPCPort, err)
	}

	healthSkip := []string{"/grpc.health.v1.Health/Check", "/grpc.health.v1.Health/Watch"}

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			RecoveryInterceptor(logger),
			LoggingInterceptor(logger),
			AuthInterceptor(authProvider, healthSkip...),
		),
		grpc.ChainStreamInterceptor(
			StreamRecoveryInterceptor(logger),
			StreamLoggingInterceptor(logger),
			StreamAuthInterceptor(authProvider, healthSkip...),
		),
	)

	hs := health.NewServer()
	healthpb.RegisterHealthServer(srv, hs)

	if cfg.ServerMode != "release" {
		reflection.Register(srv)
	}

	return &Server{
		GRPCServer: srv,
		Health:     hs,
		listener:   lis,
		logger:     logger,
	}, nil
}

// Serve starts the gRPC server. Blocks until stopped.
func (s *Server) Serve() error {
	s.logger.Info("starting grpc server", zap.String("addr", s.listener.Addr().String()))
	return s.GRPCServer.Serve(s.listener)
}

// GracefulStop drains active RPCs then stops.
func (s *Server) GracefulStop() {
	s.logger.Info("stopping grpc server")
	s.GRPCServer.GracefulStop()
}
