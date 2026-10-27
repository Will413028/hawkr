#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR/.."
PROTO_DIR="$PROJECT_ROOT/api/proto"
PROTO_COMMON_DIR="$PROJECT_ROOT/services/java/shared/java-proto"

echo "Proto directory: $PROTO_DIR"
echo "Output directory: $PROTO_COMMON_DIR"

echo "Generating Java protos..."

stage="$(mktemp -d "$PROTO_COMMON_DIR/src/main/.proto.XXXXXX")"
trap 'rm -rf "$stage"' EXIT
cp -R "$PROTO_DIR/." "$stage/"
rm -rf "$PROTO_COMMON_DIR/src/main/proto"
mv "$stage" "$PROTO_COMMON_DIR/src/main/proto"

echo "Building proto-common..."
(cd "$PROJECT_ROOT/services/java" && ./mvnw clean install -pl :proto-common -DskipTests -B)

echo "Java proto generation complete."
