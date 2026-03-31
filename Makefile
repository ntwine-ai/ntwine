.PHONY: build-frontend build-backend build run dev

build-frontend:
	cd frontend && npm run build

build-backend:
	go build -o dist/socratic-slopinar ./cmd/server

build: build-frontend build-backend

run: build
	./dist/socratic-slopinar

dev:
	cd frontend && npm run dev &
	go run ./cmd/server -dev
