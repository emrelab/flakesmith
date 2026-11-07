.PHONY: all build test test-repeat cost vet fmt cover fixtures clean run smoke

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

# Where the suite's wall time goes, and how many subprocesses it spawns.
# Operator tool, not a gate: see the header of the script.
cost:
	scripts/test-cost.sh

vet:
	go vet ./...

fmt:
	gofmt -l -w .

cover:
	# -coverpkg is required, not decoration: the CLI's integration tests live in
	# their own packages (internal/cli/hunt, ...), so without it each package is
	# only measured by the tests sitting inside it — internal/cli would report
	# ~0% while its tests all live next door. Measured: 51.1% without, 78.0%
	# with, on the same tree.
	go test -coverprofile=coverage.out -coverpkg=./... ./...
	go tool cover -func=coverage.out | tail -1

# Each fixture is its own module; the root module ignores them, they build separately.
fixtures:
	@for d in testdata/fixtures/*/; do \
		echo "== $$d"; (cd "$$d" && go build ./... && go vet ./...) || exit 1; \
	done

clean:
	rm -f $(BINARY) coverage.out
	rm -rf .flakesmith
