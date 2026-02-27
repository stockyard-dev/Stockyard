.PHONY: all build test clean run dev check lint

VERSION ?= dev

all: build

# Build the unified binary
build:
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o dist/stockyard ./cmd/stockyard
	@echo "Built dist/stockyard ($(VERSION))"

# Build all binaries (unified + tools)
build-all: build
	CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/sy-api ./cmd/sy-api
	CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/sy-keygen ./cmd/sy-keygen
	@echo "Built 3 binaries in dist/"

test:
	go test ./... -count=1 -race -timeout 120s

test-v:
	go test ./... -count=1 -v -timeout 120s

cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

lint:
	go vet ./...
	@echo "No issues"

clean:
	rm -rf dist/ coverage.out

# Run locally
run:
	go run ./cmd/stockyard

dev:
	STOCKYARD_ADMIN_KEY=dev go run ./cmd/stockyard

# Verify it compiles
check:
	CGO_ENABLED=0 go build -o /dev/null ./cmd/stockyard && echo "stockyard: ok"
	CGO_ENABLED=0 go build -o /dev/null ./cmd/sy-api && echo "sy-api: ok"

# Cross-compile for releases
release-build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o dist/stockyard_linux_amd64 ./cmd/stockyard
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o dist/stockyard_linux_arm64 ./cmd/stockyard
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o dist/stockyard_darwin_amd64 ./cmd/stockyard
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o dist/stockyard_darwin_arm64 ./cmd/stockyard
	@echo "Built 4 release binaries in dist/"

# Docker
docker:
	docker build -t stockyard:$(VERSION) .
	docker tag stockyard:$(VERSION) stockyard:latest

# Benchmarks
bench:
	go test ./internal/proxy/ -bench=. -benchmem -count=3 -timeout 120s

bench-short:
	go test ./internal/proxy/ -bench=. -benchmem -count=1 -timeout 60s

# Doctor (run health check)
doctor: build
	./dist/stockyard doctor

# Sync site/ to internal/site/static/
site-sync:
	@find site -name "*.html" -o -name "*.xml" -o -name "*.sh" -o -name "*.txt" | while read f; do \
		dest="internal/site/static/$${f#site/}"; \
		mkdir -p "$$(dirname "$$dest")"; \
		cp "$$f" "$$dest"; \
	done
	@echo "Synced site/ → internal/site/static/"

# Full check before pushing
pre-push: lint test bench-short
	@echo "All checks passed"

# Integration smoke test (builds binary, boots server, hits all endpoints)
smoke:
	bash test/smoke_test.sh
