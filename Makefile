.PHONY: test run seed pub web

test:
	go test ./...

run:
	go run ./cmd/server

seed:
	go run ./cmd/seed

pub:
	go run ./cmd/pub

web:
	cd web && npm install && npm run build
