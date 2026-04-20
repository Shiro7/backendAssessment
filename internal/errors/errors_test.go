package apperrors

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewCreatesError(t *testing.T) {
	err := New(http.StatusTeapot, "brew_error", "short and stout")

	if err.Status != http.StatusTeapot {
		t.Fatalf("unexpected status: got %d want %d", err.Status, http.StatusTeapot)
	}
	if err.Code != "brew_error" {
		t.Fatalf("unexpected code: got %q want %q", err.Code, "brew_error")
	}
	if err.Message != "short and stout" {
		t.Fatalf("unexpected message: got %q want %q", err.Message, "short and stout")
	}
	if got := err.Error(); got != "short and stout" {
		t.Fatalf("unexpected Error() output: got %q want %q", got, "short and stout")
	}
}

func TestWriteHTTPMapping(t *testing.T) {
	rr := httptest.NewRecorder()
	Write(rr, APILimitExceeded)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("unexpected status: got %d want %d", rr.Code, http.StatusTooManyRequests)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("unexpected content-type: got %q want %q", got, "application/json")
	}

	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}

	if envelope.Error.Code != APILimitExceeded.Code {
		t.Fatalf("unexpected error code: got %q want %q", envelope.Error.Code, APILimitExceeded.Code)
	}
	if envelope.Error.Message != APILimitExceeded.Message {
		t.Fatalf("unexpected error message: got %q want %q", envelope.Error.Message, APILimitExceeded.Message)
	}
}

func TestPredefinedErrorStatuses(t *testing.T) {
	tests := []struct {
		name   string
		err    *Error
		status int
	}{
		{name: "method not allowed", err: MethodNotAllowed, status: http.StatusMethodNotAllowed},
		{name: "invalid upload", err: InvalidUpload, status: http.StatusBadRequest},
		{name: "api limit exceeded", err: APILimitExceeded, status: http.StatusTooManyRequests},
		{name: "storage limit exceeded", err: StorageLimitExceeded, status: http.StatusRequestEntityTooLarge},
		{name: "internal", err: Internal, status: http.StatusInternalServerError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Status != tc.status {
				t.Fatalf("unexpected status: got %d want %d", tc.err.Status, tc.status)
			}
		})
	}
}
