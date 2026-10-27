# syntax=docker/dockerfile:1.7
ARG GO_VERSION
ARG GO_BASE_IMAGE=golang:${GO_VERSION}-alpine

# === Stage: Font (平台代製收據的中文字型，#3026) ===
#
# 🔴 **必須是 TTF，TTC 不行**：go-pdf/fpdf 對 .ttc 回 "get metrics Error: not supported"
# 之後把字型當成未定義，症狀是產不出 PDF。而 alpine 的 CJK 套件**全部都是 .ttc**
# （font-noto-cjk 的四個檔、font-wqy-zenhei 都是；實測 apk search 過沒有 TTF 的 CJK 套件）
# ⇒ 不能用 apk，改抓一份釘住 SHA256 的靜態 TTF。
#
# Noto Sans TC（SIL Open Font License 1.1，可自由散布與內嵌）。12 MB 進 image，
# 但 fpdf 只把**用到的字**做成子集放進每一份 PDF（實測一張收據約 7.5 KB）。
# 對照：apk add font-noto-cjk 會多 88.8 MB 而且還是不能用。
FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce AS font
RUN apk --no-cache add ca-certificates curl
ARG NOTO_TC_URL=https://raw.githubusercontent.com/google/fonts/main/ofl/notosanstc/NotoSansTC%5Bwght%5D.ttf
ARG NOTO_TC_SHA256=864727d210d54f2537bbe23b3a839436c3992af72de9322af5270897246bd44f
RUN curl -fsSL -o /NotoSansTC.ttf "$NOTO_TC_URL" \
 && echo "$NOTO_TC_SHA256  /NotoSansTC.ttf" | sha256sum -c - \
 && head -c 4 /NotoSansTC.ttf | od -An -tx1 | grep -q '00 01 00 00'

# === Stage: Dev (Air hot-reload，本機開發用，獨立不依賴 go-base) ===
FROM golang:${GO_VERSION}-alpine AS dev
RUN apk add --no-cache git tzdata
RUN go install github.com/air-verse/air@v1.63.0
ENV GOWORK=off
WORKDIR /app
COPY pkg/go.mod pkg/go.sum /app/pkg/
COPY services/invoice-service/go.mod services/invoice-service/go.sum /app/services/invoice-service/
RUN cd /app/services/invoice-service && go mod download
# 收據字型（#3026）。本機 compose 走的是這個 stage，而字型缺席在啟動時就 fail-closed
# ⇒ dev stage 必須跟 prod 拿同一份，否則本機一律起不來。
COPY --from=font /NotoSansTC.ttf /app/fonts/NotoSansTC.ttf
ENV RECEIPT_FONT_PATH=/app/fonts/NotoSansTC.ttf
CMD ["air", "-c", "services/invoice-service/.air.toml"]

# === Stage: Builder（GO_BASE_IMAGE 可換成 Dockerfile.go-base 建出的映像以預載依賴） ===
FROM ${GO_BASE_IMAGE} AS builder
WORKDIR /workspace
COPY pkg ./pkg
COPY services/invoice-service ./services/invoice-service
WORKDIR /workspace/services/invoice-service
ENV GOWORK=off
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" \
        -o /out/invoice-service ./cmd/invoice-service

# === Stage: Prod (最終 thin image) ===
FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce AS prod
RUN apk --no-cache add ca-certificates wget tzdata \
 && addgroup -S hawkr && adduser -S hawkr -G hawkr
WORKDIR /app
COPY --from=builder /out/invoice-service .
# 收據字型（#3026）。路徑由 RECEIPT_FONT_PATH 覆寫。
COPY --from=font /NotoSansTC.ttf /app/fonts/NotoSansTC.ttf
ENV RECEIPT_FONT_PATH=/app/fonts/NotoSansTC.ttf
# 三個 port 是三個獨立的面（58 號設計書 §4.3.1）：
#   10040 HTTP 內部運維面（health/metrics/pprof，永不承載業務 API）
#   10041 gRPC 業務面（唯一 caller 是 Gateway）
#   10042 HTTP PDF 授權下載通道（獨立 http.Server，不掛在 10040 的共用 router 上）
ENV HTTP_PORT=10040
ENV GRPC_PORT=10041
ENV INVOICE_SERVICE_FILE_PORT=10042
USER hawkr
EXPOSE 10040 10041 10042
CMD ["./invoice-service"]
