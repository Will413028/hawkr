#!/bin/sh
set -e

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

FLUTTER_VERSION=$(grep '"flutter"' "$REPO_PATH/.fvmrc" | sed -E 's/.*:[[:space:]]*"([^"]+)".*/\1/')
[ -n "$FLUTTER_VERSION" ] || { echo "::error:: 無法從 .fvmrc 讀出 Flutter 版本"; exit 1; }
echo "Flutter version (from .fvmrc): $FLUTTER_VERSION"
FLUTTER_HOME="$HOME/flutter-$FLUTTER_VERSION"
rm -rf "$FLUTTER_HOME"
git clone https://github.com/flutter/flutter.git --depth 1 -b "$FLUTTER_VERSION" "$FLUTTER_HOME"
export PATH="$FLUTTER_HOME/bin:$PATH"

cd "$REPO_PATH"

git fetch --unshallow 2>/dev/null || true

flutter precache --ios
flutter pub get

VERSION_NAME=$(grep -E '^version:' pubspec.yaml | sed -E 's/version:[[:space:]]*([0-9.]+).*/\1/')
BUILD_NUMBER=$(git rev-list --count HEAD)
echo "Building ${FLAVOR}: build_name=${VERSION_NAME}"
flutter build ios --config-only --release --flavor "$FLAVOR" \
  --build-number="$BUILD_NUMBER" --build-name="$VERSION_NAME"

cd ios
if ! command -v pod >/dev/null 2>&1; then
  HOMEBREW_NO_AUTO_UPDATE=1 brew install cocoapods
else
  HOMEBREW_NO_AUTO_UPDATE=1 brew upgrade cocoapods 2>/dev/null || true
fi
echo "pod version: $(pod --version)"

pod repo update
pod install
