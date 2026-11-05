.PHONY: all build test vet fmt cover fixtures clean run smoke

BINARY := flakesmith
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

all: fmt vet test build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/flakesmith

run: build
	./$(BINARY) help

test:
	go test -race ./...

# action.yml's step scripts are validated locally, without GitHub.
smoke:
	scripts/action-smoke.sh

# Using our own tool on ourselves: a state leak would surface here.
test-repeat:
	go test -count=10 ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

# Each fixture is its own module; the root module ignores them, they build separately.
fixtures:
	@for d in testdata/fixtures/*/; do \
		echo "== $$d"; (cd "$$d" && go build ./... && go vet ./...) || exit 1; \
	done

clean:
	rm -f $(BINARY) coverage.out
	rm -rf .flakesmith
