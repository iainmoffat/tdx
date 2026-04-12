.PHONY: all build test vet fmt lint clean coverage

all: fmt vet lint test build

build:
	go build -o tdx ./cmd/tdx

test:
	go test ./... -count=1

vet:
	go vet ./...

fmt:
	@gofmt -l -w .
	@echo "gofmt done"

lint:
	golangci-lint run ./...

clean:
	rm -f tdx coverage.out

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
