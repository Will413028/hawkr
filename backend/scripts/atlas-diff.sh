#!/bin/bash
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
	docker exec "$container" psql -U postgres -tAc 'SELECT 1' >/dev/null 2>&1 && break
	sleep 1
done
port="$(docker port "$container" 5432/tcp | head -1 | sed 's/.*://')"

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
