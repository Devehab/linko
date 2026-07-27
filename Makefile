BINARY  := linko
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all
all: verify build

## deps: download modules and write go.sum
.PHONY: deps
deps:
	go mod tidy

## build: compile ./linko for the current platform
.PHONY: build
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

## install: build and install into $GOPATH/bin
.PHONY: install
install:
	go install -trimpath -ldflags "$(LDFLAGS)" .

## test: run the full test suite
.PHONY: test
test:
	go test ./... -count=1

## race: run the tests with the race detector
.PHONY: race
race:
	go test ./... -race -count=1

## cover: run tests and print per-package coverage
.PHONY: cover
cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -n 20

## verify: fmt check + vet + tests (run this before committing)
.PHONY: verify
verify: fmt-check vet test

.PHONY: fmt
fmt:
	gofmt -s -w .

.PHONY: fmt-check
fmt-check:
	@out=$$(gofmt -s -l .); \
	if [ -n "$$out" ]; then echo "gofmt needed for:"; echo "$$out"; exit 1; fi

.PHONY: vet
vet:
	go vet ./...

## release: cross-compile archives into dist/
.PHONY: release
release:
	@rm -rf dist && mkdir -p dist
	@for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64; do \
		os=$${target%/*}; arch=$${target#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		echo "building $$os/$$arch"; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)$$ext . || exit 1; \
		if [ "$$os" = "windows" ]; then \
			(cd dist && zip -q $(BINARY)_$(VERSION)_$${os}_$${arch}.zip $(BINARY)$$ext && rm $(BINARY)$$ext); \
		else \
			(cd dist && tar -czf $(BINARY)_$(VERSION)_$${os}_$${arch}.tar.gz $(BINARY) && rm $(BINARY)); \
		fi; \
	done
	@ls -1 dist

## clean: remove build artefacts
.PHONY: clean
clean:
	rm -rf dist coverage.out $(BINARY)

.PHONY: help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //'
