package server

import (
	"context"

	evrpv1 "github.com/thomaybalazs/evrp-orchestrator/gen/einride/evrp/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListProblems lists eVRP problems.
func (s *Server) ListProblems(ctx context.Context, req *evrpv1.ListProblemsRequest) (*evrpv1.ListProblemsResponse, error) {
	pageSize := req.GetPageSize()
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	problems, nextToken, err := s.store.ListProblems(ctx, pageSize, req.GetPageToken())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list problems: %v", err)
	}

	return &evrpv1.ListProblemsResponse{
		Problems:      problems,
		NextPageToken: nextToken,
	}, nil
}
