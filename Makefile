.PHONY: up down logs gen lint build-backend build-backend-image test-backend

COMPOSE := podman-compose -f infra/podman-compose.yml

## Bring up the full local stack (infra + backend-services) — run
## build-backend-image first if backend-services' code changed
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

## Build the backend-services Go binary (local, not containerized)
build-backend:
	cd backend-services && go build ./...

## Build the backend-services container image (podman-compose can't build it
## directly on Windows — see infra/podman-compose.yml's comment)
build-backend-image:
	podman build -t servicesphere/backend-services -f infra/containerfiles/backend-services.Containerfile .

## Run backend-services tests
test-backend:
	cd backend-services && go test ./...
