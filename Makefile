.PHONY: build run test clean

# Сборка проекта
build:
	docker-compose build

run:
	docker-compose up

test:
	go test -v ./tests/e2e

clean:
	docker-compose down

# Локальный запуск (требует установленный Go и PostgreSQL)
run-local:
	go run main.go

# Миграции базы данных
migrate:
	docker-compose exec postgres psql -U user -d pr_reviewer -f /migrations/001_init.sql

logs:
	docker-compose logs -f app

help:
	@echo "Available commands:"
	@echo "  build     - Build Docker images"
	@echo "  run       - Run the service"
	@echo "  test      - Run E2E tests"
	@echo "  clean     - Stop and clean containers"
	@echo "  run-local - Run without docker"
	@echo "  migrate   - Apply database migrations manually"
	@echo "  logs      - View application logs"