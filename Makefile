.PHONY: run build clean docker-build docker-up docker-down docker-dev-up docker-dev-down

run:
	go run main.go

build:
	go build -o inspector.exe main.go

clean:
	del /f inspector.exe inspector.db 2>nul || true

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-dev-up:
	docker compose -f docker-compose.dev.yml up -d

docker-dev-down:
	docker compose -f docker-compose.dev.yml down
