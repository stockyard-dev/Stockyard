.PHONY: all build test clean snapshot release docker-build npm-publish export-site check-links

# Officially shipping products (see PRODUCTS_SHIPPING.md)
PRODUCTS = costcap llmcache jsonguard routefall rateshield promptreplay \
           keypool promptguard modelswitch evalgate usagepulse \
           promptpad tokentrim batchqueue multicall streamsnap llmtap contextpack retrypilot \
           stockyard

# Infrastructure tools
TOOLS = sy-keygen sy-api sy-docs

VERSION ?= dev

all: build

build:
	@for p in $(PRODUCTS) $(TOOLS); do \
		echo "Building $$p..."; \
		CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(VERSION)" -o dist/$$p ./cmd/$$p; \
	done
	@echo "Done. $(words $(PRODUCTS)) products + $(words $(TOOLS)) tools in dist/"

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

# Run individual products
run-%:
	go run ./cmd/$*

# Verify all shipping binaries compile
check:
	@for p in $(PRODUCTS) $(TOOLS); do \
		echo -n "$$p: "; \
		CGO_ENABLED=0 go build -o /dev/null ./cmd/$$p && echo "ok" || echo "FAIL"; \
	done

# Export documentation + product pages as static site
export-site:
	./export-site.sh

# Check for broken internal links in dist/
check-links:
	./check-links.sh

# GoReleaser
snapshot:
	goreleaser build --snapshot --clean

release:
	goreleaser release --clean

# Docker
docker-build:
	@for p in $(PRODUCTS); do \
		echo "Building Docker image for $$p..."; \
		docker build --build-arg PRODUCT=$$p --build-arg VERSION=$(VERSION) -t ghcr.io/stockyard-dev/$$p:$(VERSION) .; \
		docker tag ghcr.io/stockyard-dev/$$p:$(VERSION) ghcr.io/stockyard-dev/$$p:latest; \
	done

docker-push:
	@for p in $(PRODUCTS); do \
		docker push ghcr.io/stockyard-dev/$$p:$(VERSION); \
		docker push ghcr.io/stockyard-dev/$$p:latest; \
	done

npm-publish:
	@for p in $(PRODUCTS); do \
		echo "Publishing @stockyard/$$p..."; \
		cd npm/$$p && npm version $(VERSION) --no-git-tag-version && npm publish --access public && cd ../..; \
	done
