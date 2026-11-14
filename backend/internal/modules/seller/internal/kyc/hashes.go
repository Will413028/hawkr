package kyc

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

// 防詐保存的比對 hash 的欄位名（account_identity_hashes.field）：申請的統編（Stripe 計畫 D42：申請沒有自己的 hash 表，身分證號、負責人身分證號與
// 平台自收的銀行帳號三個舊種類已刪，D38）。
const fieldTaxID = "TAX_ID"

// fieldBankFingerprint 是 provider 回報的外部銀行帳號 fingerprint 的 blind index（來源是 provider 不是申請）。
// 申請的草稿同步、修剪與補算都只處理統編，不能動到它。
const fieldBankFingerprint = "BANK_FINGERPRINT"

// linkedIdentityOf 把防詐保存的欄位對到關聯帳號命中的身分種類。
func linkedIdentityOf(field string) (model.LinkedIdentity, bool) {
	switch field {
	case fieldTaxID:
		return model.LinkedUBN, true
	case fieldBankFingerprint:
		return model.LinkedBankAccount, true
	}
	return "", false
}

// writeHashes 把申請的統編以金鑰環每一把金鑰算的 blind index 寫進防詐保存的 account_identity_hashes（已存在的略過）：寫入只有這一個呼叫點。
// account_identity_hashes 不隨個資清理刪除（過期由清理 job 刪，ADR 的 2026-10-30 修訂）。申請沒有統編時什麼都不寫。
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

// syncHashes 在寫入申請的同一個 transaction 內讓防詐保存的統編 hash 與申請一致：跟寫入前的列（before，新建時是 nil）比，統編的明文有變才修剪並重寫
// （修剪只刪這個 stint 已沒有任何申請持有的值，草稿裡改掉的值不是送出的事實，不保存），沒動就什麼都不做。
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

// rehashBatch 是補算一批的申請數。
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

// rehashBatchFor 補一批店家（至多 rehashBatch 個）：店家的申請在同一個 transaction 內全部補完，回傳補算的申請數、店家數與最後一個店家。
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
