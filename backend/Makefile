.PHONY: run build test test-cover fmt vet arch arch-check check tidy

# Run the API. Executed from this directory so .env and clouds.yaml are found
# (configs.LoadEnv also walks up to the directory holding go.mod, so a
# subdirectory works too).
run:
	go run ./cmd

build:
	go build -o bin/api ./cmd

test:
	go test ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

fmt:
	gofmt -w .

vet:
	go vet ./...

# Architecture-boundary check. Requires go-arch-lint:
#   go install github.com/fe3dback/go-arch-lint@latest
arch:
	go-arch-lint check

# Same as `arch` but intended for CI: exits non-zero on a violation.
arch-check:
	go-arch-lint check --max-warnings 0

# Everything the repo can verify without external tools.
check: vet
	@out="$$(gofmt -l .)"; \
	if [ -n "$$out" ]; then echo "unformatted files:"; echo "$$out"; exit 1; fi
	go build ./...
	@echo "check: OK"

tidy:
	go mod tidy
