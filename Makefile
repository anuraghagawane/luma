.PHONY: help
.PHONY: up down restart ps stop hard-restart
.PHONY: collector worker query
.PHONY: test fmt

help:
	@echo "Available commands:"
	@echo ""
	@echo "Infrastructure:"
	@echo "  make up                  Start infrastructure"
	@echo "  make down                Stop infrastructure and remove containers"
	@echo "  make stop                Stop infrastructure"
	@echo "  make restart             Restart infrastructure"
	@echo "  make hard-restart        Restart infrastructure while removing containers"
	@echo "  make ps                  Show running containers"
	@echo ""
	@echo "Services:"
	@echo "  make collector           Run collector"
	@echo "  make worker              Run worker"
	@echo ""
	@echo "Code:"
	@echo "  make test                Run tests"
	@echo "  make fmt                 Format code"



# Infrastructure
up:
	docker compose up -d

down:
	docker compose down

stop:
	docker compose stop

restart:
	docker compose stop
	docker compose up -d

hard-restart:
	docker compose down
	docker compose up -d


ps:
	docker compose ps -a


#Services
collector:
	go run ./cmd/collector/main.go

worker:
	go run ./cmd/worker/main.go

query:
	go run ./cmd/query/main.go

#Code Quality
test:
	go test ./...

fmt:
	go fmt ./...
