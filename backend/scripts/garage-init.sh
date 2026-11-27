#!/usr/bin/env bash
set -euo pipefail

compose=(docker compose -f docker-compose-infra.yml)
garage() { "${compose[@]}" exec -T garage /garage "$@"; }

bucket="${MEDIA_S3_BUCKET:-media-local}"
env_file="${MEDIA_GARAGE_ENV_FILE:-.dev/garage.env}"
keks_file="${MEDIA_KEKS_FILE:-.dev/media-keks}"

mkdir -p "$(dirname "$env_file")" "$(dirname "$keks_file")"
if [ ! -s "$env_file" ]; then
  umask 077
  printf 'MEDIA_S3_ACCESS_KEY_ID=GK%s\nMEDIA_S3_SECRET_ACCESS_KEY=%s\n' "$(openssl rand -hex 12)" "$(openssl rand -hex 32)" >"$env_file"
fi
if [ ! -s "$keks_file" ]; then
  umask 077
  printf 'dev1:%s\n' "$(openssl rand -base64 32)" >"$keks_file"
fi
chmod 600 "$env_file" "$keks_file"
key_id="$(sed -n 's/^MEDIA_S3_ACCESS_KEY_ID=//p' "$env_file")"
secret="$(sed -n 's/^MEDIA_S3_SECRET_ACCESS_KEY=//p' "$env_file")"
key_name="media-local-key"

if garage status | grep -q 'NO ROLE ASSIGNED'; then
  node="$(garage node id -q | grep -Eo '^[0-9a-f]{64}')"
  garage layout assign -z dc1 -c 1G "$node"
  garage layout apply --version 1
fi

if ! garage key info "$key_name" >/dev/null 2>&1; then
  garage key import --yes -n "$key_name" "$key_id" "$secret"
fi

if ! garage bucket info "$bucket" >/dev/null 2>&1; then
  garage bucket create "$bucket"
fi
garage bucket allow --read --write --owner "$bucket" --key "$key_name"
