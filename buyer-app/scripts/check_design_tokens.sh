#!/usr/bin/env bash
# 檢查 staged Dart 檔的「新增行」是否含設計 token 違規。
# 只檢查新增（+）的行，不會對既有違規誤報。
#
# 用法：
#   scripts/check_design_tokens.sh                    # 檢查所有 staged 變更
#   scripts/check_design_tokens.sh path/a.dart b.dart # 檢查指定檔案
#   ALLOW_LEGACY=1 scripts/check_design_tokens.sh     # 暫時放行（不建議常用）
#
# 退出碼：
#   0 = 沒有違規
#   1 = 偵測到違規

set -uo pipefail

# ── 跳過邏輯 ────────────────────────────────────────────────
if [[ "${ALLOW_LEGACY:-0}" == "1" ]]; then
  echo "[design-tokens] ALLOW_LEGACY=1 → 跳過檢查"
  exit 0
fi

# ── 取得待檢查檔案（macOS bash 3.2 相容，不用 mapfile）────
files=()
if [[ $# -gt 0 ]]; then
  files=("$@")
else
  while IFS= read -r f; do
    files+=("$f")
  done < <(git diff --cached --name-only --diff-filter=ACM 2>/dev/null | grep -E '\.dart$' || true)
fi

if [[ ${#files[@]} -eq 0 ]]; then
  exit 0
fi

# ── 例外清單（這些檔案內部允許硬編碼，因為它們 *是* token 定義）────
is_token_file() {
  case "$1" in
    *lib/core/theme/*) return 0 ;;
    *.g.dart) return 0 ;;
    *.freezed.dart) return 0 ;;
    *.gr.dart) return 0 ;;
    *) return 1 ;;
  esac
}

violations=0
report_violation() {
  local file=$1 line=$2 pattern=$3 suggestion=$4 context=$5
  echo ""
  echo "  ✗ $file:$line"
  echo "    違反：$pattern"
  echo "    建議：$suggestion"
  echo "    內容：$context"
  violations=$((violations + 1))
}

# ── 主檢查迴圈 ─────────────────────────────────────────────
for file in "${files[@]}"; do
  [[ ! -f "$file" ]] && continue
  is_token_file "$file" && continue

  # 取得 staged diff 的「新增行」與行號
  # diff -U0 只顯示變更，不顯示 context
  while IFS= read -r line; do
    # 解析 hunk header 取起始行號：@@ -1,3 +5,7 @@
    if [[ "$line" =~ ^@@\ -[0-9,]+\ \+([0-9]+) ]]; then
      current_line=${BASH_REMATCH[1]}
      continue
    fi
    # 跳過非新增行
    [[ "$line" =~ ^\+\+\+ ]] && continue
    [[ ! "$line" =~ ^\+ ]] && continue

    content="${line:1}"  # 去掉前綴 +

    # 1. 硬編碼顏色 Color(0x...)
    if echo "$content" | grep -qE 'Color\(\s*0x[0-9A-Fa-f]{6,8}'; then
      report_violation "$file" "$current_line" \
        "硬編碼顏色 Color(0x...)" \
        "改用 AppColors.* （參考 lib/core/theme/app_colors.dart）" \
        "$(echo "$content" | sed 's/^[[:space:]]*//')"
    fi

    # 2. 硬編碼字級 fontSize: <number>
    if echo "$content" | grep -qE 'fontSize:\s*[0-9]'; then
      report_violation "$file" "$current_line" \
        "硬編碼 fontSize" \
        "改用 AppTypography.* （body / sectionTitle / h1 / display 等）" \
        "$(echo "$content" | sed 's/^[[:space:]]*//')"
    fi

    # 3. 硬編碼 fontFamily: 'Roboto' / "Roboto"
    if echo "$content" | grep -qE "fontFamily:\s*['\"]Roboto['\"]"; then
      report_violation "$file" "$current_line" \
        "硬編碼 fontFamily: 'Roboto'" \
        "改用 AppTypography.family（避免重複宣告）" \
        "$(echo "$content" | sed 's/^[[:space:]]*//')"
    fi

    # 4. FontWeight 規範外
    if echo "$content" | grep -qE 'FontWeight\.(w500|w600|w800|w900)'; then
      offending=$(echo "$content" | grep -oE 'FontWeight\.(w500|w600|w800|w900)' | head -1)
      report_violation "$file" "$current_line" \
        "FontWeight 規範外：$offending" \
        "只允許 w400 / w700。需中間階層請改變顏色或大小" \
        "$(echo "$content" | sed 's/^[[:space:]]*//')"
    fi

    # 行號遞增（每跑完一個 + 行）
    current_line=$((current_line + 1))
  done < <(git diff --cached -U0 -- "$file" 2>/dev/null)
done

# ── 輸出總結 ────────────────────────────────────────────────
if [[ $violations -gt 0 ]]; then
  echo ""
  echo "═══════════════════════════════════════════════════════════════"
  echo "  ✗ 偵測到 $violations 個設計 token 違規"
  echo "═══════════════════════════════════════════════════════════════"
  echo ""
  echo "規範參考：openspec/specs/core-design-tokens/DESIGN_SYSTEM_v1.md"
  echo "Token 檔： lib/core/theme/{app_colors,app_typography,app_spacing,app_radius,app_elevation}.dart"
  echo ""
  echo "緊急放行（不建議）：ALLOW_LEGACY=1 git commit ..."
  exit 1
fi

exit 0
