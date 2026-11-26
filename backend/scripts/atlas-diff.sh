#!/bin/bash
# 為模組產生 migration：起一個暫時的 Postgres（版本與 platform/testdb 相同）當 Atlas 的 dev database，
# 先套用平台擁有的 extension（internal/platform/migrate 的 Extensions，與執行期 Apply 同一份），再跑
# `atlas migrate diff`。用法（在 backend 目錄）：make atlas-diff module=<模組> name=<migration 名稱>
set -euo pipefail

module="${1:?用法：scripts/atlas-diff.sh <模組> <名稱>}"
name="${2:?用法：scripts/atlas-diff.sh <模組> <名稱>}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dir="$root/internal/modules/$module/db"
[ -f "$dir/atlas.hcl" ] || { echo "error: $dir/atlas.hcl 不存在" >&2; exit 1; }

container="atlasdev-$module-$$"
cleanup() { docker rm -f "$container" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run -d --rm --name "$container" -e POSTGRES_PASSWORD=atlas -p 127.0.0.1::5432 postgres:18-alpine >/dev/null
for _ in $(seq 60); do
	# initdb 的暫時 server 也會回應 pg_isready，所以另外確認查得到資料
	docker exec "$container" psql -U postgres -tAc 'SELECT 1' >/dev/null 2>&1 && break
	sleep 1
done
port="$(docker port "$container" 5432/tcp | head -1 | sed 's/.*://')"

# Atlas 在讀完 desired state 之後、重播 migration 之前會 DROP SCHEMA public CASCADE 再重建，extension 會跟著消失
# （社群版沒有 docker block 的 baseline）。所以除了先裝一次，還掛一個 event trigger：每次 CREATE SCHEMA（包括 Atlas 重建 public）
# 之後重新安裝同一份清單，desired state 與重播的 migration 都看得到 extension。函式放在 information_schema：
# Atlas 不管那個 schema，也不會隨 public 被 DROP CASCADE。
ext_sql="$(cd "$root" && GOWORK=off go run ./cmd/atlas-extensions)"
docker exec -i "$container" psql -U postgres -v ON_ERROR_STOP=1 -q <<SQL
$ext_sql
CREATE FUNCTION information_schema.atlas_dev_install_extensions() RETURNS event_trigger LANGUAGE plpgsql AS \$fn\$
BEGIN
$(printf '%s\n' "$ext_sql" | sed "s/^/  EXECUTE '/; s/;\$/';/")
END
\$fn\$;
CREATE EVENT TRIGGER atlas_dev_install_extensions ON ddl_command_end WHEN TAG IN ('CREATE SCHEMA') EXECUTE FUNCTION information_schema.atlas_dev_install_extensions();
SQL
cd "$dir"
atlas migrate diff "$name" --env dev --dev-url "postgres://postgres:atlas@127.0.0.1:$port/postgres?sslmode=disable"
