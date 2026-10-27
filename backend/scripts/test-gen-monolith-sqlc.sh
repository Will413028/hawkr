#!/usr/bin/env bash
set -euo pipefail
script="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/gen-monolith-sqlc.sh"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/scripts" "$fixture/bin" "$fixture/internal/modules/test/db/queries" "$fixture/internal/modules/test/internal/store"
cp "$script" "$fixture/scripts/"
printf 'version: "2"\n' > "$fixture/internal/modules/test/db/sqlc.yaml"
printf 'SELECT 1;\n' > "$fixture/internal/modules/test/db/queries/test.sql"
printf 'original generated code\n' > "$fixture/internal/modules/test/internal/store/unchanged.go"
cp "$fixture/internal/modules/test/internal/store/unchanged.go" "$fixture/expected"
cat > "$fixture/bin/docker" <<'EOF'
#!/usr/bin/env bash
for ((i = 1; i <= $#; i++)); do
  if [ "${!i}" = '-v' ]; then
    next=$((i + 1))
    [[ "${!next}" == "$FAKE_SQLC_ROOT"/.sqlc-stage.*:/work ]] || exit 2
    exit 1
  fi
done
exit 2
EOF
chmod +x "$fixture/bin/docker"
result=0
FAKE_SQLC_ROOT="$fixture" PATH="$fixture/bin:$PATH" bash "$fixture/scripts/gen-monolith-sqlc.sh" fake-image || result=$?
if [ "$result" -ne 1 ]; then
  echo "unexpected generation result: $result" >&2
  exit 1
fi
cmp "$fixture/expected" "$fixture/internal/modules/test/internal/store/unchanged.go"
if compgen -G "$fixture/.sqlc-stage.*" >/dev/null; then
  echo 'staging directory was not removed' >&2
  exit 1
fi
echo 'sqlc generation failure preserves store: PASS'
