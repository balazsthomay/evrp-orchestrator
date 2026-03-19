package server

import (
	"context"

	evrpv1 "github.com/thomaybalazs/evrp-orchestrator/gen/einride/evrp/v1"
	"github.com/thomaybalazs/evrp-orchestrator/storage"
	"buf.build/go/protovalidate"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetProblem retrieves an eVRP problem.
func (s *Server) GetProblem(ctx context.Context, req *evrpv1.GetProblemRequest) (*evrpv1.Problem, error) {
	if err := protovalidate.Validate(req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	problem, err := s.store.GetProblem(ctx, req.GetName())
	if err != nil {
		if storage.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "problem not found: %s", req.GetName())
		}
		return nil, status.Errorf(codes.Internal, "failed to get problem: %v", err)
	}

	return problem, nil
}
