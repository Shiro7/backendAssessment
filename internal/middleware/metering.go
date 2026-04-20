package middleware

import (
	"errors"
	"net/http"

	apperrors "backendassessment/internal/errors"
	"backendassessment/internal/metering"
)

func MeterRequests(service *metering.Service, endpoint, method string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			next.ServeHTTP(w, r)
			return
		}

		err := service.Increment(endpoint)
		if err == nil {
			next.ServeHTTP(w, r)
			return
		}

		if errors.Is(err, metering.ErrLimitExceeded) {
			apperrors.Write(w, apperrors.APILimitExceeded)
			return
		}

		apperrors.Write(w, apperrors.Internal)
	})
}
