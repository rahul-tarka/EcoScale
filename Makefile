.PHONY: build test run docker helm lint tidy

BINARY := ecoscale
VERSION ?= 0.1.0

build:
	go build -o bin/$(BINARY) ./cmd/ecoscale

test:
	go test ./...

run: build
	./bin/$(BINARY)

docker:
	docker build -t ecoscale:$(VERSION) .

tidy:
	go mod tidy

lint:
	go vet ./...
	staticcheck ./... 2>/dev/null || true

.DEFAULT_GOAL := build
