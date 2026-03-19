// Package storage defines the storage interface and implementations for the
// eVRP orchestration service.
package storage

import (
	"context"
	"errors"

	evrpv1 "github.com/thomaybalazs/evrp-orchestrator/gen/einride/evrp/v1"
	longrunningpb "cloud.google.com/go/longrunning/autogen/longrunningpb"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// IsNotFound reports whether an error wraps ErrNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// Storage defines the persistence interface for the eVRP orchestration service.
type Storage interface {
	// Problems

	// CreateProblem persists a new problem. The problem's Name field must be set
	// and unique.
	CreateProblem(ctx context.Context, problem *evrpv1.Problem) error

	// GetProblem retrieves a problem by its resource name. Returns ErrNotFound
	// if the problem does not exist.
	GetProblem(ctx context.Context, name string) (*evrpv1.Problem, error)

	// ListProblems returns a page of problems. pageSize controls how many are
	// returned (default 50, max 1000). pageToken is an opaque pagination cursor;
	// pass the empty string for the first page.
	ListProblems(ctx context.Context, pageSize int32, pageToken string) ([]*evrpv1.Problem, string, error)

	// UpdateProblemState transitions a problem to the given state. Returns
	// ErrNotFound if the problem does not exist.
	UpdateProblemState(ctx context.Context, name string, state evrpv1.Problem_State) error

	// Solutions

	// CreateSolution persists a new solution. The solution's Name field must be
	// set and unique.
	CreateSolution(ctx context.Context, solution *evrpv1.Solution) error

	// GetSolution retrieves a solution by its resource name. Returns ErrNotFound
	// if the solution does not exist.
	GetSolution(ctx context.Context, name string) (*evrpv1.Solution, error)

	// Operations

	// CreateOperation persists a new long-running operation. The operation's
	// Name field must be set and unique.
	CreateOperation(ctx context.Context, op *longrunningpb.Operation) error

	// GetOperation retrieves an operation by its resource name. Returns
	// ErrNotFound if the operation does not exist.
	GetOperation(ctx context.Context, name string) (*longrunningpb.Operation, error)

	// UpdateOperation replaces an existing operation. Returns ErrNotFound if the
	// operation does not exist.
	UpdateOperation(ctx context.Context, op *longrunningpb.Operation) error

	// ListOperations returns a page of operations. pageSize controls how many
	// are returned (default 50, max 1000). pageToken is an opaque pagination
	// cursor; pass the empty string for the first page.
	ListOperations(ctx context.Context, pageSize int32, pageToken string) ([]*longrunningpb.Operation, string, error)
}
