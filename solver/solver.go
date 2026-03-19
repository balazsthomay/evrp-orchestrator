package solver

import (
	"context"
	"time"

	"github.com/thomaybalazs/evrp-orchestrator/domain"
)

// SolveOptions configures the solver behavior.
type SolveOptions struct {
	// MaxDuration is the maximum time to spend solving.
	// If zero, defaults to 5 seconds.
	MaxDuration time.Duration
}

// Solver produces solutions for eVRP problems.
type Solver interface {
	// Solve attempts to find a feasible solution for the given problem.
	Solve(ctx context.Context, problem *domain.Problem, opts SolveOptions) (*domain.Solution, error)
}
