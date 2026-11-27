#!/usr/bin/env bash
set -euo pipefail

if [[ "${RUNNER_TEMP:-}" != /* || ! -d "$RUNNER_TEMP" ]]; then
  echo 'Flutter test requires an existing absolute RUNNER_TEMP directory.' >&2
  exit 2
fi

flutter_test_tmp="$(mktemp -d "${RUNNER_TEMP%/}/flutter-test.XXXXXX")"
export TMPDIR="$flutter_test_tmp"
printf 'Flutter test TMPDIR: %s\n' "$TMPDIR"

exec flutter test "$@"
