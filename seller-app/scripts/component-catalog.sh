#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

if [[ -x .fvm/flutter_sdk/bin/flutter ]]; then
  catalog_flutter=(.fvm/flutter_sdk/bin/flutter)
elif command -v fvm >/dev/null 2>&1; then
  catalog_flutter=(fvm flutter)
else
  catalog_flutter=(flutter)
fi

case "${1:-run}" in
  run)
    shift || true
    exec "${catalog_flutter[@]}" run -d chrome --target lib/main_catalog.dart "$@"
    ;;
  build)
    shift
    exec "${catalog_flutter[@]}" build web --target lib/main_catalog.dart \
      --output build/catalog --base-href "${CATALOG_BASE_HREF:-/flutter/seller/}" \
      --no-web-resources-cdn "$@"
    ;;
  test)
    shift
    exec "${catalog_flutter[@]}" test test/catalog/component_catalog_test.dart "$@"
    ;;
  *)
    echo 'Usage: bash scripts/component-catalog.sh [run|build|test] [Flutter options]' >&2
    exit 2
    ;;
esac
