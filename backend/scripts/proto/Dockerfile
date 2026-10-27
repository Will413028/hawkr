# syntax=docker/dockerfile:1
#
# 固定 protoc + plugin 版本，避免本地 / CI 跑出來的 .pb.go header
# (`// protoc xxx`) 不一致造成假性 diff。版本與 CLAUDE.md §3 規範對齊：
# - libprotoc 33.4 = pb wire format 對應目前 commit 過的生成物
# - protoc-gen-go v1.36.11、protoc-gen-go-grpc v1.6.1 為 pkg/go.mod direct deps
#   （版本以下方 ARG 為準；本註解僅說明來源，改 ARG 時記得同步這行）
#
# 升級流程：(1) 改下面 ARG (2) 本地 ./scripts/gen-go-proto.sh 重 gen
#         (3) 確認 *.pb.go diff 僅為 header 一行 + 預期 wire 改動 (4) PR
FROM golang:1.25.7-alpine@sha256:f6751d823c26342f9506c03797d2527668d095b0a15f1862cddb4d927a7a4ced

ARG PROTOC_VERSION=33.4
ARG PROTOC_GEN_GO_VERSION=1.36.11
ARG PROTOC_GEN_GO_GRPC_VERSION=1.6.1

RUN apk add --no-cache curl unzip ca-certificates

RUN set -eux; \
    case "$(apk --print-arch)" in \
        x86_64)  ARCH="x86_64" ;; \
        aarch64) ARCH="aarch_64" ;; \
        *) echo "Unsupported arch: $(apk --print-arch)"; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-${ARCH}.zip" -o /tmp/protoc.zip; \
    unzip -q /tmp/protoc.zip -d /usr/local; \
    rm /tmp/protoc.zip; \
    chmod +x /usr/local/bin/protoc

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v${PROTOC_GEN_GO_VERSION} \
 && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v${PROTOC_GEN_GO_GRPC_VERSION}

ENV PATH="/root/go/bin:${PATH}"
WORKDIR /work
