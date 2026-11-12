package kyc

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
)

// 申請寫入防詐保存的欄位只有統編（TAX_ID）；BANK_FINGERPRINT 不是申請的欄位（草稿的修剪只刪 TAX_ID 列，SQL 的 field = 'TAX_ID'），
// 三個已刪的舊種類（D38）也不再認得：關聯帳號比對遇到它們是錯誤，不是命中。
// mutation：把 fieldTaxID 改成 BANK_FINGERPRINT → 本測試失敗。
func TestBankFingerprintIsNotAnApplicationHashField(t *testing.T) {
	if fieldTaxID == fieldBankFingerprint || fieldTaxID != "TAX_ID" {
		t.Fatalf("申請的欄位 %q 不能是 fingerprint 的欄位 %q", fieldTaxID, fieldBankFingerprint)
	}
	if got, ok := linkedIdentityOf(fieldTaxID); !ok || got != model.LinkedUBN || !got.CountsAsGround() {
		t.Fatalf("統編命中是可以當依據的 UBN：%v %v", got, ok)
	}
	if got, ok := linkedIdentityOf(fieldBankFingerprint); !ok || got != model.LinkedBankAccount || got.CountsAsGround() {
		t.Fatalf("fingerprint 命中是只列出的銀行帳號：%v %v", got, ok)
	}
	for _, removed := range []string{"ID_CARD_NO", "REP_ID_CARD_NO", "BANK_ACCOUNT"} {
		if _, ok := linkedIdentityOf(removed); ok {
			t.Errorf("%s 是已刪的種類，不應該再對應到身分", removed)
		}
	}
}

// fingerprint 的 hash 輸入帶 provider（不同 provider 的同一字串不同），也不等於統編的輸入。
// mutation：輸入不帶 provider → 第一個比較失敗；標籤與 IdentityTaxID 相同 → 最後一個比較失敗。
func TestBankFingerprintInputIsScopedByProviderAndSeparatedFromKYCInputs(t *testing.T) {
	a, b := model.BankFingerprintInput("stripe", "fp1"), model.BankFingerprintInput("other", "fp1")
	if bytes.Equal(a, b) || bytes.Equal(a, model.BankFingerprintInput("stripe", "fp2")) {
		t.Fatal("provider 與 fingerprint 都要進輸入")
	}
	if bytes.Equal(a, model.IdentityInput(model.IdentityTaxID, "stripe:fp1")) {
		t.Fatal("與統編的輸入要有不同標籤")
	}
}

func TestRecordBankFingerprintsRejectsEmptyValuesBeforeWriting(t *testing.T) {
	f := NewFingerprints(sealtest.MAC(t))
	if err := f.RecordBankFingerprints(context.Background(), nil, 1, "stripe", []string{""}, time.Time{}); err == nil {
		t.Fatal("空字串的 fingerprint 通過了")
	}
	if err := f.RecordBankFingerprints(context.Background(), nil, 1, "", []string{"fp"}, time.Time{}); err == nil {
		t.Fatal("沒有 provider 通過了")
	}
	if err := f.RecordBankFingerprints(context.Background(), nil, 1, "stripe", nil, time.Time{}); err != nil {
		t.Fatalf("沒有 fingerprint 什麼都不做：%v", err)
	}
}
