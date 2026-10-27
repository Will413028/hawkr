#!/bin/bash
# 建立 Tempo datasource（非 provisioned，讓 trace-to-logs 連結能正常顯示）
# 詳見 TROUBLESHOOTING.md #14

set -e

GRAFANA_URL="${GRAFANA_URL:-http://127.0.0.1:3000}"
GRAFANA_USER="${GF_SECURITY_ADMIN_USER:-admin}"
GRAFANA_PASS="${GF_SECURITY_ADMIN_PASSWORD:?請設定 GF_SECURITY_ADMIN_PASSWORD}"

# 等待 Grafana 啟動
echo "等待 Grafana 啟動..."
for i in $(seq 1 30); do
  if curl -s -o /dev/null -w '%{http_code}' "$GRAFANA_URL/api/health" | grep -q 200; then
    echo "Grafana 已啟動"
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "Grafana 啟動超時" >&2
    exit 1
  fi
  sleep 2
done

# 取得 Loki datasource UID
LOKI_UID=$(curl -s -u "$GRAFANA_USER:$GRAFANA_PASS" "$GRAFANA_URL/api/datasources/name/Loki" | python3 -c "import sys,json; print(json.load(sys.stdin)['uid'])")
echo "Loki UID: $LOKI_UID"

# 檢查 Tempo 是否已存在
HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' -u "$GRAFANA_USER:$GRAFANA_PASS" "$GRAFANA_URL/api/datasources/name/Tempo")
if [ "$HTTP_CODE" = "200" ]; then
  echo "Tempo datasource 已存在，跳過"
  exit 0
fi

# 建立 Tempo datasource
python3 -c "
import urllib.request, json, base64

credentials = base64.b64encode('$GRAFANA_USER:$GRAFANA_PASS'.encode()).decode()
headers = {'Content-Type': 'application/json', 'Authorization': f'Basic {credentials}'}

payload = {
    'name': 'Tempo',
    'type': 'tempo',
    'access': 'proxy',
    'url': 'http://tempo:3200',
    'uid': 'tempo',
    'jsonData': {
        'httpMethod': 'GET',
        'serviceMap': {'datasourceUid': 'prometheus'},
        'nodeGraph': {'enabled': True},
        'search': {'hide': False},
        'lokiSearch': {'datasourceUid': '$LOKI_UID'},
        'tracesToLogsV2': {
            'datasourceUid': '$LOKI_UID',
            'spanStartTimeShift': '-5m',
            'spanEndTimeShift': '5m',
            'filterByTraceID': True,
            'filterBySpanID': False,
            'tags': [{'key': 'service.name', 'value': 'service_name'}]
        }
    }
}

req = urllib.request.Request(
    '$GRAFANA_URL/api/datasources',
    data=json.dumps(payload).encode(),
    headers=headers,
    method='POST'
)
with urllib.request.urlopen(req) as resp:
    print('Tempo datasource 建立成功')
"

echo "完成"
