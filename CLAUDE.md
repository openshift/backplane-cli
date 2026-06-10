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

# Clean cross-build artifacts
make clean-cross-build
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

# Generate code (including mocks)
make generate
```

### Container Operations
```bash
# Build builder image for containerized builds
make build-image

# Run tests in container
make test-in-container

# Run lint in container
make lint-in-container
```

### Release Management
```bash
# Create a release using goreleaser
make release

# Create a release with custom release notes file
make release-with-note NOTE=/path/to/release-note.md

# See docs/release.md for full release process
```

## Project Structure

- `cmd/ocm-backplane/` - Main CLI application entry point
- `pkg/` - Core packages and libraries
- `internal/` - Internal packages
- `docs/` - Documentation
- `hack/` - Build and development scripts
- `make.Dockerfile` - Builder image for containerized linting/testing

## Go Configuration

- **Go Version**:
  - Minimum: See `go` directive in [go.mod](go.mod) (currently 1.25.5)
  - GOTOOLCHAIN enforced in Makefile: `go1.26.4+auto`
  - Recommendation: Use latest stable Go version
- **Module**: `github.com/openshift/backplane-cli`
- **Binary Name**: `ocm-backplane`
- **Build Tags**:
  - Standard: `include_gcs include_oss containers_image_openpgp gssapi`
  - Darwin: `include_gcs include_oss containers_image_openpgp`
  - Linux Cross: `include_gcs include_oss containers_image_openpgp`

## Linting and Code Quality

The project uses the following tools:
- **golangci-lint** : Comprehensive Go linter
  - Timeout: 5 minutes
  - Cache: /tmp (for CI/CD compatibility)
- **goreleaser** : Release automation
- **govulncheck** : Vulnerability scanning (non-blocking warnings)

## Development Workflow

1. Make changes to code
2. Run `make lint` to check code quality
3. Run `make test` to run tests
4. Run `make build` to build the binary
5. Test the functionality manually if needed

## Important Notes

- This is an OCM plugin - the binary has prefix `ocm-` and can be invoked as `ocm backplane`
- Configuration file expected at `$HOME/.config/backplane/config.json`
- The project uses Go modules (no vendoring)
- Static linking is available via `make build-static` target
- Cross-compilation supported for Darwin (macOS) and Linux on AMD64
- The `make.Dockerfile` is used only for containerized linting/testing, not for production images
- Container image builds and publishing (`image`, `skopeo-push`) have been removed from the Makefile
