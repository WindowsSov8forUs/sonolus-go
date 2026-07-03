.PHONY: fmt fmt-check vet test build clean

# Remove build artifacts.
clean:
	rm -f sonolus-go.exe engine.test.exe

# Format all Go source files.
fmt:
	gofmt -w internal cmd sonolus

# Fail if any Go file is not gofmt-compliant.
fmt-check:
	test -z "$$(gofmt -l internal cmd sonolus)"

# Run go vet.
vet:
	go vet ./...

# Run all tests.
test:
	go test ./...

# Build all packages.
build:
	go build ./...
