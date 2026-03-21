APP=gateway

.PHONY: tidy build run

tidy:
	go mod tidy

build:
	go build -o bin/$(APP) ./cmd/gateway

run:
	go run ./cmd/gateway
