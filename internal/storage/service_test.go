package storage

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
)

func TestTryAddLimitEnforcement(t *testing.T) {
	service := NewService(10)

	if err := service.TryAdd(4); err != nil {
		t.Fatalf("unexpected error adding 4 bytes: %v", err)
	}
	if err := service.TryAdd(6); err != nil {
		t.Fatalf("unexpected error adding 6 bytes: %v", err)
	}

	err := service.TryAdd(1)
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected ErrLimitExceeded, got %v", err)
	}

	if got := service.Used(); got != 10 {
		t.Fatalf("unexpected used bytes: got %d want 10", got)
	}
}

func TestConcurrentTryAdd(t *testing.T) {
	const (
		adds = 5000
		size = 128
	)

	service := NewService(adds * size)

	var wg sync.WaitGroup
	var successCount atomic.Int64
	var failureCount atomic.Int64

	for i := 0; i < adds; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := service.TryAdd(size); err != nil {
				failureCount.Add(1)
				return
			}
			successCount.Add(1)
		}()
	}
	wg.Wait()

	if got := failureCount.Load(); got != 0 {
		t.Fatalf("unexpected failures: got %d want 0", got)
	}
	if got := successCount.Load(); got != adds {
		t.Fatalf("unexpected successes: got %d want %d", got, adds)
	}

	if got, want := service.Used(), uint64(adds*size); got != want {
		t.Fatalf("unexpected used bytes: got %d want %d", got, want)
	}
}

func TestConcurrentTryAddRespectsLimit(t *testing.T) {
	const (
		limit    = 1000
		attempts = 5000
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
			err := service.TryAdd(1)
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
		t.Fatalf("unexpected errors: got %d want 0", got)
	}
	if got := successCount.Load(); got != limit {
		t.Fatalf("unexpected successes: got %d want %d", got, limit)
	}
	if got := limitedCount.Load(); got != attempts-limit {
		t.Fatalf("unexpected limit rejections: got %d want %d", got, attempts-limit)
	}
	if got := service.Used(); got != limit {
		t.Fatalf("unexpected used bytes: got %d want %d", got, limit)
	}
}

func TestTryAddOverflowHandling(t *testing.T) {
	service := NewService(5)
	service.used.Store(6)

	err := service.TryAdd(0)
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected ErrLimitExceeded when used > limit, got %v", err)
	}
}

func TestTryAddNearMaxUint64(t *testing.T) {
	service := NewService(math.MaxUint64)

	if err := service.TryAdd(math.MaxUint64); err != nil {
		t.Fatalf("unexpected error adding max uint64: %v", err)
	}

	err := service.TryAdd(1)
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("expected ErrLimitExceeded after filling limit, got %v", err)
	}

	if got := service.Used(); got != math.MaxUint64 {
		t.Fatalf("unexpected used bytes: got %d want %d", got, uint64(math.MaxUint64))
	}
}
