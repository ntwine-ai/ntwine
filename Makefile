.PHONY: build run dev cli clean

build:
	go build -o dist/ntwine ./cmd/server
	go build -o dist/ntwine-cli ./cmd/cli

run:
	go run ./cmd/server

dev:
	go run ./cmd/server -dev -no-browser

cli:
	go run ./cmd/cli -prompt "$(PROMPT)" -codebase "$(CODEBASE)" -rounds "$(or $(ROUNDS),5)"

clean:
	rm -rf dist/
