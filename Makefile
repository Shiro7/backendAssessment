.PHONY: run test test-race

run:
	go run ./cmd/server

test:
	go test ./...

test-race:
	go test -race ./...
