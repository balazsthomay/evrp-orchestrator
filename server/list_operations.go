package server

import (
	"context"

	longrunningpb "cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListOperations lists long-running operations.
func (s *Server) ListOperations(ctx context.Context, req *longrunningpb.ListOperationsRequest) (*longrunningpb.ListOperationsResponse, error) {
	pageSize := req.GetPageSize()
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	ops, nextToken, err := s.store.ListOperations(ctx, pageSize, req.GetPageToken())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list operations: %v", err)
	}

	return &longrunningpb.ListOperationsResponse{
		Operations:    ops,
		NextPageToken: nextToken,
	}, nil
}
