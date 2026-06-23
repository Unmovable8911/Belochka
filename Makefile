.PHONY: dev-backend dev-frontend build clean

dev-backend:
	go run ./cmd/server/

dev-frontend:
	cd web && npm run dev

build:
	cd web && npm run build
	mkdir -p bin
	go build -o bin/belochka ./cmd/server/

clean:
	rm -rf bin web/dist
