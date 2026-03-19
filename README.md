# eVRP Orchestration Service

A Go gRPC service that orchestrates Electric Vehicle Routing Problem (eVRP) solving. It accepts optimization requests, dispatches them to solver workers, tracks progress via AIP-151 Long Running Operations, and returns optimized route assignments.

Built with Einride's backend stack conventions: protobuf-first API design with [AIP patterns](https://google.aip.dev), gRPC, `log/slog`, [protovalidate](https://github.com/bufbuild/protovalidate), and [Buf](https://buf.build) for proto management.

## Architecture

```
┌─────────────┐     CreateProblem      ┌──────────────┐
│  gRPC Client │ ───────────────────▶  │  gRPC Server  │
│  (grpcurl)   │ ◀─── Operation ─────  │               │
└─────────────┘                        └──────┬───────┘
       │                                      │
       │  GetOperation / GetSolution          │ SubmitProblem
       │                                      ▼
       │                               ┌──────────────┐
       └─────────────────────────────▶  │ Orchestrator  │
                                       │               │
                                       │  • Create LRO │
                                       │  • Dispatch    │
                                       │  • Track       │
                                       └──────┬───────┘
                                              │ goroutine
                                              ▼
                                       ┌──────────────┐
                                       │    Solver     │
                                       │              │
                                       │ Greedy +     │
                                       │ Local Search │
                                       └──────────────┘
```

**Request flow:** Client calls `CreateProblem` with an eVRP definition (shipments, electric vehicles, chargers). The server validates the input, stores the problem, and dispatches a solver goroutine. The client receives an LRO immediately and polls `GetOperation` until completion, then retrieves the solution via `GetSolution`.

### Packages

| Package | Purpose |
|---------|---------|
| `proto/` | Protobuf API definitions with AIP annotations |
| `gen/` | Generated Go code from protos |
| `domain/` | Pure Go types, Haversine distance, battery/energy calculations, route feasibility |
| `storage/` | Storage interface + thread-safe in-memory implementation |
| `solver/` | Solver interface, nearest-neighbor greedy heuristic, 2-opt local search with simulated annealing |
| `orchestrator/` | Goroutine dispatch, LRO lifecycle, proto-domain conversion |
| `server/` | gRPC handlers for EVRPService and Operations |
| `cmd/evrp-server/` | Server entrypoint with graceful shutdown |

## Prerequisites

- Go 1.22+
- [Buf CLI](https://buf.build/docs/installation) (for proto linting/generation)
- [golangci-lint](https://golangci-lint.run/welcome/install/) (optional, for linting)

## Build & Run

```bash
# Build
make build

# Run (listens on :8080)
make run

# Or directly
go run ./cmd/evrp-server
```

The server starts on port 8080 by default. Set `PORT` env var to override.

## Usage with grpcurl

### Submit a problem

```bash
grpcurl -plaintext -d '{
  "problem": {
    "display_name": "Stockholm Deliveries",
    "shipments": [
      {
        "shipment_id": "s1",
        "pickup_location": {"latitude": 59.3293, "longitude": 18.0686},
        "delivery_location": {"latitude": 59.3500, "longitude": 18.0200},
        "weight_kg": 500,
        "delivery_window": {
          "start_time": "2025-01-01T08:00:00Z",
          "end_time": "2025-01-01T20:00:00Z"
        },
        "service_time_seconds": 600
      },
      {
        "shipment_id": "s2",
        "pickup_location": {"latitude": 59.3293, "longitude": 18.0686},
        "delivery_location": {"latitude": 59.3100, "longitude": 18.1000},
        "weight_kg": 300,
        "delivery_window": {
          "start_time": "2025-01-01T08:00:00Z",
          "end_time": "2025-01-01T20:00:00Z"
        },
        "service_time_seconds": 600
      }
    ],
    "vehicles": [
      {
        "vehicle_id": "v1",
        "battery_capacity_kwh": 80,
        "current_charge_kwh": 80,
        "energy_consumption_rate": 0.2,
        "max_payload_kg": 2000,
        "depot_location": {"latitude": 59.3293, "longitude": 18.0686},
        "speed_kmh": 50
      }
    ],
    "chargers": [
      {
        "charger_id": "c1",
        "location": {"latitude": 59.3200, "longitude": 18.0500},
        "num_slots": 2,
        "power_kw": 150
      }
    ],
    "start_time": "2025-01-01T08:00:00Z",
    "end_time": "2025-01-01T20:00:00Z"
  },
  "solve_duration": "3s"
}' localhost:8080 einride.evrp.v1.EVRPService/CreateProblem
```

This returns an Operation with a `name` like `operations/problems/<uuid>`.

### Poll the operation

```bash
grpcurl -plaintext -d '{
  "name": "operations/problems/<uuid>"
}' localhost:8080 google.longrunning.Operations/GetOperation
```

Repeat until `"done": true`.

### Get the solution

```bash
grpcurl -plaintext -d '{
  "name": "problems/<uuid>/solution"
}' localhost:8080 einride.evrp.v1.EVRPService/GetSolution
```

### List problems

```bash
grpcurl -plaintext -d '{"page_size": 10}' \
  localhost:8080 einride.evrp.v1.EVRPService/ListProblems
```

## Testing

```bash
# Run all tests with race detector
make test

# Run with coverage
make coverage

# Lint protos and Go code
make lint
```

## Design Decisions

- **Proto types as canonical data model.** No separate domain-to-proto mapping layer for storage — proto-generated Go structs are stored directly. The `domain/` package exists only for solver-internal calculations where pure Go types are cleaner.
- **AIP-151 Long Running Operations.** `CreateProblem` returns a `google.longrunning.Operation` that the client polls. The server implements both `EVRPServiceServer` and `OperationsServer`.
- **Goroutine-based workers.** Solver dispatch uses goroutines with context cancellation, simulating Cloud Run Jobs locally without external dependencies.
- **No Wire, no hex arch, no Zap.** Following Einride's tech radar — direct constructor injection, flat packages, `log/slog`.

## Simplifications vs. Production

| This project | Production (Einride v2 architecture) |
|---|---|
| In-memory storage | Cloud Spanner for metadata, Cloud Storage for payloads |
| Goroutine workers | Cloud Run Jobs per sub-problem |
| In-process dispatch | Pub/Sub for event-driven result collection |
| Simple greedy + local search solver | Large Neighborhood Search / ALNS with hours of compute |
| No authentication | IAM + service accounts |
| No observability | OpenTelemetry traces + Grafana dashboards |
| Single-node | Horizontally scaled Cloud Run services |
