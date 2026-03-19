package server

import (
	"log/slog"

	evrpv1 "github.com/thomaybalazs/evrp-orchestrator/gen/einride/evrp/v1"
	longrunningpb "cloud.google.com/go/longrunning/autogen/longrunningpb"
	"github.com/thomaybalazs/evrp-orchestrator/orchestrator"
	"github.com/thomaybalazs/evrp-orchestrator/storage"
)

// Server implements the EVRPService and Operations gRPC services.
type Server struct {
	evrpv1.UnimplementedEVRPServiceServer
	longrunningpb.UnimplementedOperationsServer

	store        storage.Storage
	orchestrator *orchestrator.Orchestrator
	logger       *slog.Logger
}

// New creates a new Server.
func New(store storage.Storage, orch *orchestrator.Orchestrator, logger *slog.Logger) *Server {
	return &Server{
		store:        store,
		orchestrator: orch,
		logger:       logger,
	}
}
