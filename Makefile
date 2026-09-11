.PHONY: test postgres seed run pub web web-dev

test:
	go test ./...

postgres:
	docker compose up -d --wait postgres

seed: postgres
	go run ./cmd/seed

run:
	go run ./cmd/server

pub:
	go run ./cmd/pub

web-dev:
	cd web && npm install && npm run dev

web:
	cd web && npm install && npm run build
