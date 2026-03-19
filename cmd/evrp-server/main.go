package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	longrunningpb "cloud.google.com/go/longrunning/autogen/longrunningpb"
	evrpv1 "github.com/thomaybalazs/evrp-orchestrator/gen/einride/evrp/v1"
	"github.com/thomaybalazs/evrp-orchestrator/orchestrator"
	"github.com/thomaybalazs/evrp-orchestrator/server"
	"github.com/thomaybalazs/evrp-orchestrator/solver"
	"github.com/thomaybalazs/evrp-orchestrator/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Initialize components.
	store := storage.NewMemoryStorage()
	s := solver.NewLocalSearchSolver()
	orch := orchestrator.New(store, s, logger)

	// Create gRPC server.
	srv := server.New(store, orch, logger)
	grpcServer := grpc.NewServer()
	evrpv1.RegisterEVRPServiceServer(grpcServer, srv)
	longrunningpb.RegisterOperationsServer(grpcServer, srv)
	reflection.Register(grpcServer)

	// Listen.
	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error("failed to listen", "addr", addr, "error", err)
		os.Exit(1)
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		logger.Info("shutting down")
		orch.Shutdown()
		grpcServer.GracefulStop()
	}()

	logger.Info("server starting", "addr", addr)
	if err := grpcServer.Serve(lis); err != nil {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}
}
