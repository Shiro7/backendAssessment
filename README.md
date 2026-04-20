# API and Storage Metering Service

This project implements a small in-memory Go service that meters:

- API request counts per endpoint
- Total uploaded storage usage

It enforces two global hard limits:

- Max API requests: `1000`
- Max storage usage: `1 GB`

The implementation is intentionally focused on correctness, concurrency safety, and clean HTTP/service boundaries for an assessment setting.

## Project Overview

Exposed endpoints:

- `POST /api/endpoint1`
    - Counts as one API request.
    - Rejected once the global API request limit is reached.
- `GET /api/metrics`
    - Returns tracked endpoint counters and total API requests as JSON.
- `POST /upload`
    - Accepts a multipart file upload (`file` field).
    - Tracks uploaded byte size.
    - Rejected if accepting the upload would exceed the storage limit.
- `GET /storage`
    - Returns total storage used and max storage limit as JSON.

## How To Run

1. Ensure Go is installed (Go 1.22+).
2. Start the server:

```bash
go run ./cmd/server
```

Server listens on `:8080` by default.

## How To Test

```bash
go test ./...
```

Tests use `testing` + `httptest` and cover behavior and concurrency expectations.

## Architecture Summary

```text
cmd/server/main.go
internal/config       - defaults for limits
internal/metering     - API request counting + API limit enforcement
internal/storage      - storage byte accounting + storage limit enforcement
internal/middleware   - API metering middleware for tracked endpoints
internal/api          - HTTP handlers + JSON response helpers
internal/errors       - typed API error model and shared error definitions
tests                 - integration-style handler tests
```

## Design Decisions

- In-memory state only:
    - Keeps focus on synchronization correctness and metering logic.
    - Avoids persistence/distributed concerns not needed for this assessment.
- `sync/atomic` for hot counters:
    - Global request total and storage bytes are updated with CAS loops.
    - Avoids coarse locks in high-frequency paths.
- `sync.Map` for per-endpoint counters:
    - Endpoint counters are dynamically managed with lock-free reads in steady state.
- Method-scoped API metering:
    - Only `POST /api/endpoint1` consumes API quota.
    - Method mismatches return `405` and do not consume request budget.
- Thin handlers, logic in services:
    - Handlers perform request parsing/validation and response mapping.
    - Services own metering/storage rules and limit enforcement.
- Consistent JSON error envelopes:
    - Predictable error shape across invalid input, limit, and internal failures.

## Assumptions

- State is process-local and resets on restart.
- Limits are global for this single service instance.
- The assessment is treated as single-user/single-tenant because no tenant model, identity header, or authentication contract is specified.
- Uploaded files do not require permanent persistence for this assessment.
- Primary focus is correctness and concurrency safety, not distributed deployment.

## Limitations

- No durable storage or horizontal consistency across instances.
- No authentication/authorization.
- Upload implementation measures file size from request stream and only meters total bytes.
- Near the storage limit, concurrent uploads may send full request bodies and still receive `413` if quota is exhausted before commit.
- No background cleanup or retention model (by design for this scope).

## Possible Future Improvements

- Add persistence (DB/object storage) when requirements include durability.
- Add per-tenant/per-endpoint quota policies.
- Add authentication and authorization.
- Add structured logging, request IDs, and richer observability.
- Add graceful shutdown and configurable limits via env/flags.
