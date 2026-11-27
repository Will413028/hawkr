#!/bin/bash
set -e

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
