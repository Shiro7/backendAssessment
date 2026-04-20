package apperrors

import "net/http"

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func New(status int, code, message string) *Error {
	return &Error{
		Status:  status,
		Code:    code,
		Message: message,
	}
}

var (
	MethodNotAllowed     = New(http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	InvalidUpload        = New(http.StatusBadRequest, "invalid_upload", "invalid upload payload")
	APILimitExceeded     = New(http.StatusTooManyRequests, "api_limit_exceeded", "api request limit exceeded")
	StorageLimitExceeded = New(http.StatusRequestEntityTooLarge, "storage_limit_exceeded", "storage limit exceeded")
	Internal             = New(http.StatusInternalServerError, "internal_error", "internal server error")
)
