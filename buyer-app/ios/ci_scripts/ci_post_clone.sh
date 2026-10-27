#!/bin/sh
# Xcode Cloud post-clone hook（flavor 依 workflow 的 scheme 自動對應，dev/stage/prod 通用）。
# Xcode Cloud 不認得 Flutter，這支在 clone 完、xcodebuild 之前先把 Flutter 環境備好：
# 裝對版本的 Flutter → pub get → 寫好 Generated.xcconfig（含 FLUTTER_ROOT，讓 Xcode 的
# Run Script 找得到 Flutter）→ pod install。之後 Xcode Cloud 用該 scheme(Release-<flavor>)
# archive 並上 TestFlight。各環境只要在 Xcode Cloud 建對應 scheme 的 workflow、且綁到對應
# 的 App Store Connect app 即可，這支不用改。
set -e

# --- 0. flavor 依正在 build 的 scheme 決定（dev/stage/prod；本機無此變數時退 dev）---
REPO_PATH="${CI_PRIMARY_REPOSITORY_PATH:-$(cd "$(dirname "$0")/../.." && pwd)}"
SCHEME="${CI_XCODE_SCHEME:-dev}"
case "$SCHEME" in
  dev|stage|prod)
    FLAVOR="$SCHEME"
    ;;
  *-dev|*-stage|*-prod)
    FLAVOR="${SCHEME##*-}"
    ;;
  *)
    echo "::error:: Unsupported Xcode scheme '$SCHEME'. Expected dev/stage/prod or *-dev/*-stage/*-prod."
    exit 1
    ;;
esac
echo "Scheme: $SCHEME"
echo "Flavor: $FLAVOR"

# --- 1. 安裝 Flutter（版本動態讀 .fvmrc，不寫死）---
FLUTTER_VERSION=$(grep '"flutter"' "$REPO_PATH/.fvmrc" | sed -E 's/.*:[[:space:]]*"([^"]+)".*/\1/')
[ -n "$FLUTTER_VERSION" ] || { echo "::error:: 無法從 .fvmrc 讀出 Flutter 版本"; exit 1; }
echo "Flutter version (from .fvmrc): $FLUTTER_VERSION"
FLUTTER_HOME="$HOME/flutter-$FLUTTER_VERSION"
rm -rf "$FLUTTER_HOME"
git clone https://github.com/flutter/flutter.git --depth 1 -b "$FLUTTER_VERSION" "$FLUTTER_HOME"
export PATH="$FLUTTER_HOME/bin:$PATH"

# --- 2. 回到 repo 根目錄 ---
cd "$REPO_PATH"

# Xcode Cloud 可能淺層 clone；後面要算 build name/number 需完整歷史，先補深。
git fetch --unshallow 2>/dev/null || true

# --- 3. Flutter 相依 + iOS 產物 ---
flutter precache --ios
flutter pub get

# --- 4. 行銷版本動態讀 pubspec；建置版號交給 Xcode Cloud 管 ---
VERSION_NAME=$(grep -E '^version:' pubspec.yaml | sed -E 's/version:[[:space:]]*([0-9.]+).*/\1/')
BUILD_NUMBER=$(git rev-list --count HEAD)
echo "Building ${FLAVOR}: build_name=${VERSION_NAME}"
# --config-only 寫 Generated.xcconfig（FLUTTER_ROOT + 行銷版本），Xcode archive 才找得到
# Flutter、CFBundleShortVersionString 才正確。
# 註：建置版號(CFBundleVersion) Xcode Cloud 一律用自己的 CI_BUILD_NUMBER 蓋掉，
#     起始值在 App Store Connect → Xcode Cloud → Settings → Build Number 設；這裡的
#     --build-number 只是讓 config-only 有合法值，最終會被蓋，不必在意。
# 媒體域名基底（49 §8.5）由 flavor 自動對應（dev/stage→test-media、prod→media），
# 見 lib/config/flavor.dart mediaBaseHost + lib/config/media_image.dart；不需任何
# dart-define / 環境變數注入。
flutter build ios --config-only --release --flavor "$FLAVOR" \
  --build-number="$BUILD_NUMBER" --build-name="$VERSION_NAME"

# --- 5. CocoaPods（Flutter 設定就緒後才裝得對）---
cd ios
# 確保 CocoaPods 為最新版本：部分 Xcode Cloud image 預裝舊版 pod 與新 trunk specs 不相容，
# 觸發 "specs repository is too out-of-date" 即使 lock 已是合法 spec。
if ! command -v pod >/dev/null 2>&1; then
  HOMEBREW_NO_AUTO_UPDATE=1 brew install cocoapods
else
  HOMEBREW_NO_AUTO_UPDATE=1 brew upgrade cocoapods 2>/dev/null || true
fi
echo "pod version: $(pod --version)"

# 分離 repo update 與 install：'pod install --repo-update' 在某些情境下仍會先撞到
# stale specs check 才走 update；明確先 update 再 install 較穩健。
pod repo update
pod install
