CMD     := ./cmd/invincible
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
ifeq ($(OS),Windows_NT)
    BIN := invincible.exe
else
    BIN := invincible
endif

.PHONY: build test fmt vet lint clean run cover

build:
	go build $(LDFLAGS) -o $(BIN) $(CMD)

test:
	go test ./...

test-verbose:
	go test -v ./...

cover:
	go test -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func coverage.out
	go tool cover -html=coverage.out -o coverage.html

fmt:
	gofmt -w .

vet:
	go vet ./...

lint: vet
	@echo "=== gofmt ==="
	@unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then echo "$$unformatted"; exit 1; fi
	@echo "=== go mod tidy ==="
	@go mod tidy
	@if ! git diff --exit-code go.mod go.sum; then echo "go.mod or go.sum not tidy"; exit 1; fi
	go run golang.org/x/tools/cmd/deadcode@latest ./... 2>/dev/null || true
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

clean:
	rm -f $(BIN)

run: build
	./$(BIN)
