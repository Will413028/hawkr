# syntax=docker/dockerfile:1.7
ARG GO_VERSION
ARG GO_BASE_IMAGE=golang:${GO_VERSION}-alpine

# === Stage: Dev (Air hot-reload，本機開發用，獨立不依賴 go-base) ===
FROM golang:${GO_VERSION}-alpine AS dev
RUN apk add --no-cache git tzdata
RUN go install github.com/air-verse/air@v1.63.0
ENV GOWORK=off
WORKDIR /app
COPY pkg/go.mod pkg/go.sum /app/pkg/
COPY services/review-service/go.mod services/review-service/go.sum /app/services/review-service/
RUN cd /app/services/review-service && go mod download
CMD ["air", "-c", "services/review-service/.air.toml"]

# === Stage: Builder（GO_BASE_IMAGE 可換成 Dockerfile.go-base 建出的映像以預載依賴） ===
FROM ${GO_BASE_IMAGE} AS builder
WORKDIR /workspace
COPY pkg ./pkg
COPY services/review-service ./services/review-service
WORKDIR /workspace/services/review-service
ENV GOWORK=off
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" \
        -o /out/review-service ./cmd/review-service
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" \
        -o /out/grpc-healthcheck ./cmd/grpc-healthcheck

# === Stage: Prod (最終 thin image) ===
FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce AS prod
RUN apk --no-cache add ca-certificates tzdata \
 && addgroup -S hawkr && adduser -S hawkr -G hawkr
WORKDIR /app
COPY --from=builder /out/review-service .
COPY --from=builder /out/grpc-healthcheck .
ENV GRPC_PORT=10033
USER hawkr
EXPOSE 10033
CMD ["./review-service"]
