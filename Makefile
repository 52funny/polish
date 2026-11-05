.DEFAULT_GOAL := build

GO ?= go
GOFMT ?= gofmt
PYTHON ?= python3
ARGS ?=

.PHONY: help build install run test test-race vet fmt fmt-check check e2e clean

help:
	@printf '%s\n' \
		'make build       Build bin/polish (default)' \
		'make install     Install to the Go bin directory (GOBIN or GOPATH/bin)' \
		'make run         Build and run; accepts ARGS=--selection' \
		'make test        Run tests' \
		'make test-race   Run tests with the race detector' \
		'make vet         Run go vet' \
		'make fmt         Format Go source files' \
		'make fmt-check   Check formatting without modifying files' \
		'make check       Check formatting, run go vet and race tests' \
		'make e2e         Build and run the native Wayland smoke test' \
		'make clean       Remove bin/polish and bin/polish-e2e'

build:
	$(GO) build -o bin/polish ./cmd/polish

install:
	$(GO) install ./cmd/polish

run: build
	./bin/polish $(ARGS)

test:
	$(GO) test -timeout 45s ./...

test-race:
	$(GO) test -race -timeout 60s ./...

vet:
	$(GO) vet ./...

fmt:
	$(GOFMT) -w cmd internal

fmt-check:
	@unformatted="$$($(GOFMT) -l cmd internal)" || exit $$?; \
	if [ -n "$$unformatted" ]; then \
		printf '%s\n' "$$unformatted" 'Run make fmt to format the files listed above.'; \
		exit 1; \
	fi

check: fmt-check vet test-race

e2e:
	$(GO) build -tags e2e -o bin/polish-e2e ./cmd/polish
	$(PYTHON) scripts/smoke.py

clean:
	rm -f bin/polish bin/polish-e2e
