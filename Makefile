APP_NAME := smart-cs-agent
PORT ?= 8090
DATA_DIR ?= ./data

.PHONY: fmt test build run clean

fmt:
	go fmt ./...

test:
	go test ./...

build:
	go build -o bin/$(APP_NAME) .

run:
	PORT=$(PORT) DATA_DIR=$(DATA_DIR) go run .

clean:
	rm -rf bin data
