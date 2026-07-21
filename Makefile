.PHONY: up down logs gen lint build-backend test-backend

COMPOSE := podman-compose -f infra/podman-compose.yml

## Bring up the full local stack (infra + backend-services)
up:
	$(COMPOSE) up -d

## Tear down the local stack
down:
	$(COMPOSE) down

## Tail logs from every service
logs:
	$(COMPOSE) logs -f

## Regenerate Go/Python stubs from protos/
gen:
	cd protos && buf generate

## Lint proto contracts (breaking-change + style checks)
lint:
	cd protos && buf lint && buf breaking --against '.git#branch=main'

## Build the backend-services Go binary
build-backend:
	cd backend-services && go build ./...

## Run backend-services tests
test-backend:
	cd backend-services && go test ./...
