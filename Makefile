.PHONY: generate lint test build run clean

generate:
	buf generate proto

lint:
	buf lint
	golangci-lint run ./...

test:
	go test -race -count=1 ./...

coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

build:
	go build -o bin/evrp-server ./cmd/evrp-server

run: build
	./bin/evrp-server

clean:
	rm -rf bin/ coverage.out
