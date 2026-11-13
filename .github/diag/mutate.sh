#!/usr/bin/env bash
# Applies one mutation for the Stripe step 8d tests and prints the diff; exits 1 if nothing changed.
set -euo pipefail
root=$(pwd)
cd backend/internal/modules/seller
files="db/objects.sql db/migrations/20261112233958_seller_shared_bank_account_review.sql"
case "$1" in
  nohash) perl -pi -e 's/ON f\.hash = mi\.hash AND f\.key_id = mi\.key_id/ON f.key_id = mi.key_id/' $files ;;
  noreviewed) perl -ni -e 'print unless /NOT EXISTS \(SELECT 1 FROM seller\.measure_linked_reviews r /' $files ;;
  notime) perl -ni -e 'print unless /HAVING min\(h\.created_at\) > m\.created_at/' $files ;;
  rowwise) perl -pi -e 's/^(\s*)HAVING min\(h\.created_at\) > m\.created_at/$1HAVING true/; s/(AND h\.account_id <> m\.account_id)$/$1 AND h.created_at > m.created_at/' $files ;;
  *) exit 2 ;;
esac
cp "$root/.github/diag/$1/atlas.sum" db/migrations/atlas.sum
git diff --exit-code --stat . && { echo "mutation $1 did not apply"; exit 1; } || true
git diff .
