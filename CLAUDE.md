# Claude Code Configuration

This file contains configuration and documentation for Claude Code to help with development tasks.

## Project Overview

This is the backplane-cli project, a CLI tool to interact with the OpenShift backplane API. It's written in Go and provides cluster management capabilities.

## Common Commands

### Building and Development
```bash
# Build the project
make build

# Build static binary
make build-static

# Install binary system-wide
make install

# Clean build artifacts
make clean

# Cross-platform builds
make cross-build
make cross-build-darwin-amd64
make cross-build-linux-amd64
```

### Testing and Quality
```bash
# Run tests
make test

# Run linting
make lint

# Generate coverage report
make coverage

# Security vulnerability scan
make scan

# Generate mocks
make mock-gen

# Generate code
make generate
```

### Container Operations
```bash
# Build container image
make image

# Build builder image
make build-image

# Run tests in container
make test-in-container

# Run lint in container
make lint-in-container
```

## Project Structure

- `cmd/ocm-backplane/` - Main CLI application entry point
- `pkg/` - Core packages and libraries
- `internal/` - Internal packages
- `vendor/` - Vendored dependencies
- `docs/` - Documentation
- `hack/` - Build and development scripts

## Go Configuration

- **Go Version**: 1.19+
- **Module**: `github.com/openshift/backplane-cli`
- **Binary Name**: `ocm-backplane`

## Linting Configuration

The project uses golangci-lint with the following enabled linters:
- errcheck
- gosec
- gosimple
- govet
- ineffassign
- staticcheck
- typecheck
- unused
- stylecheck

## Development Workflow

1. Make changes to code
2. Run `make lint` to check code quality
3. Run `make test` to run tests
4. Run `make build` to build the binary
5. Test the functionality manually if needed

## Important Notes

- This is an OCM plugin - the binary has prefix `ocm-` and can be invoked as `ocm backplane`
- Configuration file expected at `$HOME/.config/backplane/config.json`
- The project uses Go modules with vendoring
- Static linking is used for the final binary