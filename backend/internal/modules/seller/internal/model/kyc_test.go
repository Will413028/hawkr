package model

import (
	"testing"
	"time"
	"uuid"
)

// 統編的 blind index 輸入與銀行帳號 fingerprint 的輸入是不同的 hash 空間：值相同也不會互相命中。
func TestIdentityInputIsDomainSeparated(t *testing.T) {
	tax := string(IdentityInput(IdentityTaxID, "12345678"))
	fingerprint := string(BankFingerprintInput("stripe", "12345678"))
	if tax == fingerprint {
		t.Fatalf("統編與 fingerprint 的輸入相同：%q", tax)
	}
	if string(BankFingerprintInput("stripe", "x")) == string(BankFingerprintInput("other", "x")) {
		t.Fatal("provider 代碼要進 fingerprint 的輸入")
	}
}

// 申請只持有統編一種識別值（D22、D38）：其他種類不是可以算 hash 的輸入。
func TestIdentityInputOnlyKnowsTheTaxID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("IdentityInput(BANK_ACCOUNT) 應該 panic")
		}
	}()
	IdentityInput(IdentityBankAccount, "12345678")
}

func TestParseDate(t *testing.T) {
	// 2026-10-01 23:30 UTC 是台北的 10 月 2 日：台北的今天是有效的日期，明天不是。
	now := time.Date(2026, 10, 1, 23, 30, 0, 0, time.UTC)
	for raw, ok := range map[string]bool{
		"1990-05-06": true, "2026-10-02": true, "2026-10-03": false, "2025-02-29": false, "2024-02-29": true,
		"1899-12-31": false, "": false, "90-05-06": false, "1990/05/06": false,
	} {
		if _, got := ParseDate(raw, now); got != ok {
			t.Errorf("ParseDate(%q) = %v，預期 %v", raw, got, ok)
		}
	}
}

func TestNormalizeMobile(t *testing.T) {
	if got, ok := NormalizeMobile("+886 912-345-678"); !ok || got != "0912345678" {
		t.Errorf("mobile = %q, %v", got, ok)
	}
	if _, ok := NormalizeMobile("0229010572"); ok {
		t.Error("市話不是手機")
	}
}

// 統編：8 碼數字加 2023 年起的檢查碼（第 7 位是 7 時總和或總和加 1 是 5 的倍數）；空白去掉。
func TestNormalizeTaxID(t *testing.T) {
	if got, ok := NormalizeTaxID(" 3333 0333 "); !ok || got != "33330333" {
		t.Errorf("空白要去掉：%q %v", got, ok)
	}
	for in, want := range map[string]bool{
		"33330333": true, "71777777": true, "00000000": true, "11111101": true,
		"10458570": true,  // 第 7 位是 7，總和 25
		"10000073": true,  // 第 7 位是 7，總和 14，加 1 是 5 的倍數
		"10000030": false, // 總和 14，第 7 位不是 7：不適用加 1
		"12345678": false, "22099132": false, "": false, "1234567": false, "1234567a": false, "１２３４５６７８": false,
	} {
		if got, ok := NormalizeTaxID(in); ok != want {
			t.Errorf("NormalizeTaxID(%q) = %q, %v，預期 %v", in, got, ok, want)
		}
	}
}

// 佇列的排序方向只有 OldestFirst 一處定義：只有篩選審核中是舊到新。
func TestKYCListInputOldestFirstOnlyForTheSubmittedQueue(t *testing.T) {
	for _, status := range KYCStatusValues() {
		if got := (KYCListInput{Status: &status}).OldestFirst(); got != (status == KYCStatusSubmitted) {
			t.Errorf("%s → %v", status, got)
		}
	}
	if (KYCListInput{}).OldestFirst() {
		t.Error("沒有篩選是新到舊")
	}
}

func TestKYCCursorValid(t *testing.T) {
	now := time.Now()
	for name, c := range map[string]KYCCursor{"zero": {}, "no time": {ApplicationID: uuid.NewV7()}, "no id": {SubmittedAt: now}} {
		if c.Valid() {
			t.Errorf("%s 不是完整的位置", name)
		}
	}
	if !(KYCCursor{SubmittedAt: now, ApplicationID: uuid.NewV7()}).Valid() {
		t.Error("完整的位置要有效")
	}
}
