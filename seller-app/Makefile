# Flutter command - use FVM if available, otherwise use system Flutter
FLUTTER = $(shell which fvm > /dev/null 2>&1 && echo "fvm flutter" || echo "flutter")
DART = $(shell which fvm > /dev/null 2>&1 && echo "fvm dart" || echo "dart")

# 外部服務設定（Firebase、Sentry、API 主機）：複製 config/dart-defines.example.json 成
# config/dart-defines.json 並填值（不進版控）。檔案不存在時不帶，Firebase 與 Sentry 停用，
# dev 連本機單體。登入用的 GOOGLE_IOS_CLIENT_ID、GOOGLE_WEB_CLIENT_ID、LINE_CHANNEL_ID 也在這個檔案；沒填時登入顯示「登入服務尚未設定」。
DEFINES_FILE ?= config/dart-defines.json
# GATEWAY_HOST 也可以直接帶：`make stage-debug GATEWAY_HOST=https://api.example.test`。
GATEWAY_HOST ?=
DEFINES_FLAG = $(if $(wildcard $(DEFINES_FILE)),--dart-define-from-file=$(DEFINES_FILE)) $(if $(GATEWAY_HOST),--dart-define=GATEWAY_HOST=$(GATEWAY_HOST))

.PHONY: require-gateway help dev-debug dev-profile dev-release stage-debug stage-profile stage-release prod-debug prod-profile prod-release clean clean-ios clean-reinstall analyze format test check-updates deadcode enum-text


# ============================================================
# 開發環境 (dev)
# ============================================================

dev-debug:
	@echo "Running dev environment (Debug)..."
	$(FLUTTER) run --flavor dev -t lib/main.dart --dart-define=FLUTTER_FLAVOR=dev $(DEFINES_FLAG)

dev-profile:
	@echo "Running dev environment (Profile)..."
	$(FLUTTER) run --flavor dev -t lib/main.dart --dart-define=FLUTTER_FLAVOR=dev $(DEFINES_FLAG) --profile

dev-release:
	@echo "Running dev environment (Release)..."
	$(FLUTTER) run --flavor dev -t lib/main.dart --dart-define=FLUTTER_FLAVOR=dev $(DEFINES_FLAG) --release

# ============================================================
# 預發布環境 (stage)
# ============================================================

stage-debug: require-gateway
	@echo "Running stage environment (Debug)..."
	$(FLUTTER) run --flavor stage -t lib/main.dart --dart-define=FLUTTER_FLAVOR=stage $(DEFINES_FLAG)

stage-profile: require-gateway
	@echo "Running stage environment (Profile)..."
	$(FLUTTER) run --flavor stage -t lib/main.dart --dart-define=FLUTTER_FLAVOR=stage $(DEFINES_FLAG) --profile

stage-release: require-gateway
	@echo "Running stage environment (Release)..."
	$(FLUTTER) run --flavor stage -t lib/main.dart --dart-define=FLUTTER_FLAVOR=stage $(DEFINES_FLAG) --release

# ============================================================
# 生產環境 (prod)
# ============================================================

prod-debug: require-gateway
	@echo "Running prod environment (Debug)..."
	$(FLUTTER) run --flavor prod -t lib/main.dart --dart-define=FLUTTER_FLAVOR=prod $(DEFINES_FLAG)

prod-profile: require-gateway
	@echo "Running prod environment (Profile)..."
	$(FLUTTER) run --flavor prod -t lib/main.dart --dart-define=FLUTTER_FLAVOR=prod $(DEFINES_FLAG) --profile

prod-release: require-gateway
	@echo "Running prod environment (Release)..."
	$(FLUTTER) run --flavor prod -t lib/main.dart --dart-define=FLUTTER_FLAVOR=prod $(DEFINES_FLAG) --release

# ============================================================
# iOS 構建命令
# ============================================================

build-ios-dev:
	@echo "Building iOS (dev)..."
	$(FLUTTER) build ios --flavor dev -t lib/main.dart --dart-define=FLUTTER_FLAVOR=dev $(DEFINES_FLAG) --release

build-ios-stage: require-gateway
	@echo "Building iOS (stage)..."
	$(FLUTTER) build ios --flavor stage -t lib/main.dart --dart-define=FLUTTER_FLAVOR=stage $(DEFINES_FLAG) --release

build-ios-prod: require-gateway
	@echo "Building iOS (prod)..."
	$(FLUTTER) build ios --flavor prod -t lib/main.dart --dart-define=FLUTTER_FLAVOR=prod $(DEFINES_FLAG) --release

build-ios-ipa-dev:
	@echo "Building iOS IPA (dev)..."
	$(FLUTTER) build ipa --flavor dev -t lib/main.dart --dart-define=FLUTTER_FLAVOR=dev $(DEFINES_FLAG)

build-ios-ipa-stage: require-gateway
	@echo "Building iOS IPA (stage)..."
	$(FLUTTER) build ipa --flavor stage -t lib/main.dart --dart-define=FLUTTER_FLAVOR=stage $(DEFINES_FLAG)

build-ios-ipa-prod: require-gateway
	@echo "Building iOS IPA (prod)..."
	$(FLUTTER) build ipa --flavor prod -t lib/main.dart --dart-define=FLUTTER_FLAVOR=prod $(DEFINES_FLAG)

# stage 與 prod 必須有 GATEWAY_HOST（API 主機是必要設定，沒有預設值）；缺少時拒絕建置。
# 檔案裡只看 GATEWAY_HOST 一行是否有非空值。
require-gateway:
	@if [ -z "$(GATEWAY_HOST)" ] && ! grep -Eq '"GATEWAY_HOST"[[:space:]]*:[[:space:]]*"[^"]+"' "$(DEFINES_FILE)" 2>/dev/null; then \
		echo "error: stage 與 prod 必須提供 GATEWAY_HOST：在 $(DEFINES_FILE) 填入，或 make <target> GATEWAY_HOST=<API 主機>" >&2; \
		exit 1; \
	fi

# ============================================================
# 清理命令
# ============================================================

clean:
	@echo "Cleaning Flutter build cache..."
	$(FLUTTER) clean
	@echo "Getting dependencies..."
	$(FLUTTER) pub get
	@echo "Clean completed!"

clean-ios:
	@echo "Cleaning iOS DerivedData..."
	rm -rf ~/Library/Developer/Xcode/DerivedData/Runner-*
	@echo "iOS DerivedData cleaned!"

clean-reinstall:
	@echo "Full clean and reinstall all dependencies..."
	@echo "Cleaning Flutter build cache..."
	$(FLUTTER) clean
	@echo "Getting Flutter dependencies..."
	$(FLUTTER) pub get
	@echo "Deintegrating CocoaPods..."
	cd ios && pod deintegrate || true
	@echo "Removing old CocoaPods files..."
	cd ios && rm -rf Pods Podfile.lock .symlinks
	@echo "Installing CocoaPods dependencies..."
	cd ios && pod install
	@echo "All dependencies reinstalled successfully!"
	@echo "Please run Product > Clean Build Folder (Shift+Command+K) in Xcode"

# ============================================================
# 其他實用命令
# ============================================================

# 分析代碼
analyze:
	@echo "Analyzing code..."
	$(FLUTTER) analyze

# 格式化代碼
format:
	@echo "Formatting code..."
	$(DART) format lib test integration_test tool

# 運行測試
test:
	@echo "Running tests..."
	$(FLUTTER) test

# 守門：不得手寫 proto enum 的文字表（共用文案目錄 packages/hawkr_ui_text；設計 step10e-Lprime-design.md §5.2）
enum-text:
	@echo "Checking hand-written proto enum text tables..."
	cd ../packages/hawkr_ui_text && $(FLUTTER) pub get && $(DART) run tool/check_enum_text.dart ../../seller-app/lib

# 檢查依賴更新
check-updates:
	@echo "Checking dependency updates..."
	$(FLUTTER) pub outdated

# ============================================================
# 死碼/未用依賴掃描（類 Knip）
#   - dependency_validator：未使用的 dependencies（白名單見 dart_dependency_validator.yaml）
#   - dart_code_linter：未使用的檔案 + 未使用的 exports/code
#     check-unused-code 不可加 --exclude generated，否則 riverpod provider source
#     會因看不到 .g.dart 的引用而被誤報；改在輸出端濾掉 generated 檔的噪音
#     （freezed 3.0 的 XxxPatterns extension、riverpod 4.x 的 provider 變數等；
#     Riverpod 2.x 時代是 XxxRef typedef）
# ============================================================
deadcode:
	@echo "== 未使用的 dependencies =="
	@$(DART) run dependency_validator || true
	@echo ""
	@echo "== 未使用的檔案 =="
	@$(DART) run dart_code_linter:metrics check-unused-files lib 2>/dev/null | grep -E 'unused file|total unused|no unused' || echo "  none"
	@echo ""
	@echo "== 未使用的 exports/code（已濾除 generated 檔的噪音）=="
	@$(DART) run dart_code_linter:metrics check-unused-code lib 2>/dev/null \
		| grep -E '⚠|at /' | paste - - \
		| grep -vE '\.g\.dart:|\.freezed\.dart:' \
		| sed -E 's#\t.*/lib/#  →  lib/#; s#:[0-9]+:[0-9]+$$##' \
		|| echo "  none"
