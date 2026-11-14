package kyc

import (
	"context"
	"errors"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/seal"
)

// Fingerprints 把 provider 回報的外部銀行帳號 fingerprint 以 blind index 寫進防詐保存的 account_identity_hashes（欄位 BANK_FINGERPRINT）。
// 它只需要 HMAC 金鑰環，不依賴 Service，所以收款帳戶的同步可以在 Service 之前組裝（Service 依賴收款帳戶）。
type Fingerprints struct{ mac *seal.MAC }

// NewFingerprints 建立寫入器；mac 必填。
func NewFingerprints(mac *seal.MAC) *Fingerprints {
	if mac == nil {
		panic("kyc: Fingerprints requires a MAC keyring")
	}
	return &Fingerprints{mac: mac}
}

// RecordBankFingerprints 在呼叫方的 transaction 內，對 storeID 的每個 fingerprint 寫金鑰環每一把金鑰算的一列（已存在的略過，重複同步不增加列）。
// 值帶 provider 代碼；明文 fingerprint 不落地。已掛上並經 provider 驗證的銀行帳號是事實，不是草稿的值：之後的快照不再有它也不刪列，
// 只由 5 年過期刪除。空字串的 fingerprint 是錯誤，不當成值寫入。
func (f *Fingerprints) RecordBankFingerprints(ctx context.Context, q *store.Queries, storeID int64, provider string, fingerprints []string, now time.Time) error {
	if provider == "" {
		return errors.New("kyc：寫入銀行帳號 fingerprint 需要 provider")
	}
	for _, fp := range fingerprints {
		if fp == "" {
			return errors.New("kyc：銀行帳號 fingerprint 不能是空字串")
		}
		for _, h := range f.mac.SumAll(model.BankFingerprintInput(provider, fp)) {
			if err := q.InsertAccountIdentityHash(ctx, store.InsertAccountIdentityHashParams{
				StoreID: storeID, Field: fieldBankFingerprint, KeyID: h.KeyID, Hash: h.Sum, CreatedAt: now,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
