package kyc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

const fieldTaxID = "TAX_ID"

const fieldBankFingerprint = "BANK_FINGERPRINT"

func linkedIdentityOf(field string) (model.LinkedIdentity, bool) {
	switch field {
	case fieldTaxID:
		return model.LinkedUBN, true
	case fieldBankFingerprint:
		return model.LinkedBankAccount, true
	}
	return "", false
}

func (p protector) writeHashes(ctx context.Context, q *store.Queries, row store.SellerKycApplication, now time.Time) error {
	if row.TaxID == nil {
		return nil
	}
	for _, h := range p.mac.SumAll(taxInput(*row.TaxID)) {
		if err := q.InsertAccountIdentityHash(ctx, store.InsertAccountIdentityHashParams{
			StoreID: row.StoreID, Field: fieldTaxID, KeyID: h.KeyID, Hash: h.Sum, CreatedAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (p protector) syncHashes(ctx context.Context, q *store.Queries, before *store.SellerKycApplication, after store.SellerKycApplication, now time.Time) error {
	if before != nil && model.Deref(before.TaxID) == model.Deref(after.TaxID) {
		return nil
	}
	held, err := q.StoreKYCTaxIDs(ctx, after.StoreID)
	if err != nil {
		return err
	}
	keep := [][]byte{}
	for _, taxID := range held {
		keep = append(keep, p.hashes(taxInput(taxID))...)
	}
	if err := q.PruneAccountIdentityHashes(ctx, store.PruneAccountIdentityHashesParams{StoreID: after.StoreID, Keep: keep}); err != nil {
		return err
	}
	return p.writeHashes(ctx, q, after, now)
}

const rehashBatch = 100

// RehashIdentities 補算金鑰環每一把金鑰的統編比對 hash（輪替之後新增的金鑰，既有申請還沒有它的列）：只讀缺列的店家的申請（統編取自申請的明文欄位，同一店家的申請在同一個 transaction 內補完），鎖住申請列（與草稿寫入互斥，
// 不會把剛被改掉的欄位的舊值寫回去）。已經齊全的申請不會被讀到，所以沒有輪替時什麼都不做。回傳補算的申請數；啟動時呼叫，可重複執行。
func (s *Service) RehashIdentities(ctx context.Context) (int, error) {
	total := 0
	for _, keyID := range s.p.mac.IDs() {
		var after *int64
		for {
			n, stores, last, err := s.rehashBatchFor(ctx, keyID, after)
			if err != nil {
				return total, err
			}
			total += n
			if stores < rehashBatch {
				break
			}
			after = &last
		}
	}
	return total, nil
}

func (s *Service) rehashBatchFor(ctx context.Context, keyID string, after *int64) (applications, stores int, last int64, err error) {
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		now := s.d.Now().UTC()
		q := store.New(s.d.DB.Conn(ctx))
		rows, err := q.KYCApplicationsMissingHashKey(ctx, store.KYCApplicationsMissingHashKeyParams{KeyID: keyID, AfterStoreID: after, BatchLimit: rehashBatch})
		if err != nil {
			return err
		}
		seen := map[int64]bool{}
		for _, row := range rows {
			if err := s.p.writeHashes(ctx, q, row, now); err != nil {
				return fmt.Errorf("seller：補算申請 %s 的統編比對 hash：%w", row.ApplicationID, err)
			}
			seen[row.StoreID] = true
			last = max(last, row.StoreID)
		}
		applications, stores = len(rows), len(seen)
		return nil
	})
	return applications, stores, last, err
}
