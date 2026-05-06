# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-bookworm AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/inspector .
RUN mkdir -p /out/data

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app

ENV INSPECTOR_ENV=production

COPY --from=builder /out/inspector /app/inspector
COPY --from=builder /src/web/templates /app/web/templates
COPY --from=builder /src/config.example.yaml /app/config.example.yaml
COPY --from=builder --chown=nonroot:nonroot /out/data /app/data

EXPOSE 9090
VOLUME ["/app/data"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD ["/app/inspector", "healthcheck", "http://127.0.0.1:9090/readyz"]

ENTRYPOINT ["/app/inspector"]
CMD ["/app/config.yaml"]
