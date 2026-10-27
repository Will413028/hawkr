#!/usr/bin/env bash
# watchdog-probe.sh 的自我測試。
#
# 為什麼需要：這道探測的價值全部來自「該紅的時候真的會紅」。它平常永遠綠
# （監控棧正常時三道檢查都通過），所以「它已經失去鑑別力」跟「一切正常」
# 在 CI 上長得一模一樣——正是這支腳本本身要擋的失效形狀。
#
# 做法：起一個假的 HTTP server，用環境變數控制它對三個路徑各自回什麼，
# 逐個情境驗證 probe 的 exit code 與訊息。不連任何真實服務。
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROBE="$HERE/watchdog-probe.sh"
[[ -x "$PROBE" ]] || { echo "找不到或不可執行：$PROBE"; exit 2; }

TMP="$(mktemp -d)"
stop_srv() { [[ -n "${SRV_PID:-}" ]] && kill "$SRV_PID" 2>/dev/null; wait "${SRV_PID:-}" 2>/dev/null; SRV_PID=""; }
trap 'stop_srv; rm -rf "$TMP"' EXIT

cat > "$TMP/fake.py" <<'PYEOF'
import os, sys
from http.server import BaseHTTPRequestHandler, HTTPServer

PROM_HEALTH = int(os.environ.get("FAKE_PROM_HEALTH", "200"))
AM_HEALTH = int(os.environ.get("FAKE_AM_HEALTH", "200"))
AM_ALERTS = os.environ.get("FAKE_AM_ALERTS", '[{"labels":{"alertname":"Watchdog"}}]')
AM_ALERTS_CODE = int(os.environ.get("FAKE_AM_ALERTS_CODE", "200"))

class H(BaseHTTPRequestHandler):
    def do_GET(self):
        # /prom/-/healthy 與 /am/-/healthy 用前綴區分，一個 server 兼兩個角色
        if self.path.startswith("/prom/-/healthy"):
            self.send_response(PROM_HEALTH); self.end_headers(); self.wfile.write(b"ok")
        elif self.path.startswith("/am/-/healthy"):
            self.send_response(AM_HEALTH); self.end_headers(); self.wfile.write(b"ok")
        elif self.path.startswith("/am/api/v2/alerts"):
            self.send_response(AM_ALERTS_CODE); self.end_headers()
            self.wfile.write(AM_ALERTS.encode())
        else:
            self.send_response(404); self.end_headers()
    def log_message(self, *a): pass

srv = HTTPServer(("127.0.0.1", 0), H)
# 綁 0 讓 OS 給一個空閒埠，再把它寫給呼叫端。
# 寫死埠號在 self-hosted runner 上會跟並行的另一個 job 撞——
# 那台 runner 反覆踩過「並行 run 共用固定路徑/資源」這類 bug。
with open(sys.argv[1], "w") as f:
    f.write(str(srv.server_address[1]))
srv.serve_forever()
PYEOF

pass=0; fail=0
run_case() {
  local desc="$1" want_rc="$2" want_msg="$3"; shift 3
  stop_srv
  : > "$TMP/port"
  env "$@" python3 "$TMP/fake.py" "$TMP/port" & SRV_PID=$!
  local port=""
  for _ in $(seq 1 50); do
    port="$(cat "$TMP/port" 2>/dev/null)"
    [[ -n "$port" ]] && curl -sS -o /dev/null --max-time 1 "http://127.0.0.1:$port/prom/-/healthy" 2>/dev/null && break
    sleep 0.2
  done
  [[ -n "$port" ]] || { echo "  ❌ $desc — 假 server 沒起來"; fail=$((fail+1)); return; }
  out="$(PROM_URL="http://127.0.0.1:$port/prom" AM_URL="http://127.0.0.1:$port/am" \
         ATTEMPTS=1 RETRY_SLEEP=0 CURL_TIMEOUT=3 bash "$PROBE" 2>&1)"
  rc=$?
  if [[ "$rc" == "$want_rc" ]] && grep -q "$want_msg" <<<"$out"; then
    echo "  ✅ $desc"; pass=$((pass+1))
  else
    echo "  ❌ $desc — 期望 rc=$want_rc 且訊息含 '$want_msg'，實得 rc=$rc"
    sed 's/^/       /' <<<"$out"; fail=$((fail+1))
  fi
}

echo "── watchdog-probe.sh 自我測試 ──"
run_case "全部健康 → 綠" 0 "WATCHDOG OK"
run_case "Prometheus 不健康 → 紅且指名 Prometheus" 1 "Prometheus /-/healthy" FAKE_PROM_HEALTH=503
run_case "Alertmanager 不健康 → 紅且指名 Alertmanager" 1 "Alertmanager /-/healthy" FAKE_AM_HEALTH=503
run_case "Alertmanager 活著但沒有 Watchdog → 紅（本檢查的核心）" 1 "查不到 Watchdog" FAKE_AM_ALERTS='[{"labels":{"alertname":"SomethingElse"}}]'
run_case "alerts API 回 500 → 紅" 1 "讀不到 Alertmanager 的 alerts API" FAKE_AM_ALERTS_CODE=500
# 空的 alert 清單與「有別的告警但沒有 Watchdog」是同一種失效，要分別驗到：
# 監控棧靜默時 Alertmanager 常常是空的，那也必須紅。
run_case "alerts 清單為空 → 紅" 1 "查不到 Watchdog" FAKE_AM_ALERTS='[]'

echo "── $pass 通過／$fail 失敗 ──"
[[ $fail -eq 0 ]]
