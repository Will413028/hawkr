#!/usr/bin/env bash
# Keep each Flutter test invocation outside the VM-wide /tmp namespace.
set -euo pipefail

if [[ "${RUNNER_TEMP:-}" != /* || ! -d "$RUNNER_TEMP" ]]; then
  echo 'Flutter test requires an existing absolute RUNNER_TEMP directory.' >&2
  exit 2
fi

flutter_test_tmp="$(mktemp -d "${RUNNER_TEMP%/}/flutter-test.XXXXXX")"
export TMPDIR="$flutter_test_tmp"
printf 'Flutter test TMPDIR: %s\n' "$TMPDIR"

# GitHub empties RUNNER_TEMP at job boundaries. Leave this owned leaf to that
# cleanup and replace Bash so supervisor signals reach Flutter's original PID.
# https://docs.github.com/en/actions/reference/workflows-and-actions/variables
exec flutter test "$@"
