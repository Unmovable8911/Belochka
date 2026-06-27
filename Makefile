VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  = -s -w -X main.version=$(VERSION)

.PHONY: dev-backend dev-frontend build clean release

dev-backend:
	go run ./cmd/server/

dev-frontend:
	cd web && npm run dev

build:
	cd web && npm run build
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/belochka ./cmd/server/

clean:
	rm -rf bin web/dist

release: clean
	cd web && npm ci && npm run build
	mkdir -p bin
	CGO_ENABLED=1 \
	  GOOS=linux GOARCH=amd64 \
	  go build -ldflags "$(LDFLAGS)" -o bin/belochka-linux-amd64 ./cmd/server/
	PKG_CONFIG_PATH=/usr/lib/aarch64-linux-gnu/pkgconfig \
	  CC=aarch64-linux-gnu-gcc \
	  CGO_ENABLED=1 \
	  GOOS=linux GOARCH=arm64 \
	  go build -ldflags "$(LDFLAGS)" -o bin/belochka-linux-arm64 ./cmd/server/
	CC=x86_64-w64-mingw32-gcc \
	  CGO_ENABLED=1 \
	  GOOS=windows GOARCH=amd64 \
	  go build -ldflags "$(LDFLAGS) -H windowsgui" -o bin/belochka-windows-amd64.exe ./cmd/server/
