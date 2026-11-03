VERSION ?= 0.0.1
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X github.com/enver-dev/enver/internal/buildinfo.Version=$(VERSION) \
	-X github.com/enver-dev/enver/internal/buildinfo.Commit=$(COMMIT) \
	-X github.com/enver-dev/enver/internal/buildinfo.Date=$(DATE)

.PHONY: all web build test vet fmt dev clean

all: build

web:
	cd web && npm ci && npm run build

build: web
	mkdir -p dist
	go build -trimpath -ldflags "$(LDFLAGS)" -o dist/enver ./cmd/enver

test:
	go test ./... -count=1

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal

dev:
	go run ./cmd/enver ui --no-auth --no-open --port 7788

clean:
	rm -rf dist
