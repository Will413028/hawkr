#!/usr/bin/env bash
set -uo pipefail

PROM_URL="${PROM_URL:?PROM_URL required（例：http://localhost:9091）}"
AM_URL="${AM_URL:?AM_URL required（例：http://localhost:9093）}"
ATTEMPTS="${ATTEMPTS:-3}"
RETRY_SLEEP="${RETRY_SLEEP:-5}"
CURL_TIMEOUT="${CURL_TIMEOUT:-10}"

fail() { echo "🔴 WATCHDOG FAIL: $*" >&2; exit 1; }

retry_until_ok() {
  local desc="$1" url="$2" i out code
  for ((i = 1; i <= ATTEMPTS; i++)); do
    code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time "$CURL_TIMEOUT" "$url" 2>/dev/null || echo 000)"
    if [[ "$code" == "200" ]]; then
      echo "  ✅ ${desc}（HTTP 200，第 $i 次）"
      return 0
    fi
    echo "  ⏳ $desc 第 $i/$ATTEMPTS 次得到 HTTP $code"
    [[ $i -lt $ATTEMPTS ]] && sleep "$RETRY_SLEEP"
  done
  fail "$desc 連續 $ATTEMPTS 次非 200（最後一次 HTTP ${code}，url=${url}）"
}

retry_until_ok "Prometheus /-/healthy" "$PROM_URL/-/healthy"
retry_until_ok "Alertmanager /-/healthy" "$AM_URL/-/healthy"

for ((i = 1; i <= ATTEMPTS; i++)); do
  resp="$(curl -sS --max-time "$CURL_TIMEOUT" -w $'\n%{http_code}' "$AM_URL/api/v2/alerts?active=true" 2>/dev/null)"
  code="${resp##*$'\n'}"
  body="${resp%$'\n'*}"
  if [[ "$code" == "200" && -n "$body" ]]; then
    if grep -q '"alertname":"Watchdog"' <<<"$body"; then
      echo "  ✅ Alertmanager 上找得到 Watchdog（規則評估管線活著）"
      echo "🟢 WATCHDOG OK"
      exit 0
    fi
    fail "Alertmanager 活著但**查不到 Watchdog 告警**——Prometheus 規則評估停止，或告警送不到 Alertmanager。監控棧看起來活著但實際上不會發出任何告警。"
  fi
  echo "  ⏳ 讀 Alertmanager alerts API 第 $i/$ATTEMPTS 次失敗（HTTP ${code:-000}）"
  [[ $i -lt $ATTEMPTS ]] && sleep "$RETRY_SLEEP"
done
fail "連續 $ATTEMPTS 次讀不到 Alertmanager 的 alerts API（url=$AM_URL/api/v2/alerts）"
