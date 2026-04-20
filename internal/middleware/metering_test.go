package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"backendassessment/internal/metering"
)

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestMeterRequestsBypassesMeteringOnMethodMismatch(t *testing.T) {
	service := metering.NewService(10)

	var nextCalls atomic.Int64
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})

	handler := MeterRequests(service, "/api/endpoint1", http.MethodPost, next)

	req := httptest.NewRequest(http.MethodGet, "/api/endpoint1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: got %d want %d", rr.Code, http.StatusNoContent)
	}
	if nextCalls.Load() != 1 {
		t.Fatalf("unexpected next call count: got %d want 1", nextCalls.Load())
	}
	if service.Total() != 0 {
		t.Fatalf("unexpected metered total: got %d want 0", service.Total())
	}
	if got := len(service.Snapshot()); got != 0 {
		t.Fatalf("unexpected snapshot size: got %d want 0", got)
	}
}

func TestMeterRequestsReturnsTooManyRequestsOnLimitExceeded(t *testing.T) {
	service := metering.NewService(0)

	var nextCalls atomic.Int64
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})

	handler := MeterRequests(service, "/api/endpoint1", http.MethodPost, next)

	req := httptest.NewRequest(http.MethodPost, "/api/endpoint1", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected status: got %d want %d", rr.Code, http.StatusTooManyRequests)
	}
	if nextCalls.Load() != 0 {
		t.Fatalf("unexpected next call count: got %d want 0", nextCalls.Load())
	}

	var response errorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if response.Error.Code != "api_limit_exceeded" {
		t.Fatalf("unexpected error code: got %q want %q", response.Error.Code, "api_limit_exceeded")
	}
}
