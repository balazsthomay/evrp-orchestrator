package server

import (
	"context"

	longrunningpb "cloud.google.com/go/longrunning/autogen/longrunningpb"
	"github.com/thomaybalazs/evrp-orchestrator/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetOperation retrieves a long-running operation.
func (s *Server) GetOperation(ctx context.Context, req *longrunningpb.GetOperationRequest) (*longrunningpb.Operation, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	op, err := s.store.GetOperation(ctx, req.GetName())
	if err != nil {
		if storage.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "operation not found: %s", req.GetName())
		}
		return nil, status.Errorf(codes.Internal, "failed to get operation: %v", err)
	}

	return op, nil
}
