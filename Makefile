SERVER_DIR := apps/server
WEB_DIR := apps/web
BIN := bin/cron-agent
DATA_DIR ?= ./data
DATA_ABS := $(abspath $(DATA_DIR))
PORT ?= 14141

-include .env
export

.PHONY: setup dev build run test clean

setup:
	cd $(SERVER_DIR) && go mod tidy
	pnpm install

dev:
	trap 'kill 0' INT TERM; \
	(cd $(SERVER_DIR) && go run ./cmd/cron-agent --data $(DATA_ABS) --port $(PORT)) & \
	(pnpm --dir $(WEB_DIR) dev) & \
	wait

build:
	pnpm --dir $(WEB_DIR) build
	mkdir -p $(SERVER_DIR)/cmd/cron-agent/webdist
	find $(SERVER_DIR)/cmd/cron-agent/webdist -mindepth 1 -not -name README.md -delete
	cp -r $(WEB_DIR)/dist/. $(SERVER_DIR)/cmd/cron-agent/webdist/
	cd $(SERVER_DIR) && go build -o ../../$(BIN) ./cmd/cron-agent
	cp cron-agent.command $(BIN).command
	chmod +x $(BIN).command

run: build
	./$(BIN) --data $(DATA_ABS) --port $(PORT)

test:
	cd $(SERVER_DIR) && go test ./...
	pnpm --dir $(WEB_DIR) typecheck

clean:
	rm -rf $(BIN) $(BIN).command $(WEB_DIR)/dist
