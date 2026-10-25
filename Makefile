.PHONY: help
.PHONY: up down restart ps stop hard-restart
.PHONY: collector log-indexer query outbox-worker
.PHONY: test fmt
.PHONY: migrate
.PHONY: integration-postgres

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
	@echo "  make query               Run query"
	@echo ""
	@echo "Code:"
	@echo "  make test                Run tests"
	@echo "  make fmt                 Format code"
	@echo ""
	@echo "Migration:"
	@echo "  make migrate             Migrate Schema"

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

log-indexer:
	go run ./cmd/log-indexer/main.go

query:
	go run ./cmd/query/main.go

outbox-worker:
	go run ./cmd/outbox-worker/main.go

#Code Quality
test:
	go test ./...

fmt:
	go fmt ./...

#Migrations
migrate:
	go run ./cmd/migrate/main.go

#Integration test
integration-postgres:
	docker compose up -d postgresdb
	-docker compose exec postgresdb psql -U myuser -d postgres -c 'CREATE DATABASE luma_test;'
	LUMA_TEST_DATABASE_URL="postgres://myuser:mysecretpassword@localhost:5432/luma_test" go test -tags=integration ./internal/repository/postgres

