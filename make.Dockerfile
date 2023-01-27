ARG GOLANGCI_LINT_VERSION

FROM registry.access.redhat.com/ubi8/ubi

RUN yum install -y ca-certificates git go-toolset make
ENV PATH="/root/go/bin:${PATH}"

ADD https://password.corp.redhat.com/RH-IT-Root-CA.crt /etc/pki/ca-trust/source/anchors/
RUN update-ca-trust extract

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin ${GOLANGCI_LINT_VERSION}

RUN go install github.com/golang/mock/mockgen@v1.6.0
