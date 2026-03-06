FROM registry.access.redhat.com/ubi9/go-toolset:9.7-1772728670 AS builder
WORKDIR /app
USER root
COPY go.mod go.sum ./
RUN go mod download
COPY --chown=default . .
RUN CGO_ENABLED=0 go build -o /tmp/http-test-services .

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
COPY --from=builder /tmp/http-test-services /http-test-services
COPY docs/ /docs/
ENTRYPOINT ["/http-test-services"]
