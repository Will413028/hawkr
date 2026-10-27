#!/usr/bin/env bash
# 監控棧的外部 watchdog 探測。
#
# 解決的問題：Prometheus 與 Alertmanager 自己死掉時沒有東西能報告——前者死了
# 規則不會被評估，後者死了告警送不出去。2026-09-10／09-11 補的 up series 讓監控棧
# 其他元件被 ServiceDown 蓋到，這兩個是自我涵蓋不了的（見 prometheus.yml 該段註解）。
#
# 三道檢查，由外而內：
#   1. Prometheus /-/healthy        —— process 活著
#   2. Alertmanager /-/healthy      —— process 活著
#   3. Alertmanager 上有 Watchdog   —— **規則評估管線活著且告警送得到 Alertmanager**
#      這一道才是重點。前兩道只證明 process 沒死；第三道證明整條
#      「Prometheus 評估規則 → 送到 Alertmanager」還通。Watchdog 是
#      prometheus/rules/watchdog.yml 的 `vector(1)`，恆為 firing。
#      Prometheus 停止評估後，Alertmanager 上的它會在 resolve_timeout（5m）內過期消失
#      ⇒ 這道檢查的偵測延遲上限是那個 timeout，不是即時。
#
# 刻意不檢查「告警有沒有送到 Discord」：那要 Discord 端的回饋，本探測拿不到。
# 該段由 discord-webhook-gateway 的 forward_failures_total ＋
# infra-alerts.yml 的 DiscordWebhookForwardFailing 承接。
#
# 用法：PROM_URL=http://host:9091 AM_URL=http://host:9093 bash watchdog-probe.sh
set -uo pipefail

PROM_URL="${PROM_URL:?PROM_URL required（例：http://localhost:9091）}"
AM_URL="${AM_URL:?AM_URL required（例：http://localhost:9093）}"
ATTEMPTS="${ATTEMPTS:-3}"
RETRY_SLEEP="${RETRY_SLEEP:-5}"
CURL_TIMEOUT="${CURL_TIMEOUT:-10}"

fail() { echo "🔴 WATCHDOG FAIL: $*" >&2; exit 1; }

# 重試存在的理由：runner 到監控 VM 之間的一次網路抖動不該變成一則假告警。
# 但重試次數要有限——「一直重試到成功」等於這道檢查永遠不會紅。
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

# 第三道：Watchdog 在不在。
for ((i = 1; i <= ATTEMPTS; i++)); do
  # 必須自己驗狀態碼：`curl -sS` 對 HTTP 500 一樣回 exit 0（只是把錯誤 body 印出來），
  # 只看 exit code 的話「Alertmanager 回 500 但 body 剛好含 Watchdog」會被誤判成健康。
  # 這個洞是本腳本的自我測試抓到的（test-watchdog-probe.sh「alerts API 回 500」那格）。
  resp="$(curl -sS --max-time "$CURL_TIMEOUT" -w $'\n%{http_code}' "$AM_URL/api/v2/alerts?active=true" 2>/dev/null)"
  code="${resp##*$'\n'}"
  body="${resp%$'\n'*}"
  if [[ "$code" == "200" && -n "$body" ]]; then
    # 不用 jq：runner 上不保證有。grep 字面 alertname 即可——
    # 誤判方向是「有別的告警剛好叫 Watchdog」，那不會發生（本 repo 只有一條）。
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
