FROM registry.access.redhat.com/ubi8/ubi:8.9-1107.1705420509 as base

### Pre-install dependencies
# These packages will end up in the final image
# Installed here to save build time
RUN yum --assumeyes install \
    jq \
    && yum clean all;

### Build backplane-cli
FROM registry.access.redhat.com/ubi8/ubi:8.9-1107.1705420509 as bp-cli-builder

RUN yum install --assumeyes \
    make \
    git \
    go-toolset

ENV GOOS=linux GO111MODULE=on GOPROXY=https://proxy.golang.org 
ENV GOBIN=/gobin GOPATH=/usr/src/go CGO_ENABLED=0
ENV PATH=/usr/local/go/bin/:${PATH}

# Directory for the binary
RUN mkdir /out

# Build ocm-backplane from latest
COPY . /ocm-backplane
WORKDIR /ocm-backplane

RUN make build-static
RUN cp ./ocm-backplane /out

RUN chmod -R +x /out

### Build dependencies
FROM registry.access.redhat.com/ubi8/ubi:8.9-1107.1705420509 as dep-builder

RUN yum install --assumeyes \
    jq \
    unzip \
    wget

ARG GITHUB_URL="https://api.github.com"
ARG GITHUB_TOKEN=""

# Replace version with a version number to pin a specific version (eg: "4.7.8")
ARG OC_VERSION="stable"
ENV OC_URL="https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/${OC_VERSION}"

# Replace "/latest" with "/tags/{tag}" to pin to a specific version (eg: "/tags/v0.4.0")
ARG OCM_VERSION="latest"
ENV OCM_URL="${GITHUB_URL}/repos/openshift-online/ocm-cli/releases/${OCM_VERSION}"

# Directory for the extracted binaries, etc
RUN mkdir /out

# Install the latest OC Binary from the mirror
RUN mkdir /oc
WORKDIR /oc
# Download the checksum
RUN curl -sSLf ${OC_URL}/sha256sum.txt -o sha256sum.txt
# Download the amd64 binary tarball
RUN curl -sSLf -O ${OC_URL}/$(awk -v asset="openshift-client-linux" '$0~asset {print $2}' sha256sum.txt | grep -v arm64)
# Check the tarball and checksum match
RUN sha256sum --check --ignore-missing sha256sum.txt
RUN tar --extract --gunzip --no-same-owner --directory /out oc --file *.tar.gz

# Install ocm
# ocm is not in a tarball
RUN mkdir /ocm
WORKDIR /ocm

RUN if [[ -n ${GITHUB_TOKEN} ]]; then \
    echo "Authorization: Bearer ${GITHUB_TOKEN}" > auth.txt; \
    else \
    touch auth.txt; \
    fi

# Download the checksum
RUN curl -H @auth.txt -sSLf $(curl -H @auth.txt -sSLf ${OCM_URL} -o - | jq -r '.assets[] | select(.name|test("linux-amd64.sha256")) | .browser_download_url') -o sha256sum.txt
# Download the binary
RUN curl -H @auth.txt -sSLf -O $(curl -H @auth.txt -sSLf ${OCM_URL} -o - | jq -r '.assets[] | select(.name|test("linux-amd64$")) | .browser_download_url')
# Check the binary and checksum match
RUN sha256sum --check --ignore-missing sha256sum.txt
RUN cp ocm* /out/ocm

# Make binaries executable
RUN chmod -R +x /out

### Build the final image
# This is based on the first image build, with the packages installed
FROM base

# Copy previously acquired binaries into the $PATH
ENV BIN_DIR="/usr/local/bin"
COPY --from=dep-builder /out/oc ${BIN_DIR}
COPY --from=dep-builder /out/ocm ${BIN_DIR}
COPY --from=bp-cli-builder /out/ocm-backplane ${BIN_DIR}

# Validate
RUN oc completion bash > /etc/bash_completion.d/oc
RUN ocm completion > /etc/bash_completion.d/ocm

ENV HOME="/home"
RUN chmod a+w -R ${HOME}
WORKDIR ${HOME}

ENTRYPOINT ["/bin/bash"]
