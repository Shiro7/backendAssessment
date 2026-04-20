package metering

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestIncrementLimitEnforcement(t *testing.T) {
	service := NewService(2)

	if err := service.Increment("/api/endpoint1"); err != nil {
		t.Fatalf("unexpected error on first increment: %v", err)
	}
	if err := service.Increment("/api/endpoint1"); err != nil {
		t.Fatalf("unexpected error on second increment: %v", err)
	}

	err := service.Increment("/api/endpoint1")
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected ErrLimitExceeded, got %v", err)
	}

	if got := service.Total(); got != 2 {
		t.Fatalf("unexpected total: got %d want 2", got)
	}

	snapshot := service.Snapshot()
	if got := snapshot["/api/endpoint1"]; got != 2 {
		t.Fatalf("unexpected endpoint count: got %d want 2", got)
	}
}

func TestIncrementEmptyEndpoint(t *testing.T) {
	service := NewService(10)

	err := service.Increment("")
	if !errors.Is(err, ErrEmptyEndpoint) {
		t.Fatalf("expected ErrEmptyEndpoint, got %v", err)
	}

	if got := service.Total(); got != 0 {
		t.Fatalf("unexpected total: got %d want 0", got)
	}
}

func TestConcurrentIncrementRespectsLimit(t *testing.T) {
	const (
		limit    = 1000
		attempts = 4000
	)

	service := NewService(limit)

	var wg sync.WaitGroup
	var successCount atomic.Int64
	var limitedCount atomic.Int64
	var unexpectedCount atomic.Int64

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := service.Increment("/api/endpoint1")
			switch {
			case err == nil:
				successCount.Add(1)
			case errors.Is(err, ErrLimitExceeded):
				limitedCount.Add(1)
			default:
				unexpectedCount.Add(1)
			}
		}()
	}

	wg.Wait()

	if got := unexpectedCount.Load(); got != 0 {
		t.Fatalf("unexpected error count: got %d want 0", got)
	}
	if got := successCount.Load(); got != limit {
		t.Fatalf("unexpected success count: got %d want %d", got, limit)
	}
	if got := limitedCount.Load(); got != attempts-limit {
		t.Fatalf("unexpected limited count: got %d want %d", got, attempts-limit)
	}

	if got := service.Total(); got != limit {
		t.Fatalf("unexpected total: got %d want %d", got, limit)
	}
}

func TestSnapshotAfterConcurrentIncrements(t *testing.T) {
	const total = 2000

	service := NewService(total + 10)

	var wg sync.WaitGroup
	var unexpectedErrors atomic.Int64
	for i := 0; i < total; i++ {
		endpoint := "/api/endpoint1"
		if i%2 == 0 {
			endpoint = "/api/endpoint2"
		}

		wg.Add(1)
		go func(ep string) {
			defer wg.Done()
			if err := service.Increment(ep); err != nil {
				unexpectedErrors.Add(1)
			}
		}(endpoint)
	}
	wg.Wait()

	if got := unexpectedErrors.Load(); got != 0 {
		t.Fatalf("unexpected increment errors: got %d want 0", got)
	}

	snapshot := service.Snapshot()
	if got := snapshot["/api/endpoint1"]; got != total/2 {
		t.Fatalf("unexpected endpoint1 count: got %d want %d", got, total/2)
	}
	if got := snapshot["/api/endpoint2"]; got != total/2 {
		t.Fatalf("unexpected endpoint2 count: got %d want %d", got, total/2)
	}

	sum := snapshot["/api/endpoint1"] + snapshot["/api/endpoint2"]
	if got := service.Total(); sum != got {
		t.Fatalf("snapshot sum and total mismatch: sum=%d total=%d", sum, got)
	}

	snapshot["/api/endpoint1"] = 0
	refreshed := service.Snapshot()
	if refreshed["/api/endpoint1"] == 0 {
		t.Fatalf("snapshot mutation leaked into service state")
	}
}
