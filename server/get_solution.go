package server

import (
	"context"

	evrpv1 "github.com/thomaybalazs/evrp-orchestrator/gen/einride/evrp/v1"
	"github.com/thomaybalazs/evrp-orchestrator/storage"
	"buf.build/go/protovalidate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetSolution retrieves a solution for an eVRP problem.
func (s *Server) GetSolution(ctx context.Context, req *evrpv1.GetSolutionRequest) (*evrpv1.Solution, error) {
	if err := protovalidate.Validate(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	solution, err := s.store.GetSolution(ctx, req.GetName())
	if err != nil {
		if storage.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "solution not found: %s", req.GetName())
		}
		return nil, status.Errorf(codes.Internal, "failed to get solution: %v", err)
	}

	return solution, nil
}
