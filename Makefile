.PHONY: dev-backend dev-frontend build

dev-backend:
	go run ./cmd/server/

dev-frontend:
	cd web && npm run dev

build:
	@echo "Production build placeholder"
	cd web && npm run build
	go build -o belochka ./cmd/server/
