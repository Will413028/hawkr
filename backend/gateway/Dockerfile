# syntax=docker/dockerfile:1.7
ARG GO_VERSION
ARG GO_BASE_IMAGE=golang:${GO_VERSION}-alpine

# === Stage: Dev (Air hot-reload，本機開發用，獨立不依賴 go-base) ===
FROM golang:${GO_VERSION}-alpine AS dev
RUN apk add --no-cache git tzdata
RUN go install github.com/air-verse/air@v1.63.0
ENV GOWORK=off
ENV GIN_MODE=release
WORKDIR /app
COPY pkg/go.mod pkg/go.sum /app/pkg/
COPY gateway/go.mod gateway/go.sum /app/gateway/
RUN cd /app/gateway && go mod download
EXPOSE 9900 8080
CMD ["air", "-c", "gateway/.air.toml"]

# === Stage: Builder（GO_BASE_IMAGE 可換成 Dockerfile.go-base 建出的映像以預載依賴） ===
FROM ${GO_BASE_IMAGE} AS builder
WORKDIR /workspace
COPY pkg ./pkg
COPY gateway ./gateway
WORKDIR /workspace/gateway
ENV GOWORK=off
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" \
        -o /out/gateway ./cmd/main.go

# === Stage: Prod (最終 thin image) ===
FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce AS prod
RUN apk --no-cache add ca-certificates wget tzdata
WORKDIR /app
COPY --from=builder /out/gateway .
ENV GIN_MODE=release
EXPOSE 9900 8080
CMD ["./gateway"]
