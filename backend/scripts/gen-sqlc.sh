#!/bin/bash
set -e

# sqlc 官方 prebuilt image，已鎖定版本。不像 protoc 需要自建 Dockerfile，
# 這裡版本只有這一處宣告——不會出現「Dockerfile ARG 與腳本 IMAGE tag
# 兩處不同步」的問題（scripts/proto/Dockerfile 對照組曾踩過這個坑）。
#
# 升版流程：(1) 改下面 IMAGE 的 tag (2) 本地 ./scripts/gen-sqlc.sh 重 gen 全部 service
#         (3) review diff (4) commit + PR（.github/workflows/sqlc-generated-diff.yml
#             讀的是同一個變數，不需要另外改版本號）
IMAGE="sqlc/sqlc:1.31.1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

gen_one() {
    local svc="$1"
    echo ">>> $svc"
    docker run --rm \
        -v "$ROOT_DIR/services/$svc:/work" \
        -w /work/db \
        "$IMAGE" generate
}

if [ -n "${1:-}" ]; then
    if [ ! -d "$ROOT_DIR/services/$1/db" ]; then
        echo "error: no such service '$1' (expected services/$1/db to exist)" >&2
        exit 1
    fi
    gen_one "$1"
else
    for dir in "$ROOT_DIR"/services/*/db; do
        [ -f "$dir/sqlc.yaml" ] || continue
        gen_one "$(basename "$(dirname "$dir")")"
    done
fi
