package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"backendassessment/internal/api"
	"backendassessment/internal/config"
)

type metricsResponse struct {
	RequestCounts map[string]uint64 `json:"request_counts"`
	TotalRequests uint64            `json:"total_requests"`
	MaxRequests   uint64            `json:"max_requests"`
}

type storageResponse struct {
	UsedBytes uint64 `json:"used_bytes"`
	MaxBytes  uint64 `json:"max_bytes"`
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func TestRequestCounterIncrementsCorrectly(t *testing.T) {
	handler := newHandler(1000, 1<<20)

	for i := 0; i < 3; i++ {
		rr := executeRequest(handler, http.MethodPost, "/api/endpoint1", nil, "")
		if rr.Code != http.StatusOK {
			t.Fatalf("unexpected status for endpoint1: got %d", rr.Code)
		}
	}

	metrics := fetchMetrics(t, handler)
	if got := metrics.RequestCounts["/api/endpoint1"]; got != 3 {
		t.Fatalf("unexpected request count: got %d want 3", got)
	}
	if metrics.TotalRequests != 3 {
		t.Fatalf("unexpected total requests: got %d want 3", metrics.TotalRequests)
	}
}

func TestMethodNotAllowedDoesNotIncrementRequestCount(t *testing.T) {
	handler := newHandler(1000, 1<<20)

	rr := executeRequest(handler, http.MethodGet, "/api/endpoint1", nil, "")
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: got %d want %d", rr.Code, http.StatusMethodNotAllowed)
	}

	metrics := fetchMetrics(t, handler)
	if got := metrics.RequestCounts["/api/endpoint1"]; got != 0 {
		t.Fatalf("unexpected request count: got %d want 0", got)
	}
	if metrics.TotalRequests != 0 {
		t.Fatalf("unexpected total requests: got %d want 0", metrics.TotalRequests)
	}
}

func TestMetricsEndpointReturnsExpectedValues(t *testing.T) {
	handler := newHandler(1000, 1<<20)

	rr := executeRequest(handler, http.MethodGet, "/api/metrics", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d", rr.Code)
	}

	var metrics metricsResponse
	decodeJSON(t, rr.Body.Bytes(), &metrics)

	if len(metrics.RequestCounts) != 0 {
		t.Fatalf("expected no tracked endpoints yet, got %d", len(metrics.RequestCounts))
	}
	if metrics.TotalRequests != 0 {
		t.Fatalf("unexpected total requests: got %d want 0", metrics.TotalRequests)
	}
	if metrics.MaxRequests != 1000 {
		t.Fatalf("unexpected max requests: got %d want 1000", metrics.MaxRequests)
	}
}

func TestUploadUpdatesStorageUsage(t *testing.T) {
	handler := newHandler(1000, 1024)

	contentType, payload := multipartPayload(t, []byte("hello world"))
	rr := executeRequest(handler, http.MethodPost, "/upload", bytes.NewReader(payload), contentType)
	if rr.Code != http.StatusCreated {
		t.Fatalf("unexpected upload status: got %d", rr.Code)
	}

	storage := fetchStorage(t, handler)
	if storage.UsedBytes != 11 {
		t.Fatalf("unexpected storage usage: got %d want 11", storage.UsedBytes)
	}
}

func TestInvalidUploadReturnsCorrectError(t *testing.T) {
	handler := newHandler(1000, 1024)

	rr := executeRequest(handler, http.MethodPost, "/upload", bytes.NewBufferString("not multipart"), "text/plain")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d", rr.Code, http.StatusBadRequest)
	}

	var response errorResponse
	decodeJSON(t, rr.Body.Bytes(), &response)
	if response.Error.Code != "invalid_upload" {
		t.Fatalf("unexpected error code: got %q", response.Error.Code)
	}
}

func TestMissingFileFieldReturnsInvalidUpload(t *testing.T) {
	handler := newHandler(1000, 1024)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("meta", "no-file"); err != nil {
		t.Fatalf("failed to write form field: %v", err)
	}
	contentType := writer.FormDataContentType()
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	rr := executeRequest(handler, http.MethodPost, "/upload", bytes.NewReader(body.Bytes()), contentType)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: got %d want %d", rr.Code, http.StatusBadRequest)
	}

	var response errorResponse
	decodeJSON(t, rr.Body.Bytes(), &response)
	if response.Error.Code != "invalid_upload" {
		t.Fatalf("unexpected error code: got %q want %q", response.Error.Code, "invalid_upload")
	}
}

func TestMethodNotAllowedRoutes(t *testing.T) {
	handler := newHandler(1000, 1024)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "metrics rejects post", method: http.MethodPost, path: "/api/metrics"},
		{name: "upload rejects get", method: http.MethodGet, path: "/upload"},
		{name: "storage rejects post", method: http.MethodPost, path: "/storage"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := executeRequest(handler, tc.method, tc.path, nil, "")
			if rr.Code != http.StatusMethodNotAllowed {
				t.Fatalf("unexpected status: got %d want %d", rr.Code, http.StatusMethodNotAllowed)
			}

			var response errorResponse
			decodeJSON(t, rr.Body.Bytes(), &response)
			if response.Error.Code != "method_not_allowed" {
				t.Fatalf("unexpected error code: got %q want %q", response.Error.Code, "method_not_allowed")
			}
		})
	}
}

func TestAPILimitExceededReturnsCorrectStatus(t *testing.T) {
	handler := newHandler(2, 1024)

	first := executeRequest(handler, http.MethodPost, "/api/endpoint1", nil, "")
	if first.Code != http.StatusOK {
		t.Fatalf("unexpected first status: got %d", first.Code)
	}

	second := executeRequest(handler, http.MethodPost, "/api/endpoint1", nil, "")
	if second.Code != http.StatusOK {
		t.Fatalf("unexpected second status: got %d", second.Code)
	}

	third := executeRequest(handler, http.MethodPost, "/api/endpoint1", nil, "")
	if third.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected third status: got %d want %d", third.Code, http.StatusTooManyRequests)
	}

	var response errorResponse
	decodeJSON(t, third.Body.Bytes(), &response)
	if response.Error.Code != "api_limit_exceeded" {
		t.Fatalf("unexpected error code: got %q", response.Error.Code)
	}

	metrics := fetchMetrics(t, handler)
	if got := metrics.RequestCounts["/api/endpoint1"]; got != 2 {
		t.Fatalf("unexpected request count: got %d want 2", got)
	}
}

func TestStorageLimitExceededReturnsCorrectStatus(t *testing.T) {
	handler := newHandler(1000, 5)

	contentType, payload := multipartPayload(t, []byte("abc"))
	first := executeRequest(handler, http.MethodPost, "/upload", bytes.NewReader(payload), contentType)
	if first.Code != http.StatusCreated {
		t.Fatalf("unexpected first upload status: got %d", first.Code)
	}

	second := executeRequest(handler, http.MethodPost, "/upload", bytes.NewReader(payload), contentType)
	if second.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("unexpected second upload status: got %d want %d", second.Code, http.StatusRequestEntityTooLarge)
	}

	var response errorResponse
	decodeJSON(t, second.Body.Bytes(), &response)
	if response.Error.Code != "storage_limit_exceeded" {
		t.Fatalf("unexpected error code: got %q", response.Error.Code)
	}

	storage := fetchStorage(t, handler)
	if storage.UsedBytes != 3 {
		t.Fatalf("unexpected storage usage after rejection: got %d want 3", storage.UsedBytes)
	}
}

func TestConcurrentRequestsProduceCorrectCounts(t *testing.T) {
	const totalRequests = 10000
	handler := newHandler(totalRequests+100, 1<<20)

	var wg sync.WaitGroup
	var successCount atomic.Int64
	var failureCount atomic.Int64

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rr := executeRequest(handler, http.MethodPost, "/api/endpoint1", nil, "")
			if rr.Code == http.StatusOK {
				successCount.Add(1)
				return
			}
			failureCount.Add(1)
		}()
	}

	wg.Wait()

	if failureCount.Load() != 0 {
		t.Fatalf("unexpected failures: %d", failureCount.Load())
	}
	if successCount.Load() != totalRequests {
		t.Fatalf("unexpected successes: got %d want %d", successCount.Load(), totalRequests)
	}

	metrics := fetchMetrics(t, handler)
	if got := metrics.RequestCounts["/api/endpoint1"]; got != totalRequests {
		t.Fatalf("unexpected request count: got %d want %d", got, totalRequests)
	}
}

func TestConcurrentRequestsRespectLimit(t *testing.T) {
	const (
		maxRequests = 1000
		attempts    = 4000
	)

	handler := newHandler(maxRequests, 1<<20)

	var wg sync.WaitGroup
	var successCount atomic.Int64
	var limitedCount atomic.Int64
	var unexpectedCount atomic.Int64

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rr := executeRequest(handler, http.MethodPost, "/api/endpoint1", nil, "")
			switch rr.Code {
			case http.StatusOK:
				successCount.Add(1)
			case http.StatusTooManyRequests:
				limitedCount.Add(1)
			default:
				unexpectedCount.Add(1)
			}
		}()
	}

	wg.Wait()

	if unexpectedCount.Load() != 0 {
		t.Fatalf("unexpected statuses: %d", unexpectedCount.Load())
	}

	if got := successCount.Load(); got != maxRequests {
		t.Fatalf("unexpected successes: got %d want %d", got, maxRequests)
	}

	if got := limitedCount.Load(); got != attempts-maxRequests {
		t.Fatalf("unexpected limited count: got %d want %d", got, attempts-maxRequests)
	}

	metrics := fetchMetrics(t, handler)
	if got := metrics.RequestCounts["/api/endpoint1"]; got != maxRequests {
		t.Fatalf("unexpected request count: got %d want %d", got, maxRequests)
	}
	if metrics.TotalRequests != maxRequests {
		t.Fatalf("unexpected total requests: got %d want %d", metrics.TotalRequests, maxRequests)
	}
}

func TestConcurrentUploadsKeepStorageAccountingCorrect(t *testing.T) {
	const (
		uploadCount = 2000
		fileSize    = 256
	)

	handler := newHandler(1000, uploadCount*fileSize)

	contentType, payload := multipartPayload(t, bytes.Repeat([]byte("a"), fileSize))

	var wg sync.WaitGroup
	var successCount atomic.Int64
	var failureCount atomic.Int64

	for i := 0; i < uploadCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rr := executeRequest(handler, http.MethodPost, "/upload", bytes.NewReader(payload), contentType)
			if rr.Code == http.StatusCreated {
				successCount.Add(1)
				return
			}
			failureCount.Add(1)
		}()
	}

	wg.Wait()

	if failureCount.Load() != 0 {
		t.Fatalf("unexpected failed uploads: %d", failureCount.Load())
	}
	if successCount.Load() != uploadCount {
		t.Fatalf("unexpected successful uploads: got %d want %d", successCount.Load(), uploadCount)
	}

	storage := fetchStorage(t, handler)
	expected := uint64(uploadCount * fileSize)
	if storage.UsedBytes != expected {
		t.Fatalf("unexpected storage usage: got %d want %d", storage.UsedBytes, expected)
	}
}

func newHandler(apiLimit, storageLimit uint64) http.Handler {
	return api.NewHandler(config.Config{
		MaxAPIRequests:  apiLimit,
		MaxStorageBytes: storageLimit,
	})
}

func executeRequest(handler http.Handler, method, path string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	if body == nil {
		body = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func multipartPayload(t *testing.T, data []byte) (string, []byte) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "upload.bin")
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("failed to write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	return writer.FormDataContentType(), body.Bytes()
}

func fetchMetrics(t *testing.T, handler http.Handler) metricsResponse {
	t.Helper()

	rr := executeRequest(handler, http.MethodGet, "/api/metrics", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected metrics status: got %d", rr.Code)
	}

	var response metricsResponse
	decodeJSON(t, rr.Body.Bytes(), &response)
	return response
}

func fetchStorage(t *testing.T, handler http.Handler) storageResponse {
	t.Helper()

	rr := executeRequest(handler, http.MethodGet, "/storage", nil, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected storage status: got %d", rr.Code)
	}

	var response storageResponse
	decodeJSON(t, rr.Body.Bytes(), &response)
	return response
}

func decodeJSON(t *testing.T, raw []byte, target any) {
	t.Helper()

	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
}
