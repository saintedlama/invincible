CMD := ./cmd/invincible
ifeq ($(OS),Windows_NT)
    BIN := invincible.exe
else
    BIN := invincible
endif

.PHONY: build test vet lint clean run

build:
	go build -o $(BIN) $(CMD)

test:
	go test ./...

test-verbose:
	go test -v ./...

vet:
	go vet ./...

lint: vet
	go run golang.org/x/tools/cmd/deadcode@latest ./... 2>/dev/null || true
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

clean:
	rm -f $(BIN)

run: build
	./$(BIN)
