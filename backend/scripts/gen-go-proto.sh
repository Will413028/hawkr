#!/bin/bash
set -e

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
