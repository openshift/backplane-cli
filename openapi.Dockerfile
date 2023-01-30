FROM golang

RUN go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.10.1
