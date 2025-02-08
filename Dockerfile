# syntax = docker/dockerfile:1
ARG GO_BASE_VERSION=1.23
FROM golang:$GO_BASE_VERSION as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot AS runner-base
WORKDIR /
USER 65532:65532

ENTRYPOINT ["/kubeconfig-operator"]

FROM builder as prod-builder
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -v -a -o /kubeconfig-operator cmd/main.go

FROM runner-base as production
COPY --from=prod-builder /kubeconfig-operator /kubeconfig-operator
