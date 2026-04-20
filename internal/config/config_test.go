package config

import "testing"

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.MaxAPIRequests != DefaultMaxAPIRequests {
		t.Fatalf("unexpected max API requests: got %d want %d", cfg.MaxAPIRequests, DefaultMaxAPIRequests)
	}
	if cfg.MaxStorageBytes != DefaultMaxStorageBytes {
		t.Fatalf("unexpected max storage bytes: got %d want %d", cfg.MaxStorageBytes, DefaultMaxStorageBytes)
	}
}
