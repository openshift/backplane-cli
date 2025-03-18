unexport GOFLAGS

GOOS?=linux
GOARCH?=amd64
GOENV=GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 GOFLAGS=

# These tags make sure we can statically link and avoid shared dependencies
GO_BUILD_FLAGS :=-tags 'include_gcs include_oss containers_image_openpgp gssapi'
GO_BUILD_FLAGS_DARWIN :=-tags 'include_gcs include_oss containers_image_openpgp'
GO_BUILD_FLAGS_LINUX_CROSS :=-tags 'include_gcs include_oss containers_image_openpgp'

GOLANGCI_LINT_VERSION=v1.61.0
GORELEASER_VERSION=v1.14.1
GOVULNCHECK_VERSION=v1.0.1

TESTOPTS ?=

# Temporary lint cache
export GOLANGCI_LINT_CACHE=/tmp

IMAGE_REGISTRY?=quay.io
IMAGE_REPOSITORY?=app-sre
IMAGE_NAME?=backplane-cli
VERSION=$(shell git rev-parse --short=7 HEAD)
UNAME_S := $(shell uname -s)

IMAGE_URI_VERSION:=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME):$(VERSION)
IMAGE_URI_LATEST:=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME):latest

CONTAINER_ENGINE:=$(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
DOCKER_PRESENT := $(shell basename $$(which docker) 2>/dev/null )
ifeq ($(DOCKER_PRESENT),docker)
	DOCKER_TO_INSTALL := docker.io
endif

PODMAN_PRESENT := $(shell basename $$(which podman) 2>/dev/null  )
ifeq ($(PODMAN_PRESENT),podman)
	DOCKER_TO_INSTALL := podman
endif

ifeq ($(HOST_OS),fedora)
	DOCKER_TO_INSTALL ?= podman
	PACKAGE_MANAGER := yum
endif

RUN_IN_CONTAINER_CMD:=$(CONTAINER_ENGINE) run --platform linux/amd64 --rm -v $(shell pwd):/app -w=/app backplane-cli-builder /bin/bash -c

OUTPUT_DIR :=_output
CROSS_BUILD_BINDIR :=$(OUTPUT_DIR)/bin

build: clean
	go build -o ocm-backplane ./cmd/ocm-backplane || exit 1

build-static: clean
	go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o ocm-backplane ./cmd/ocm-backplane || exit 1

install:
	go install ./cmd/ocm-backplane

clean:
	rm -f ocm-backplane

test-in-container: build-image
	$(RUN_IN_CONTAINER_CMD) "make test"

# Installed using instructions from: https://golangci-lint.run/usage/install/#linux-and-windows
getlint:
	@mkdir -p $(GOPATH)/bin
	@ls $(GOPATH)/bin/golangci-lint 1>/dev/null || (echo "Installing golangci-lint..." && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin $(GOLANGCI_LINT_VERSION))

.PHONY: lint
lint: getlint
	$(GOPATH)/bin/golangci-lint run --timeout 5m

ensure-goreleaser:
	@ls $(GOPATH)/bin/goreleaser 1>/dev/null || go install github.com/goreleaser/goreleaser@${GORELEASER_VERSION}

release: ensure-goreleaser
	goreleaser release --rm-dist

test:
	go test -v $(TESTOPTS) ./...

.PHONY: coverage
coverage:
	hack/codecov.sh

cross-build-darwin-amd64:
	+@GOOS=darwin GOARCH=amd64 go build $(GO_BUILD_FLAGS_DARWIN) -o $(CROSS_BUILD_BINDIR)/ocm-backplane_darwin_amd64 ./cmd/ocm-backplane
.PHONY: cross-build-darwin-amd64

cross-build-linux-amd64:
	+@GOOS=linux GOARCH=amd64 go build $(GO_BUILD_FLAGS_LINUX_CROSS) -o $(CROSS_BUILD_BINDIR)/ocm-backplane_linux_amd64 ./cmd/ocm-backplane
.PHONY: cross-build-linux-amd64

cross-build: cross-build-darwin-amd64 cross-build-linux-amd64
.PHONY: cross-build

clean-cross-build:
	$(RM) -r '$(CROSS_BUILD_BINDIR)'
	if [ -d '$(OUTPUT_DIR)' ]; then rmdir --ignore-fail-on-non-empty '$(OUTPUT_DIR)'; fi
.PHONY: clean-cross-build

.PHONY: generate
generate:
	go generate ./...

.PHONY: mock-gen
mock-gen:
	mockgen -destination=./pkg/client/mocks/ClientMock.go -package=mocks github.com/openshift/backplane-api/pkg/client ClientInterface
	mockgen -destination=./pkg/client/mocks/ClientWithResponsesMock.go -package=mocks github.com/openshift/backplane-api/pkg/client ClientWithResponsesInterface
	mockgen -destination=./pkg/utils/mocks/ClusterMock.go -package=mocks github.com/openshift/backplane-cli/pkg/utils ClusterUtils
	mockgen -destination=./pkg/ocm/mocks/ocmWrapperMock.go -package=mocks github.com/openshift/backplane-cli/pkg/ocm OCMInterface
	mockgen -destination=./pkg/backplaneapi/mocks/clientUtilsMock.go -package=mocks github.com/openshift/backplane-cli/pkg/backplaneapi ClientUtils
	mockgen -destination=./pkg/cli/session/mocks/sessionMock.go -package=mocks github.com/openshift/backplane-cli/pkg/cli/session BackplaneSessionInterface
	mockgen -destination=./pkg/utils/mocks/shellCheckerMock.go -package=mocks github.com/openshift/backplane-cli/pkg/utils ShellCheckerInterface
	mockgen -destination=./pkg/pagerduty/mocks/clientMock.go -package=mocks github.com/openshift/backplane-cli/pkg/pagerduty PagerDutyClient
	mockgen -destination=./pkg/jira/mocks/jiraMock.go -package=mocks github.com/openshift/backplane-cli/pkg/jira IssueServiceInterface
	mockgen -destination=./pkg/healthcheck/mocks/networkMock.go -package=mocks github.com/openshift/backplane-cli/pkg/healthcheck NetworkInterface
	mockgen -destination=./pkg/healthcheck/mocks/httpClientMock.go -package=mocks github.com/openshift/backplane-cli/pkg/healthcheck HTTPClient
	mockgen -destination=./pkg/info/mocks/infoMock.go -package=mocks github.com/openshift/backplane-cli/pkg/info InfoService
	mockgen -destination=./pkg/info/mocks/buildInfoMock.go -package=mocks github.com/openshift/backplane-cli/pkg/info BuildInfoService
	mockgen -destination=./pkg/ssm/mocks/mock_ssmclient.go -package=mocks github.com/openshift/backplane-cli/cmd/ocm-backplane/cloud SSMClient

.PHONY: build-image
build-image:
	$(CONTAINER_ENGINE) build --pull --platform linux/amd64 --build-arg=GOLANGCI_LINT_VERSION -t backplane-cli-builder -f ./make.Dockerfile .

.PHONY: lint-in-container
lint-in-container: build-image
	$(RUN_IN_CONTAINER_CMD) "go mod download && make lint"

ensure-govulncheck:
	@ls $(GOPATH)/bin/govulncheck 1>/dev/null || go install golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}

scan: ensure-govulncheck
	@output=$$(govulncheck ./... 2>&1); \
	echo "$$output"; \
	if echo "$$output" | grep -q "Vulnerability"; then \
		echo "Note: Detected vulnerabilities (non-blocking for now). Consider updating vulnerable Go packages to their fixed versions."; \
		exit 0; \
	fi

image:
	$(CONTAINER_ENGINE) build --pull --platform linux/amd64 -t $(IMAGE_URI_VERSION) .
	$(CONTAINER_ENGINE) tag $(IMAGE_URI_VERSION) $(IMAGE_URI_LATEST)

skopeo-push: image
	skopeo copy \
		--dest-creds "${QUAY_USER}:${QUAY_TOKEN}" \
		"docker-daemon:${IMAGE_URI_VERSION}" \
		"docker://${IMAGE_URI_VERSION}"
	skopeo copy \
		--dest-creds "${QUAY_USER}:${QUAY_TOKEN}" \
		"docker-daemon:${IMAGE_URI_LATEST}" \
		"docker://${IMAGE_URI_LATEST}"

.PHONY: help
help:
	@echo
	@echo "================================================"
	@echo "           ocm-backplane Makefile Help           "
	@echo "================================================"
	@echo
	@echo "Targets:"
	@echo "  lint        Install/Run golangci-lint checks"
	@echo "  clean       Remove compiled binaries and build artifacts"
	@echo "  build       Compile project (set ARCH= for cross-compilation)"
	@echo "  install     Install binary system-wide (set ARCH= for architecture)"
	@echo "  test        Run unit tests"
	@echo "  cross-build Create multi-architecture binaries"
	@echo "  image       Build container image"
	@echo "  help        Show this help message"
	@echo
	@echo "Variables:"
	@echo "  ARCH        Target architecture (e.g., amd64, arm64 - sets GOARCH)"
	@echo "  GOOS        Target OS (default: linux)"
	@echo
	@echo "Examples:"
	@echo "  make build ARCH=arm64       # Cross-compile for ARM64"
	@echo "  make clean && make install  # Fresh install"
	@echo "  GOOS=darwin make build      # Build for macOS"
	@echo
	