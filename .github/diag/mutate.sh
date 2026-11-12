#!/usr/bin/env bash
# Applies one mutation for the Stripe step 8b tests and prints the diff; exits 1 if nothing changed.
set -euo pipefail
cd backend/internal/modules/seller
case "$1" in
  field) perl -0pi -e 's/(type KYCDraftInput struct \{\n)/$1\tRealName *string\n/' internal/model/kyc.go ;;
  column) echo 'ALTER TABLE "seller"."kyc_applications" ADD COLUMN "real_name" text;' >> db/migrations/20261112070329_seller_drop_kyc_pii_pipeline.sql ;;
  noaudit) perl -0pi -e 's/(\t\t)_, err = (q\.InsertKYCDocumentAccess\(ctx, store\.InsertKYCDocumentAccessParams\{.*?\n\t\t\}\))\n/$1if false {\n_, err = $2\n}\n/s' internal/kyc/admin.go ;;
  *) exit 2 ;;
esac
git diff --exit-code --stat . && { echo "mutation $1 did not apply"; exit 1; } || true
git diff .
