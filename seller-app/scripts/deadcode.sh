#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

if command -v fvm >/dev/null 2>&1; then
  DART=(fvm dart)
else
  DART=(dart)
fi

echo "== Dependency validator =="
"${DART[@]}" run dependency_validator

echo
echo "== Unused production files (gating) =="
set +e
unused_files=$("${DART[@]}" run dart_code_linter:metrics check-unused-files lib 2>&1)
unused_status=$?
set -e
printf '%s\n' "$unused_files"
if printf '%s\n' "$unused_files" | grep -q '⚠ unused file:'; then
  echo "Unused production files are not allowed. Delete them or document a real runtime entrypoint." >&2
  exit 1
fi
if [[ $unused_status -ne 0 ]]; then
  echo "Unused-file scanner failed without producing actionable findings." >&2
  exit "$unused_status"
fi

echo
echo "== Unused symbols (generated-code false positives filtered) =="
set +e
unused_code=$("${DART[@]}" run dart_code_linter:metrics check-unused-code lib 2>&1)
set -e
printf '%s\n' "$unused_code" | awk '
  /^lib\/.*:$/ { file=$0 }
  /⚠ unused/ {
    symbol=$0
    getline
    location=$0
    if (file !~ /\.(g|freezed)\.dart:$/) {
      print file
      print symbol
      print location
    }
  }
'
echo "Named exceptions and removal conditions: tool/hygiene_policy.json"

echo
echo "== Asset/dependency/documentation ratchet =="
"${DART[@]}" run tool/hygiene_check.dart
