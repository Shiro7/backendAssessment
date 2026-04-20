package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"backendassessment/internal/api"
	"backendassessment/internal/config"
)

func main() {
	cfg := config.Default()
	handler := api.NewHandler(cfg)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failed: %v", err)
	}
}
