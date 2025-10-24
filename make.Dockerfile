ARG GOLANGCI_LINT_VERSION

FROM registry.access.redhat.com/ubi8/ubi

RUN yum install -y ca-certificates git go-toolset make

ENV GOPATH=/go
ENV PATH="$GOPATH/bin:${PATH}"
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ${GOPATH}/bin ${GOLANGCI_LINT_VERSION}

ADD https://password.corp.redhat.com/RH-IT-Root-CA.crt /etc/pki/ca-trust/source/anchors/
RUN update-ca-trust extract

RUN go install github.com/golang/mock/mockgen@v1.6.0
