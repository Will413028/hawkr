#!/bin/bash
set -e

# protoc 與 plugin 版本固定在 scripts/proto/Dockerfile，避免不同電腦的
# 本地 protoc 版本污染 .pb.go header（造成假性 diff）。
# 首次跑會 build image，後續用 cache，~30s 內完成。
#
# tag 由 Dockerfile 的內容雜湊算出（見 scripts/proto/image-tag.sh 檔頭）：Dockerfile 改一行
# tag 就變，cache 必然 miss、必然重建。此前 tag 寫死成 PROTOC_VERSION，於是「跑在哪台機器
# 上」決定生成結果——protoc 釘死了，但決定註解排版的 Go toolchain 沒有。
IMAGE="$(bash "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/proto/image-tag.sh")"
MODULE="github.com/Will413028/hawkr/go-pkg"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PROTO_DIR="$ROOT_DIR/api/proto"
OUT_DIR="$ROOT_DIR/pkg"

if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
    echo "Building protoc image ($IMAGE)..."
    docker build -t "$IMAGE" "$SCRIPT_DIR/proto"
fi

echo "Proto directory: $PROTO_DIR"
echo "Output directory: $OUT_DIR"
echo "Generating Go code via Docker..."

find "$PROTO_DIR" -name "*.proto" | while read -r file; do
    name="$(basename "$file" .proto)"
    rel="${file#$ROOT_DIR/}"
    echo "Processing $name..."

    docker run --rm \
        -v "$ROOT_DIR:/work" \
        "$IMAGE" \
        protoc \
            --proto_path=api/proto \
            --go_out=pkg \
            --go_opt=module="$MODULE" \
            --go-grpc_out=pkg \
            --go-grpc_opt=module="$MODULE" \
            "$rel"
done

echo "Go proto generation complete."
