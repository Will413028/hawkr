package kyc

import (
	"bytes"
	"context"
	"slices"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
)

// 草稿同步、修剪與補算只走 hashFields：BANK_FINGERPRINT 在裡面就會被當成申請的欄位刪掉或重算。
// mutation：把 fieldBankFingerprint 加進 hashFields → 本測試失敗。
func TestBankFingerprintIsNotAnApplicationHashField(t *testing.T) {
	if slices.Contains(hashFields, fieldBankFingerprint) {
		t.Fatal("BANK_FINGERPRINT 不是申請的欄位")
	}
	if got, ok := linkedIdentityOf(fieldBankFingerprint); !ok || got != model.LinkedBankAccount || got.CountsAsGround() {
		t.Fatalf("fingerprint 命中是只列出的銀行帳號：%v %v", got, ok)
	}
}

// fingerprint 的 hash 輸入帶 provider（不同 provider 的同一字串不同），也不等於申請的銀行帳號輸入。
// mutation：輸入不帶 provider → 前兩個比較失敗；標籤與 IdentityBankAccount 相同 → 最後一個比較失敗。
func TestBankFingerprintInputIsScopedByProviderAndSeparatedFromKYCInputs(t *testing.T) {
	a, b := model.BankFingerprintInput("stripe", "fp1"), model.BankFingerprintInput("other", "fp1")
	if bytes.Equal(a, b) || bytes.Equal(a, model.BankFingerprintInput("stripe", "fp2")) {
		t.Fatal("provider 與 fingerprint 都要進輸入")
	}
	if bytes.Equal(a, model.IdentityInput(model.IdentityBankAccount, "stripe:fp1")) {
		t.Fatal("與申請的銀行帳號輸入要有不同標籤")
	}
}

func TestRecordBankFingerprintsRejectsEmptyValuesBeforeWriting(t *testing.T) {
	_, mac := sealtest.Keys(t)
	f := NewFingerprints(mac)
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
