#!/usr/bin/env bash
set -euo pipefail

FLUTTER_VERSION=$(python3 -c "import json; print(json.load(open('.fvmrc'))['flutter'])")
SDK_DIR="$HOME/.flutter-sdk/$FLUTTER_VERSION"

if [[ ! -x "$SDK_DIR/bin/flutter" ]]; then
  echo "共用快取沒有 Flutter ${FLUTTER_VERSION}（${SDK_DIR}），改用 git clone 安裝一次"
  rm -rf "$SDK_DIR"
  git clone --depth 1 --branch "$FLUTTER_VERSION" https://github.com/flutter/flutter.git "$SDK_DIR"
  "$SDK_DIR/bin/flutter" --version
fi

echo "$SDK_DIR/bin" >> "$GITHUB_PATH"
"$SDK_DIR/bin/flutter" --version
