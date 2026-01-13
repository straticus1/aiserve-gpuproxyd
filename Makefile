.PHONY: build run test clean docker-build docker-up docker-down help

build:
	@echo "Building aiserve-gpuproxyd..."
	CGO_ENABLED=1 go build -o bin/aiserve-gpuproxyd ./cmd/server
	@echo "Building aiserve-gpuproxy-client..."
	CGO_ENABLED=1 go build -o bin/aiserve-gpuproxy-client ./cmd/client
	@echo "Building aiserve-gpuproxy-admin..."
	CGO_ENABLED=1 go build -o bin/aiserve-gpuproxy-admin ./cmd/admin

run:
	@echo "Starting aiserve-gpuproxyd..."
	./bin/aiserve-gpuproxyd

run-dev:
	@echo "Starting aiserve-gpuproxyd in developer mode..."
	./bin/aiserve-gpuproxyd -dv -dm

test:
	@echo "Running tests..."
	go test -v ./...

clean:
	@echo "Cleaning..."
	rm -rf bin/

docker-build:
	@echo "Building Docker image..."
	docker build -t gpuproxy:latest .

docker-up:
	@echo "Starting Docker services..."
	docker-compose up -d

docker-down:
	@echo "Stopping Docker services..."
	docker-compose down

docker-logs:
	docker-compose logs -f server

migrate:
	@echo "Running migrations..."
	./bin/aiserve-gpuproxy-admin migrate

deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

help:
	@echo "GPU Proxy Makefile Commands:"
	@echo "  make build        - Build all binaries (aiserve-gpuproxyd, aiserve-gpuproxy-client, aiserve-gpuproxy-admin)"
	@echo "  make run          - Run the server (aiserve-gpuproxyd)"
	@echo "  make run-dev      - Run server in developer/debug mode"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-up    - Start Docker services"
	@echo "  make docker-down  - Stop Docker services"
	@echo "  make docker-logs  - View server logs"
	@echo "  make migrate      - Run database migrations (aiserve-gpuproxy-admin)"
	@echo "  make deps         - Install dependencies"
