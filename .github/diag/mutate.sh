#!/usr/bin/env bash
# Applies one mutation to the seller store and prints the diff; exits 1 if nothing changed.
set -euo pipefail
cd backend/internal/modules/seller/internal/store
case "$1" in
  snapshot) perl -0pi -e 's/FROM seller\.account_identity_hashes h\nWHERE h\.account_id = \$2/FROM (SELECT kh.field, kh.key_id, kh.hash, st.account_id FROM seller.kyc_identity_hashes kh JOIN seller.kyc_applications ka ON ka.application_id = kh.application_id JOIN seller.stores st ON st.store_id = ka.store_id) h\nWHERE h.account_id = \$2/' kyc.sql.go ;;
  expire) perl -0pi -e 's/(DELETE FROM seller\.account_identity_hashes a\nUSING seller\.store_closures c\nWHERE )/$1false AND /' purge.sql.go ;;
  bank) perl -0pi -e "s/(  AND mi\.kind = CASE h\.field WHEN 'TAX_ID' THEN 'TAX_ID' WHEN 'BANK_ACCOUNT' THEN 'BANK_ACCOUNT' ELSE 'ID_CARD' END)/\$1\n  AND h.field <> 'BANK_ACCOUNT'/" kyc.sql.go ;;
  keys) perl -0pi -e 's/\n    UNION SELECT key_id FROM seller\.account_identity_hashes//' kyc.sql.go ;;
  *) exit 2 ;;
esac
git diff --exit-code --stat . && { echo "mutation $1 did not apply"; exit 1; } || true
git diff .
