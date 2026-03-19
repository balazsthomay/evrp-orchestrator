package storage

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"sync"

	evrpv1 "github.com/thomaybalazs/evrp-orchestrator/gen/einride/evrp/v1"
	longrunningpb "cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/protobuf/proto"
)

const (
	defaultPageSize int32 = 50
	maxPageSize     int32 = 1000
)

// MemoryStorage is a thread-safe, in-memory implementation of the Storage
// interface. It stores deep copies of all proto messages so callers cannot
// mutate stored data through retained references.
type MemoryStorage struct {
	mu sync.RWMutex

	// Maps for O(1) lookup by name.
	problems   map[string]*evrpv1.Problem
	solutions  map[string]*evrpv1.Solution
	operations map[string]*longrunningpb.Operation

	// Ordered slices for stable pagination.
	problemNames   []string
	solutionNames  []string
	operationNames []string
}

// NewMemoryStorage creates a new, empty MemoryStorage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		problems:   make(map[string]*evrpv1.Problem),
		solutions:  make(map[string]*evrpv1.Solution),
		operations: make(map[string]*longrunningpb.Operation),
	}
}

// --- Problems ----------------------------------------------------------------

func (m *MemoryStorage) CreateProblem(_ context.Context, problem *evrpv1.Problem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.problems[problem.GetName()]; exists {
		return fmt.Errorf("problem %q already exists", problem.GetName())
	}
	m.problems[problem.GetName()] = proto.Clone(problem).(*evrpv1.Problem)
	m.problemNames = append(m.problemNames, problem.GetName())
	return nil
}

func (m *MemoryStorage) GetProblem(_ context.Context, name string) (*evrpv1.Problem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.problems[name]
	if !ok {
		return nil, fmt.Errorf("problem %q: %w", name, ErrNotFound)
	}
	return proto.Clone(p).(*evrpv1.Problem), nil
}

func (m *MemoryStorage) ListProblems(_ context.Context, pageSize int32, pageToken string) ([]*evrpv1.Problem, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	offset, err := decodePageToken(pageToken)
	if err != nil {
		return nil, "", fmt.Errorf("invalid page token: %w", err)
	}

	size := clampPageSize(pageSize)
	total := len(m.problemNames)

	if offset >= total {
		return nil, "", nil
	}

	end := offset + int(size)
	if end > total {
		end = total
	}

	results := make([]*evrpv1.Problem, 0, end-offset)
	for _, name := range m.problemNames[offset:end] {
		results = append(results, proto.Clone(m.problems[name]).(*evrpv1.Problem))
	}

	var nextToken string
	if end < total {
		nextToken = encodePageToken(end)
	}
	return results, nextToken, nil
}

func (m *MemoryStorage) UpdateProblemState(_ context.Context, name string, state evrpv1.Problem_State) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.problems[name]
	if !ok {
		return fmt.Errorf("problem %q: %w", name, ErrNotFound)
	}
	p.State = state
	return nil
}

// --- Solutions ---------------------------------------------------------------

func (m *MemoryStorage) CreateSolution(_ context.Context, solution *evrpv1.Solution) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.solutions[solution.GetName()]; exists {
		return fmt.Errorf("solution %q already exists", solution.GetName())
	}
	m.solutions[solution.GetName()] = proto.Clone(solution).(*evrpv1.Solution)
	m.solutionNames = append(m.solutionNames, solution.GetName())
	return nil
}

func (m *MemoryStorage) GetSolution(_ context.Context, name string) (*evrpv1.Solution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.solutions[name]
	if !ok {
		return nil, fmt.Errorf("solution %q: %w", name, ErrNotFound)
	}
	return proto.Clone(s).(*evrpv1.Solution), nil
}

// --- Operations --------------------------------------------------------------

func (m *MemoryStorage) CreateOperation(_ context.Context, op *longrunningpb.Operation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.operations[op.GetName()]; exists {
		return fmt.Errorf("operation %q already exists", op.GetName())
	}
	m.operations[op.GetName()] = proto.Clone(op).(*longrunningpb.Operation)
	m.operationNames = append(m.operationNames, op.GetName())
	return nil
}

func (m *MemoryStorage) GetOperation(_ context.Context, name string) (*longrunningpb.Operation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	op, ok := m.operations[name]
	if !ok {
		return nil, fmt.Errorf("operation %q: %w", name, ErrNotFound)
	}
	return proto.Clone(op).(*longrunningpb.Operation), nil
}

func (m *MemoryStorage) UpdateOperation(_ context.Context, op *longrunningpb.Operation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.operations[op.GetName()]; !exists {
		return fmt.Errorf("operation %q: %w", op.GetName(), ErrNotFound)
	}
	m.operations[op.GetName()] = proto.Clone(op).(*longrunningpb.Operation)
	return nil
}

func (m *MemoryStorage) ListOperations(_ context.Context, pageSize int32, pageToken string) ([]*longrunningpb.Operation, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	offset, err := decodePageToken(pageToken)
	if err != nil {
		return nil, "", fmt.Errorf("invalid page token: %w", err)
	}

	size := clampPageSize(pageSize)
	total := len(m.operationNames)

	if offset >= total {
		return nil, "", nil
	}

	end := offset + int(size)
	if end > total {
		end = total
	}

	results := make([]*longrunningpb.Operation, 0, end-offset)
	for _, name := range m.operationNames[offset:end] {
		results = append(results, proto.Clone(m.operations[name]).(*longrunningpb.Operation))
	}

	var nextToken string
	if end < total {
		nextToken = encodePageToken(end)
	}
	return results, nextToken, nil
}

// --- Pagination helpers ------------------------------------------------------

// clampPageSize applies the default and maximum bounds to a requested page
// size.
func clampPageSize(requested int32) int32 {
	if requested <= 0 {
		return defaultPageSize
	}
	if requested > maxPageSize {
		return maxPageSize
	}
	return requested
}

// encodePageToken encodes an integer offset into a base64 page token.
func encodePageToken(offset int) string {
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

// decodePageToken decodes a base64 page token back to an integer offset. An
// empty token decodes to offset 0.
func decodePageToken(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return 0, fmt.Errorf("base64 decode: %w", err)
	}
	offset, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("atoi: %w", err)
	}
	if offset < 0 {
		return 0, fmt.Errorf("negative offset %d", offset)
	}
	return offset, nil
}

// Compile-time interface satisfaction check.
var _ Storage = (*MemoryStorage)(nil)
