//go:build integration

package seller_test

import (
	"testing"
	"time"

	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// bankFingerprintRowsAfter 是店家在 at 之後寫入的 BANK_FINGERPRINT 雜湊列數。
func (f *fixture) bankFingerprintRowsAfter(t *testing.T, storeID int64, at time.Time) int64 {
	t.Helper()
	return f.count(t, "SELECT count(*) FROM seller.account_identity_hashes WHERE source_store_id = $1 AND field = 'BANK_FINGERPRINT' AND created_at > $2", storeID, at)
}

// 受處分的帳號，收款帳戶的同步仍照常寫入新的 fingerprint：處分建立之前同步的 fingerprint 一組列，處分生效之後 provider 才回報的新 fingerprint
// 在處分建立時間之後多出新列，原有的列不變。同步路徑不看措施、店家狀態或店家期間。
// mutation：同步或 Apply 在帳號有生效措施時跳過 RecordBankFingerprints → 處分之後的列數為 0，失敗。
func TestSuspendedAccountStillRecordsANewFingerprintOnSync(t *testing.T) {
	t.Parallel()
	f := setup(t)
	accountID, s := f.approvedWithFingerprints(t, fpShared)
	before := f.fingerprintRows(t, s.StoreID)
	f.tick(time.Hour)
	m := f.suspend(t, s.StoreID)
	f.tick(time.Hour)
	if !f.effectiveAt(t, accountID, f.now) {
		t.Fatalf("停權應已生效")
	}
	snap := enabledSnapshot
	snap.BankFingerprints = []string{fpShared, fpOther}
	f.setPayment(t, s.StoreID, snap)
	if got := f.bankFingerprintRowsAfter(t, s.StoreID, m.CreatedAt); got == 0 {
		t.Fatalf("停權生效中同步新的 fingerprint 沒有寫入任何列")
	}
	if got, want := f.fingerprintRows(t, s.StoreID)-before, f.bankFingerprintRowsAfter(t, s.StoreID, m.CreatedAt); got != want {
		t.Fatalf("新增的列=%d，處分之後寫入的列=%d，兩者應一致（舊列不變）", got, want)
	}
}

// 終止合作已生效（店已因終止關閉）的帳號，收款帳戶仍存在、同步仍寫入新的 fingerprint。
// mutation：同步在店已關閉時跳過、或 Apply 看店家狀態 → 處分之後的列數為 0，失敗。
func TestTerminatedAccountStillRecordsANewFingerprintOnSync(t *testing.T) {
	t.Parallel()
	f := setup(t)
	_, s := f.approvedWithFingerprints(t, fpShared)
	before := f.fingerprintRows(t, s.StoreID)
	f.tick(time.Hour)
	m := f.terminate(t, s.StoreID, GroundFraud)
	f.tick(time.Hour)
	f.effect(t, m)
	if got := f.storeRow(t, s.StoreID); got.Status != Closed || got.Closure == nil || got.Closure.By != ClosedByTermination {
		t.Fatalf("終止之後要關店：%+v", got)
	}
	f.tick(time.Hour)
	snap := enabledSnapshot
	snap.BankFingerprints = []string{fpShared, fpOther}
	f.setPayment(t, s.StoreID, snap)
	if got := f.bankFingerprintRowsAfter(t, s.StoreID, m.CreatedAt); got == 0 {
		t.Fatalf("終止生效之後同步新的 fingerprint 沒有寫入任何列")
	}
	if got, want := f.fingerprintRows(t, s.StoreID)-before, f.bankFingerprintRowsAfter(t, s.StoreID, m.CreatedAt); got != want {
		t.Fatalf("新增的列=%d，處分之後寫入的列=%d，兩者應一致（舊列不變）", got, want)
	}
}
