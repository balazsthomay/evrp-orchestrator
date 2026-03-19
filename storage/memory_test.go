package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	evrpv1 "github.com/thomaybalazs/evrp-orchestrator/gen/einride/evrp/v1"
	longrunningpb "cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestProblem(name string) *evrpv1.Problem {
	now := timestamppb.Now()
	return &evrpv1.Problem{
		Name:        name,
		DisplayName: "Test Problem",
		Shipments: []*evrpv1.Shipment{
			{
				ShipmentId: "s1",
				PickupLocation: &evrpv1.Location{
					Latitude:  59.3293,
					Longitude: 18.0686,
				},
				DeliveryLocation: &evrpv1.Location{
					Latitude:  59.8586,
					Longitude: 17.6389,
				},
				WeightKg: 10,
				DeliveryWindow: &evrpv1.TimeWindow{
					StartTime: now,
					EndTime:   now,
				},
			},
		},
		Vehicles: []*evrpv1.Vehicle{
			{
				VehicleId:             "v1",
				BatteryCapacityKwh:    60,
				CurrentChargeKwh:      50,
				EnergyConsumptionRate: 0.2,
				MaxPayloadKg:          1000,
				DepotLocation: &evrpv1.Location{
					Latitude:  59.3293,
					Longitude: 18.0686,
				},
				SpeedKmh: 60,
			},
		},
		StartTime:  now,
		EndTime:    now,
		CreateTime: now,
		State:      evrpv1.Problem_STATE_UNSPECIFIED,
	}
}

func newTestSolution(name string) *evrpv1.Solution {
	return &evrpv1.Solution{
		Name:                 name,
		TotalDistanceKm:      42.5,
		TotalDurationSeconds: 3600,
		ShipmentsAssigned:    1,
		ShipmentsUnassigned:  0,
		CreateTime:           timestamppb.Now(),
	}
}

func newTestOperation(name string) *longrunningpb.Operation {
	return &longrunningpb.Operation{
		Name: name,
		Done: false,
	}
}

// ---------------------------------------------------------------------------
// Problem tests
// ---------------------------------------------------------------------------

func TestCreateAndGetProblem(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	p := newTestProblem("problems/abc")
	if err := s.CreateProblem(ctx, p); err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}

	got, err := s.GetProblem(ctx, "problems/abc")
	if err != nil {
		t.Fatalf("GetProblem: %v", err)
	}
	if got.GetName() != "problems/abc" {
		t.Errorf("got name %q, want %q", got.GetName(), "problems/abc")
	}
	if got.GetDisplayName() != p.GetDisplayName() {
		t.Errorf("got display_name %q, want %q", got.GetDisplayName(), p.GetDisplayName())
	}
	if len(got.GetShipments()) != 1 {
		t.Errorf("got %d shipments, want 1", len(got.GetShipments()))
	}
	if len(got.GetVehicles()) != 1 {
		t.Errorf("got %d vehicles, want 1", len(got.GetVehicles()))
	}
}

func TestCreateProblemDuplicate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	p := newTestProblem("problems/dup")
	if err := s.CreateProblem(ctx, p); err != nil {
		t.Fatalf("first CreateProblem: %v", err)
	}
	if err := s.CreateProblem(ctx, p); err == nil {
		t.Fatal("second CreateProblem should have returned error for duplicate")
	}
}

func TestGetProblemNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	_, err := s.GetProblem(ctx, "problems/nonexistent")
	if err == nil {
		t.Fatal("GetProblem should return error for nonexistent problem")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

func TestGetProblemReturnsCopy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	p := newTestProblem("problems/copy")
	if err := s.CreateProblem(ctx, p); err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}

	got, err := s.GetProblem(ctx, "problems/copy")
	if err != nil {
		t.Fatalf("GetProblem: %v", err)
	}

	// Mutate the returned copy.
	got.DisplayName = "MUTATED"

	// Retrieve again; the stored value should be unchanged.
	got2, err := s.GetProblem(ctx, "problems/copy")
	if err != nil {
		t.Fatalf("GetProblem (second): %v", err)
	}
	if got2.GetDisplayName() == "MUTATED" {
		t.Error("stored problem was mutated through returned reference")
	}
}

func TestUpdateProblemState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	p := newTestProblem("problems/state")
	if err := s.CreateProblem(ctx, p); err != nil {
		t.Fatalf("CreateProblem: %v", err)
	}

	tests := []struct {
		name  string
		state evrpv1.Problem_State
	}{
		{"solving", evrpv1.Problem_STATE_SOLVING},
		{"solved", evrpv1.Problem_STATE_SOLVED},
		{"failed", evrpv1.Problem_STATE_FAILED},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.UpdateProblemState(ctx, "problems/state", tc.state); err != nil {
				t.Fatalf("UpdateProblemState(%s): %v", tc.name, err)
			}
			got, err := s.GetProblem(ctx, "problems/state")
			if err != nil {
				t.Fatalf("GetProblem: %v", err)
			}
			if got.GetState() != tc.state {
				t.Errorf("got state %v, want %v", got.GetState(), tc.state)
			}
		})
	}
}

func TestUpdateProblemStateNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	err := s.UpdateProblemState(ctx, "problems/ghost", evrpv1.Problem_STATE_SOLVING)
	if err == nil {
		t.Fatal("UpdateProblemState should return error for nonexistent problem")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

func TestListProblems(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	// Insert 5 problems.
	for i := range 5 {
		p := newTestProblem(fmt.Sprintf("problems/%d", i))
		if err := s.CreateProblem(ctx, p); err != nil {
			t.Fatalf("CreateProblem(%d): %v", i, err)
		}
	}

	// List all at once.
	results, nextToken, err := s.ListProblems(ctx, 10, "")
	if err != nil {
		t.Fatalf("ListProblems: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("got %d results, want 5", len(results))
	}
	if nextToken != "" {
		t.Errorf("expected empty next token, got %q", nextToken)
	}

	// Verify ordering.
	for i, r := range results {
		want := fmt.Sprintf("problems/%d", i)
		if r.GetName() != want {
			t.Errorf("result[%d] name = %q, want %q", i, r.GetName(), want)
		}
	}
}

func TestListProblemsPagination(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	// Insert 5 problems.
	for i := range 5 {
		p := newTestProblem(fmt.Sprintf("problems/%d", i))
		if err := s.CreateProblem(ctx, p); err != nil {
			t.Fatalf("CreateProblem(%d): %v", i, err)
		}
	}

	// Page 1: size 2.
	page1, token1, err := s.ListProblems(ctx, 2, "")
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page 1: got %d results, want 2", len(page1))
	}
	if token1 == "" {
		t.Fatal("page 1: expected non-empty next token")
	}

	// Page 2: size 2.
	page2, token2, err := s.ListProblems(ctx, 2, token1)
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("page 2: got %d results, want 2", len(page2))
	}
	if token2 == "" {
		t.Fatal("page 2: expected non-empty next token")
	}

	// Page 3: last item.
	page3, token3, err := s.ListProblems(ctx, 2, token2)
	if err != nil {
		t.Fatalf("page 3: %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("page 3: got %d results, want 1", len(page3))
	}
	if token3 != "" {
		t.Errorf("page 3: expected empty next token, got %q", token3)
	}

	// Collect all names across pages and verify completeness.
	var allNames []string
	for _, p := range page1 {
		allNames = append(allNames, p.GetName())
	}
	for _, p := range page2 {
		allNames = append(allNames, p.GetName())
	}
	for _, p := range page3 {
		allNames = append(allNames, p.GetName())
	}
	if len(allNames) != 5 {
		t.Errorf("total results across pages: %d, want 5", len(allNames))
	}
	for i, name := range allNames {
		want := fmt.Sprintf("problems/%d", i)
		if name != want {
			t.Errorf("allNames[%d] = %q, want %q", i, name, want)
		}
	}
}

func TestListProblemsDefaultPageSize(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	// Insert 60 problems (more than default 50).
	for i := range 60 {
		p := newTestProblem(fmt.Sprintf("problems/%03d", i))
		if err := s.CreateProblem(ctx, p); err != nil {
			t.Fatalf("CreateProblem(%d): %v", i, err)
		}
	}

	// Pass pageSize 0 => should use default of 50.
	results, nextToken, err := s.ListProblems(ctx, 0, "")
	if err != nil {
		t.Fatalf("ListProblems: %v", err)
	}
	if len(results) != 50 {
		t.Errorf("got %d results with default page size, want 50", len(results))
	}
	if nextToken == "" {
		t.Error("expected non-empty next token when more results remain")
	}
}

func TestListProblemsEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	results, nextToken, err := s.ListProblems(ctx, 10, "")
	if err != nil {
		t.Fatalf("ListProblems: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
	if nextToken != "" {
		t.Errorf("expected empty next token, got %q", nextToken)
	}
}

func TestListProblemsInvalidToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	_, _, err := s.ListProblems(ctx, 10, "not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid page token")
	}
}

// ---------------------------------------------------------------------------
// Solution tests
// ---------------------------------------------------------------------------

func TestCreateAndGetSolution(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	sol := newTestSolution("problems/abc/solution")
	if err := s.CreateSolution(ctx, sol); err != nil {
		t.Fatalf("CreateSolution: %v", err)
	}

	got, err := s.GetSolution(ctx, "problems/abc/solution")
	if err != nil {
		t.Fatalf("GetSolution: %v", err)
	}
	if got.GetName() != "problems/abc/solution" {
		t.Errorf("got name %q, want %q", got.GetName(), "problems/abc/solution")
	}
	if got.GetTotalDistanceKm() != 42.5 {
		t.Errorf("got total_distance_km %f, want 42.5", got.GetTotalDistanceKm())
	}
	if got.GetShipmentsAssigned() != 1 {
		t.Errorf("got shipments_assigned %d, want 1", got.GetShipmentsAssigned())
	}
}

func TestCreateSolutionDuplicate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	sol := newTestSolution("problems/dup/solution")
	if err := s.CreateSolution(ctx, sol); err != nil {
		t.Fatalf("first CreateSolution: %v", err)
	}
	if err := s.CreateSolution(ctx, sol); err == nil {
		t.Fatal("second CreateSolution should have returned error for duplicate")
	}
}

func TestGetSolutionNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	_, err := s.GetSolution(ctx, "problems/ghost/solution")
	if err == nil {
		t.Fatal("GetSolution should return error for nonexistent solution")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

func TestGetSolutionReturnsCopy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	sol := newTestSolution("problems/copy/solution")
	if err := s.CreateSolution(ctx, sol); err != nil {
		t.Fatalf("CreateSolution: %v", err)
	}

	got, err := s.GetSolution(ctx, "problems/copy/solution")
	if err != nil {
		t.Fatalf("GetSolution: %v", err)
	}

	got.TotalDistanceKm = 999
	got2, err := s.GetSolution(ctx, "problems/copy/solution")
	if err != nil {
		t.Fatalf("GetSolution (second): %v", err)
	}
	if got2.GetTotalDistanceKm() == 999 {
		t.Error("stored solution was mutated through returned reference")
	}
}

// ---------------------------------------------------------------------------
// Operation tests
// ---------------------------------------------------------------------------

func TestCreateAndGetOperation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	op := newTestOperation("operations/xyz")
	if err := s.CreateOperation(ctx, op); err != nil {
		t.Fatalf("CreateOperation: %v", err)
	}

	got, err := s.GetOperation(ctx, "operations/xyz")
	if err != nil {
		t.Fatalf("GetOperation: %v", err)
	}
	if got.GetName() != "operations/xyz" {
		t.Errorf("got name %q, want %q", got.GetName(), "operations/xyz")
	}
	if got.GetDone() {
		t.Error("expected done=false")
	}
}

func TestCreateOperationDuplicate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	op := newTestOperation("operations/dup")
	if err := s.CreateOperation(ctx, op); err != nil {
		t.Fatalf("first CreateOperation: %v", err)
	}
	if err := s.CreateOperation(ctx, op); err == nil {
		t.Fatal("second CreateOperation should have returned error for duplicate")
	}
}

func TestGetOperationNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	_, err := s.GetOperation(ctx, "operations/nonexistent")
	if err == nil {
		t.Fatal("GetOperation should return error for nonexistent operation")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

func TestGetOperationReturnsCopy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	op := newTestOperation("operations/copy")
	if err := s.CreateOperation(ctx, op); err != nil {
		t.Fatalf("CreateOperation: %v", err)
	}

	got, err := s.GetOperation(ctx, "operations/copy")
	if err != nil {
		t.Fatalf("GetOperation: %v", err)
	}
	got.Done = true

	got2, err := s.GetOperation(ctx, "operations/copy")
	if err != nil {
		t.Fatalf("GetOperation (second): %v", err)
	}
	if got2.GetDone() {
		t.Error("stored operation was mutated through returned reference")
	}
}

func TestUpdateOperation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	op := newTestOperation("operations/upd")
	if err := s.CreateOperation(ctx, op); err != nil {
		t.Fatalf("CreateOperation: %v", err)
	}

	updated := proto.Clone(op).(*longrunningpb.Operation)
	updated.Done = true
	if err := s.UpdateOperation(ctx, updated); err != nil {
		t.Fatalf("UpdateOperation: %v", err)
	}

	got, err := s.GetOperation(ctx, "operations/upd")
	if err != nil {
		t.Fatalf("GetOperation: %v", err)
	}
	if !got.GetDone() {
		t.Error("expected done=true after update")
	}
}

func TestUpdateOperationNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	op := newTestOperation("operations/ghost")
	err := s.UpdateOperation(ctx, op)
	if err == nil {
		t.Fatal("UpdateOperation should return error for nonexistent operation")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want ErrNotFound", err)
	}
}

func TestUpdateOperationReturnsCopy(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	op := newTestOperation("operations/updcopy")
	if err := s.CreateOperation(ctx, op); err != nil {
		t.Fatalf("CreateOperation: %v", err)
	}

	updated := proto.Clone(op).(*longrunningpb.Operation)
	updated.Done = true
	if err := s.UpdateOperation(ctx, updated); err != nil {
		t.Fatalf("UpdateOperation: %v", err)
	}

	// Mutate the message we passed to UpdateOperation.
	updated.Done = false

	// The stored copy should still be done=true.
	got, err := s.GetOperation(ctx, "operations/updcopy")
	if err != nil {
		t.Fatalf("GetOperation: %v", err)
	}
	if !got.GetDone() {
		t.Error("stored operation was mutated through caller-retained reference after Update")
	}
}

func TestListOperations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	for i := range 5 {
		op := newTestOperation(fmt.Sprintf("operations/%d", i))
		if err := s.CreateOperation(ctx, op); err != nil {
			t.Fatalf("CreateOperation(%d): %v", i, err)
		}
	}

	results, nextToken, err := s.ListOperations(ctx, 10, "")
	if err != nil {
		t.Fatalf("ListOperations: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("got %d results, want 5", len(results))
	}
	if nextToken != "" {
		t.Errorf("expected empty next token, got %q", nextToken)
	}
	for i, r := range results {
		want := fmt.Sprintf("operations/%d", i)
		if r.GetName() != want {
			t.Errorf("result[%d] name = %q, want %q", i, r.GetName(), want)
		}
	}
}

func TestListOperationsPagination(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	for i := range 7 {
		op := newTestOperation(fmt.Sprintf("operations/%d", i))
		if err := s.CreateOperation(ctx, op); err != nil {
			t.Fatalf("CreateOperation(%d): %v", i, err)
		}
	}

	var allNames []string
	token := ""
	pageCount := 0
	for {
		results, nextToken, err := s.ListOperations(ctx, 3, token)
		if err != nil {
			t.Fatalf("ListOperations page %d: %v", pageCount, err)
		}
		for _, r := range results {
			allNames = append(allNames, r.GetName())
		}
		pageCount++
		if nextToken == "" {
			break
		}
		token = nextToken
	}

	if pageCount != 3 {
		t.Errorf("got %d pages, want 3 (3+3+1)", pageCount)
	}
	if len(allNames) != 7 {
		t.Errorf("total results: %d, want 7", len(allNames))
	}
	for i, name := range allNames {
		want := fmt.Sprintf("operations/%d", i)
		if name != want {
			t.Errorf("allNames[%d] = %q, want %q", i, name, want)
		}
	}
}

func TestListOperationsEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	results, nextToken, err := s.ListOperations(ctx, 10, "")
	if err != nil {
		t.Fatalf("ListOperations: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0", len(results))
	}
	if nextToken != "" {
		t.Errorf("expected empty next token, got %q", nextToken)
	}
}

func TestListOperationsInvalidToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	_, _, err := s.ListOperations(ctx, 10, "%%%invalid%%%")
	if err == nil {
		t.Fatal("expected error for invalid page token")
	}
}

// ---------------------------------------------------------------------------
// Pagination helper tests
// ---------------------------------------------------------------------------

func TestClampPageSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input int32
		want  int32
	}{
		{0, defaultPageSize},
		{-1, defaultPageSize},
		{1, 1},
		{50, 50},
		{1000, 1000},
		{1001, maxPageSize},
		{9999, maxPageSize},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("input_%d", tc.input), func(t *testing.T) {
			t.Parallel()
			got := clampPageSize(tc.input)
			if got != tc.want {
				t.Errorf("clampPageSize(%d) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestPageTokenRoundTrip(t *testing.T) {
	t.Parallel()

	for _, offset := range []int{0, 1, 50, 999, 123456} {
		t.Run(fmt.Sprintf("offset_%d", offset), func(t *testing.T) {
			t.Parallel()
			token := encodePageToken(offset)
			got, err := decodePageToken(token)
			if err != nil {
				t.Fatalf("decodePageToken(%q): %v", token, err)
			}
			if got != offset {
				t.Errorf("round-trip failed: encoded %d, decoded %d", offset, got)
			}
		})
	}
}

func TestDecodePageTokenEmpty(t *testing.T) {
	t.Parallel()
	got, err := decodePageToken("")
	if err != nil {
		t.Fatalf("decodePageToken(\"\"): %v", err)
	}
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// Concurrent access tests
// ---------------------------------------------------------------------------

func TestConcurrentProblemAccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	const n = 100
	var wg sync.WaitGroup
	errs := make(chan error, n*3)

	// Concurrent creates.
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := newTestProblem(fmt.Sprintf("problems/concurrent-%d", i))
			if err := s.CreateProblem(ctx, p); err != nil {
				errs <- fmt.Errorf("create %d: %w", i, err)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent reads.
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("problems/concurrent-%d", i)
			if _, err := s.GetProblem(ctx, name); err != nil {
				errs <- fmt.Errorf("get %d: %w", i, err)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent state updates.
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("problems/concurrent-%d", i)
			if err := s.UpdateProblemState(ctx, name, evrpv1.Problem_STATE_SOLVING); err != nil {
				errs <- fmt.Errorf("update %d: %w", i, err)
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	// Verify all problems are present and in the expected state.
	results, _, err := s.ListProblems(ctx, 1000, "")
	if err != nil {
		t.Fatalf("ListProblems: %v", err)
	}
	if len(results) != n {
		t.Errorf("got %d problems, want %d", len(results), n)
	}
}

func TestConcurrentOperationAccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	const n = 100
	var wg sync.WaitGroup
	errs := make(chan error, n*3)

	// Concurrent creates.
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			op := newTestOperation(fmt.Sprintf("operations/concurrent-%d", i))
			if err := s.CreateOperation(ctx, op); err != nil {
				errs <- fmt.Errorf("create %d: %w", i, err)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent updates.
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			op := &longrunningpb.Operation{
				Name: fmt.Sprintf("operations/concurrent-%d", i),
				Done: true,
			}
			if err := s.UpdateOperation(ctx, op); err != nil {
				errs <- fmt.Errorf("update %d: %w", i, err)
			}
		}(i)
	}
	wg.Wait()

	// Concurrent reads.
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("operations/concurrent-%d", i)
			got, err := s.GetOperation(ctx, name)
			if err != nil {
				errs <- fmt.Errorf("get %d: %w", i, err)
				return
			}
			if !got.GetDone() {
				errs <- fmt.Errorf("operation %d: expected done=true", i)
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

func TestConcurrentMixedReadWrite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryStorage()

	// Pre-populate some data.
	for i := range 10 {
		if err := s.CreateProblem(ctx, newTestProblem(fmt.Sprintf("problems/mixed-%d", i))); err != nil {
			t.Fatalf("setup CreateProblem(%d): %v", i, err)
		}
		if err := s.CreateOperation(ctx, newTestOperation(fmt.Sprintf("operations/mixed-%d", i))); err != nil {
			t.Fatalf("setup CreateOperation(%d): %v", i, err)
		}
	}

	const goroutines = 50
	var wg sync.WaitGroup
	errs := make(chan error, goroutines*4)

	// Concurrent list + create + update operations.
	for i := range goroutines {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// List problems.
			if _, _, err := s.ListProblems(ctx, 5, ""); err != nil {
				errs <- fmt.Errorf("list problems goroutine %d: %w", i, err)
			}
		}(i)

		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// List operations.
			if _, _, err := s.ListOperations(ctx, 5, ""); err != nil {
				errs <- fmt.Errorf("list operations goroutine %d: %w", i, err)
			}
		}(i)

		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Update a random problem state.
			name := fmt.Sprintf("problems/mixed-%d", i%10)
			_ = s.UpdateProblemState(ctx, name, evrpv1.Problem_STATE_SOLVING)
		}(i)

		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Get a random operation.
			name := fmt.Sprintf("operations/mixed-%d", i%10)
			if _, err := s.GetOperation(ctx, name); err != nil {
				errs <- fmt.Errorf("get operation goroutine %d: %w", i, err)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}
