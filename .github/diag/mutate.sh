#!/usr/bin/env bash
# Applies one mutation for the Stripe step 6 tests and prints the diff; exits 1 if nothing changed.
set -euo pipefail
cd backend/internal/modules/seller
case "$1" in
  nosync) perl -0pi -e 's/snap\.BankFingerprints, now\)/nil, now)/' internal/payment/sync.go ;;
  union) perl -0pi -e "s/AND f\.field = 'BANK_FINGERPRINT'/AND false/" internal/store/kyc.sql.go ;;
  snapmap) perl -0pi -e "s/ WHEN 'BANK_FINGERPRINT' THEN 'BANK_ACCOUNT'//" internal/store/kyc.sql.go ;;
  *) exit 2 ;;
esac
git diff --exit-code --stat . && { echo "mutation $1 did not apply"; exit 1; } || true
git diff .
