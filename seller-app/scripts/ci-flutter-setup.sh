#!/usr/bin/env bash
#
# 自架 runner 專用的 Flutter SDK 設置——不用 subosito/flutter-action 的
# cache: true。那個機制是設計給用完即丟的 GitHub-hosted runner，在永久存活的
# self-hosted VM 上，實測即使回報 cache hit 仍會整包重新從 GitHub cache 服務
# 下載 SDK tarball（~1.6GB，25-30MB/s，耗時 60-100+ 秒），對永久機器是純浪費，
# 還會佔用其他 runner 共用的對外頻寬。
#
# 改成直接檢查固定共用路徑：已存在就直接用（本機磁碟，零網路），不存在才
# 用 git clone 補一次（僅版本變更時才會觸發）。
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
