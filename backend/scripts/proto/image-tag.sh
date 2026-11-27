#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."
F="scripts/proto/Dockerfile"

if command -v sha256sum >/dev/null 2>&1; then
  H=$(sha256sum "$F" | cut -c1-12)
else
  H=$(shasum -a 256 "$F" | cut -c1-12)
fi

printf 'hawkr-proto-gen:%s\n' "$H"
