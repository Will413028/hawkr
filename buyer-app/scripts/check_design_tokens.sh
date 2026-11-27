#!/usr/bin/env bash

set -uo pipefail

if [[ "${ALLOW_LEGACY:-0}" == "1" ]]; then
  echo "[design-tokens] ALLOW_LEGACY=1 → 跳過檢查"
  exit 0
fi

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

for file in "${files[@]}"; do
  [[ ! -f "$file" ]] && continue
  is_token_file "$file" && continue

  while IFS= read -r line; do
    if [[ "$line" =~ ^@@\ -[0-9,]+\ \+([0-9]+) ]]; then
      current_line=${BASH_REMATCH[1]}
      continue
    fi
    [[ "$line" =~ ^\+\+\+ ]] && continue
    [[ ! "$line" =~ ^\+ ]] && continue

    content="${line:1}"

    if echo "$content" | grep -qE 'Color\(\s*0x[0-9A-Fa-f]{6,8}'; then
      report_violation "$file" "$current_line" \
        "硬編碼顏色 Color(0x...)" \
        "改用 AppColors.* （參考 lib/core/theme/app_colors.dart）" \
        "$(echo "$content" | sed 's/^[[:space:]]*//')"
    fi

    if echo "$content" | grep -qE 'fontSize:\s*[0-9]'; then
      report_violation "$file" "$current_line" \
        "硬編碼 fontSize" \
        "改用 AppTypography.* （body / sectionTitle / h1 / display 等）" \
        "$(echo "$content" | sed 's/^[[:space:]]*//')"
    fi

    if echo "$content" | grep -qE "fontFamily:\s*['\"]Roboto['\"]"; then
      report_violation "$file" "$current_line" \
        "硬編碼 fontFamily: 'Roboto'" \
        "改用 AppTypography.family（避免重複宣告）" \
        "$(echo "$content" | sed 's/^[[:space:]]*//')"
    fi

    if echo "$content" | grep -qE 'FontWeight\.(w500|w600|w800|w900)'; then
      offending=$(echo "$content" | grep -oE 'FontWeight\.(w500|w600|w800|w900)' | head -1)
      report_violation "$file" "$current_line" \
        "FontWeight 規範外：$offending" \
        "只允許 w400 / w700。需中間階層請改變顏色或大小" \
        "$(echo "$content" | sed 's/^[[:space:]]*//')"
    fi

    current_line=$((current_line + 1))
  done < <(git diff --cached -U0 -- "$file" 2>/dev/null)
done

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
