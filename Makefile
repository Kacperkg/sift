.PHONY: dev-api dev-web build test

dev-api:
	set -a; . ./.env; set +a; go run ./cmd/server

dev-web:
	cd web && npm install && npm run dev

build:
	cd web && npm install && npm run build
	go build -o bin/job-inbox ./cmd/server

test:
	go test ./...
	cd web && npm run build
