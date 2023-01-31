unexport GOFLAGS

GOOS?=linux
GOARCH?=amd64
GOENV=GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 GOFLAGS=

GOLANGCI_LINT_VERSION=v1.50.1

# These tags make sure we can statically link and avoid shared dependencies
GO_BUILD_FLAGS :=-tags 'include_gcs include_oss containers_image_openpgp gssapi'
GO_BUILD_FLAGS_DARWIN :=-tags 'include_gcs include_oss containers_image_openpgp'
GO_BUILD_FLAGS_LINUX_CROSS :=-tags 'include_gcs include_oss containers_image_openpgp'

# To mitigate prow lint failures
HOME=$(shell mktemp -d)

OPENAPI_DESIGN='https://raw.githubusercontent.com/openshift/backplane-api/main/openapi/openapi.yaml'

IMAGE_REGISTRY?=quay.io
IMAGE_REPOSITORY?=app-sre
IMAGE_NAME?=backplane-cli
VERSION=$(shell git rev-parse --short=7 HEAD)
UNAME_S := $(shell uname -s)

IMAGE_URI_VERSION:=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME):$(VERSION)
IMAGE_URI_LATEST:=$(IMAGE_REGISTRY)/$(IMAGE_REPOSITORY)/$(IMAGE_NAME):latest

CONTAINER_ENGINE:=$(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
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
	$(GOPATH)/bin/golangci-lint run

ensure-goreleaser:
	go install github.com/goreleaser/goreleaser@v1.14.1

release: ensure-goreleaser
	goreleaser release --rm-dist

test:
	for t in $$(go list ./...); do go test -v $$t ; done

test-cover:
	go test -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

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

openapi-image:
	$(CONTAINER_ENGINE) build --pull --platform linux/amd64 -f openapi.Dockerfile -t backplane-cli-openapi .

.PHONY: generate
generate: openapi-image
	curl -sSL $(OPENAPI_DESIGN) -o ./docs/backplane-api.yaml
	$(CONTAINER_ENGINE) run --platform linux/amd64 --privileged=true --rm -v $(shell pwd):/app backplane-cli-openapi /bin/sh -c "mkdir -p /app/pkg/client && oapi-codegen -generate types,client,spec /app/docs/backplane-api.yaml > /app/pkg/client/BackplaneApi.go"
	go generate ./...

.PHONY: mock-gen
mock-gen:
	mockgen -destination=./pkg/client/mocks/ClientMock.go -package=mocks gitlab.cee.redhat.com/service/backplane-cli/pkg/client ClientInterface
	mockgen -destination=./pkg/client/mocks/ClientWithResponsesMock.go -package=mocks gitlab.cee.redhat.com/service/backplane-cli/pkg/client ClientWithResponsesInterface
	mockgen -destination=./pkg/utils/mocks/ocmWrapperMock.go -package=mocks gitlab.cee.redhat.com/service/backplane-cli/pkg/utils OCMInterface
	mockgen -destination=./pkg/utils/mocks/clientUtilsMock.go -package=mocks gitlab.cee.redhat.com/service/backplane-cli/pkg/utils ClientUtils

.PHONY: build-image
build-image:
	$(CONTAINER_ENGINE) build --pull --platform linux/amd64 --build-arg=GOLANGCI_LINT_VERSION -t backplane-cli-builder -f ./make.Dockerfile .

.PHONY: lint-in-container
lint-in-container: build-image
	$(RUN_IN_CONTAINER_CMD) "go mod download && make lint"

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
