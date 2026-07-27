.PHONY: build fmt lint vet test check

# Build the binary.
build:
	go build -o showroom-waiting .

# Format the code.
fmt:
	gofmt -w .

# Run the linters (requires golangci-lint).
lint:
	golangci-lint run

# Run go vet.
vet:
	go vet ./...

# Run the tests.
test:
	go test ./...

# Run everything CI checks (format check, vet, lint, test).
check: vet lint test
	@test -z "$$(gofmt -l .)" || { echo "unformatted files:"; gofmt -l .; exit 1; }
