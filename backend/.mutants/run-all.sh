#!/usr/bin/env bash
# Throwaway: DB-level mutation proofs for A6-1b. Run from the repo root.
set -u
PKG=./internal/modules/review/...
# json:TestRegex
MUTANTS=(
  "rd1:TestRD1ViewerBlocksFilterBeforeThePageIsCut"
  "rd2:TestRD2LengthIsCountedInRunes"
  "rd3:TestRD3HideCanBeRestoredAndHiddenAgain"
  "rd6:TestRD6SnapshotComesFromTheOrderItem"
  "rd7:TestRD7AtMostFourImages"
  "rd9:TestRD9OrderOutageIsUnavailableNotNotFound"
  "rd10:TestRD10SingleReadUsesThePublicPredicate"
  "f2:TestReviewImageAssetIsUniquePerReview"
  "f3:TestReplyWaitsForAConcurrentHide"
  "f6:TestRestoreNeverPrecedesHiddenAt"
)
bad=0
summary=()
for m in "${MUTANTS[@]}"; do
  id=${m%%:*}; test=${m#*:}
  echo "=== $id ($test)"
  python3 backend/.mutants/red-proof.py --repo . --cwd backend --timeout 900 \
    --edits "backend/.mutants/$id.json" --expect "$test" -- \
    env GOWORK=off go test -tags integration "$PKG" -run "$test" -count=1
  rc=$?
  case $rc in
    0) summary+=("KILLED   $id $test") ;;
    1) summary+=("SURVIVED $id $test"); bad=1 ;;
    *) summary+=("ERROR($rc) $id $test"); bad=1 ;;
  esac
done
echo "=== SUMMARY"
printf '%s\n' "${summary[@]}"
exit $bad
