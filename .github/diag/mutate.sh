#!/usr/bin/env bash
# Applies one mutation for the Stripe step 8e tests and prints the diff; exits 1 if nothing changed.
set -euo pipefail
root=$(pwd)
cd backend/internal/modules/seller
files="db/objects.sql db/migrations/20261113134227_seller_identity_hits_at_function.sql"
case "$1" in
  nocurrent) perl -pi -e 's/ON i\.account_id = o\.account_id$/ON i.account_id = o.account_id AND false/' $files ;;
  snaponly) perl -pi -e 's/^WHERE s\.hash = ANY\(\$2::bytea\[\]\) AND s\.account_id <> \$3::bigint$/WHERE s.hash = ANY(\$2::bytea[]) AND s.account_id <> \$3::bigint AND s.created_at IS NULL/' internal/store/kyc.sql.go ;;
  nonotice) perl -pi -e "s/\(m\.kind = 'TERMINATION' AND m\.termination_ground <> 'NOTICE'\)/m.kind = 'TERMINATION'/" $files ;;
  nogate) perl -ni -e 'if (/ON i\.account_id = o\.account_id$/) { s/$/;/; print; $skip = 1; next } if ($skip) { $skip = 0; next } print' $files ;;
  *) exit 2 ;;
esac
if [ -f "$root/.github/diag/$1/atlas.sum" ]; then cp "$root/.github/diag/$1/atlas.sum" db/migrations/atlas.sum; fi
git diff --exit-code --stat . && { echo "mutation $1 did not apply"; exit 1; } || true
git diff .
