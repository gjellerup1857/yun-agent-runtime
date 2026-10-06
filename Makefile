SHELL := /bin/sh

.PHONY: fmt vet test build ci up down logs smoke

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

build:
	go build ./cmd/server

ci: fmt vet test build

up:
	docker compose -f deployments/docker-compose.yml up --build -d

down:
	docker compose -f deployments/docker-compose.yml down -v

logs:
	docker compose -f deployments/docker-compose.yml logs -f yar

smoke:
	sh scripts/smoke.sh
