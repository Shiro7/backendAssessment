package api

import (
	"net/http"

	"backendassessment/internal/config"
	"backendassessment/internal/metering"
	"backendassessment/internal/middleware"
	"backendassessment/internal/storage"
)

type Handler struct {
	metering *metering.Service
	storage  *storage.Service
}

func NewHandler(cfg config.Config) http.Handler {
	if cfg.MaxAPIRequests == 0 {
		cfg.MaxAPIRequests = config.DefaultMaxAPIRequests
	}
	if cfg.MaxStorageBytes == 0 {
		cfg.MaxStorageBytes = config.DefaultMaxStorageBytes
	}

	h := &Handler{
		metering: metering.NewService(cfg.MaxAPIRequests),
		storage:  storage.NewService(cfg.MaxStorageBytes),
	}

	mux := http.NewServeMux()
	mux.Handle("/api/endpoint1", middleware.MeterRequests(h.metering, "/api/endpoint1", http.MethodPost, http.HandlerFunc(h.handleEndpoint1)))
	mux.HandleFunc("/api/metrics", h.handleMetrics)
	mux.HandleFunc("/upload", h.handleUpload)
	mux.HandleFunc("/storage", h.handleStorage)

	return mux
}
