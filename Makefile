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

# Release builds Linux and Windows. systray is pure Go on these platforms, so
# they cross-compile with CGO disabled — no C toolchain needed.
release: clean
	cd web && npm ci && npm run build
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	  go build -ldflags "$(LDFLAGS)" -o bin/belochka-linux-amd64 ./cmd/server/
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
	  go build -ldflags "$(LDFLAGS)" -o bin/belochka-linux-arm64 ./cmd/server/
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
	  go build -ldflags "$(LDFLAGS) -H windowsgui" -o bin/belochka-windows-x86-64.exe ./cmd/server/
	CGO_ENABLED=0 GOOS=windows GOARCH=386 \
	  go build -ldflags "$(LDFLAGS) -H windowsgui" -o bin/belochka-windows-x86.exe ./cmd/server/
