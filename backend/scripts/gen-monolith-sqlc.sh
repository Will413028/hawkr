#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
image="${1:?sqlc image required}"
stage="$(mktemp -d "$root/.sqlc-stage.XXXXXX")"
trap 'rm -rf "$stage"' EXIT
for db in "$root"/internal/modules/*/db; do
  [ -d "$db" ] || continue
  module="$(basename "$(dirname "$db")")"
  mkdir -p "$stage/$module"
  cp -R "$db" "$stage/$module/db"
done
for db in "$stage"/*/db; do
  [ -f "$db/sqlc.yaml" ] || continue
  compgen -G "$db/queries/*.sql" >/dev/null || continue
  module="$(basename "$(dirname "$db")")"
  docker run --rm -u "$(id -u):$(id -g)" -v "$stage:/work" -w "/work/$module/db" "$image" generate
done
for generated in "$stage"/*/internal/store; do
  [ -d "$generated" ] || continue
  module="$(basename "$(dirname "$(dirname "$generated")")")"
  target="$root/internal/modules/$module/internal/store"
  rm -rf "$target"
  cp -R "$generated" "$target"
done
