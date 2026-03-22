APP=gateway
GO_LD_FLAGS=-ldflags="-s -w"
BUILD_DIR=bin

.PHONY: tidy build run run-dev frontend-install frontend-build frontend-dev docker-build docker-up docker-down docker-logs clean

tidy:
	go mod tidy

build: tidy
	go build $(GO_LD_FLAGS) -o $(BUILD_DIR)/$(APP) ./cmd/gateway

build-frontend:
	cd frontend && npm install && npm run build

build-all: build build-frontend

run: build
	$(BUILD_DIR)/$(APP)

run-dev:
	go run ./cmd/gateway

frontend-install:
	cd frontend && npm install

frontend-build:
	cd frontend && npm run build

frontend-dev:
	cd frontend && npm run dev

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

clean:
	rm -rf $(BUILD_DIR)
	cd frontend && rm -rf dist node_modules
