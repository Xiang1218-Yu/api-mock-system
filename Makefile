.PHONY: build run dev test vet fmt tidy docker-up docker-down clean

APP := api-mock
PKG := ./cmd/server

# Build the server binary into ./bin/$(APP).
build:
	go build -o bin/$(APP) $(PKG)

# Run the server locally with a dev secret. The SQLite DB file is created in
# the current directory on first run.
run: build
	JWT_SECRET=dev-secret ./bin/$(APP)

# Run with live config overrides for development.
dev: build
	JWT_SECRET=dev-secret SERVER_PORT=8080 LOG_LEVEL=debug APP_ENV=development ./bin/$(APP)

# Run all tests.
test:
	go test ./...

# Static analysis.
vet:
	go vet ./...

# Format the whole module.
fmt:
	gofmt -s -w .

# Sync go.mod/go.sum with imports.
tidy:
	go mod tidy

# Build and run the Docker stack.
docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

clean:
	rm -rf bin/ $(APP).db $(APP).db-wal $(APP).db-shm
