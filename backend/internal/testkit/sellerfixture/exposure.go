//go:build integration

package sellerfixture

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

func ExposableStore(ctx context.Context, d *db.DB, accountID int64, now time.Time) (int64, error) {
	return exposableStore(ctx, d, nil, accountID, now)
}

func ExposableStoreWithID(ctx context.Context, d *db.DB, storeID, accountID int64, now time.Time) error {
	_, err := exposableStore(ctx, d, &storeID, accountID, now)
	return err
}

func exposableStore(ctx context.Context, d *db.DB, fixedStoreID *int64, accountID int64, now time.Time) (int64, error) {
	if _, err := d.Pool().Exec(ctx, `DO $$ DECLARE r record; BEGIN
		FOR r IN SELECT conrelid::regclass AS t, conname FROM pg_constraint
		         WHERE conrelid = 'seller.kyc_applications'::regclass AND contype = 'c'
		LOOP EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.t, r.conname); END LOOP; END $$`); err != nil {
		return 0, err
	}
	if fixedStoreID == nil {
		if _, err := d.Pool().Exec(ctx, `INSERT INTO seller.stores (store_code, account_id, store_name, logo_key, logo_asset_id, town_code, created_at, updated_at)
			VALUES ($1, $1, '測試店', 'logo/k', $3, '63000010', $2, $2)`, accountID, now, uuid.NewV7()); err != nil {
			return 0, err
		}
	} else {
		if _, err := d.Pool().Exec(ctx, `INSERT INTO seller.stores (store_id, store_code, account_id, store_name, logo_key, logo_asset_id, town_code, created_at, updated_at)
			OVERRIDING SYSTEM VALUE VALUES ($1, $2, $2, '測試店', 'logo/k', $4, '63000010', $3, $3)`, *fixedStoreID, accountID, now, uuid.NewV7()); err != nil {
			return 0, err
		}
	}
	var storeID int64
	if err := d.Pool().QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE account_id = $1", accountID).Scan(&storeID); err != nil {
		return 0, err
	}
	if _, err := d.Pool().Exec(ctx, `INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, created_at, updated_at, decided_at)
		VALUES ($3, $1, 'APPROVED', 'INDIVIDUAL', $2, $2, $2)`, storeID, now, uuid.NewV7()); err != nil {
		return 0, err
	}
	if _, err := d.Pool().Exec(ctx, "INSERT INTO seller.payout_capabilities (store_id, provider, transfers_active, updated_at) VALUES ($1, 'fake', true, $2)", storeID, now); err != nil {
		return 0, err
	}
	var exposable bool
	if err := d.Pool().QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM seller.pub_exposable_stores($1) WHERE store_id = $2)`, now, storeID).Scan(&exposable); err != nil {
		return 0, err
	}
	if !exposable {
		return 0, fmt.Errorf("seller store %d is not exposable", storeID)
	}
	return storeID, nil
}
