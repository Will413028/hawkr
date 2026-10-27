package model

import (
	"testing"
	"time"
	"uuid"
)

func TestNormalizeIDCardNo(t *testing.T) {
	for raw, want := range map[string]string{"A123456789": "A123456789", " a123456789 ": "A123456789", "F 131104093": "F131104093"} {
		if got, ok := NormalizeIDCardNo(raw); !ok || got != want {
			t.Errorf("NormalizeIDCardNo(%q) = %q, %v", raw, got, ok)
		}
	}
	for _, raw := range []string{"", "A123456780", "A323456789", "1234567890", "A12345678", "AA23456789"} {
		if _, ok := NormalizeIDCardNo(raw); ok {
			t.Errorf("NormalizeIDCardNo(%q) 應該被拒絕", raw)
		}
	}
}

func TestNormalizeBankAccountKeepsLeadingZeros(t *testing.T) {
	if got, ok := NormalizeBankAccount(" 0123-456 789.01 "); !ok || got != "012345678901" {
		t.Fatalf("got %q, %v", got, ok)
	}
	for _, raw := range []string{"", "12345", "12a456789", "123456789012345678901"} {
		if _, ok := NormalizeBankAccount(raw); ok {
			t.Errorf("NormalizeBankAccount(%q) 應該被拒絕", raw)
		}
	}
}

// 不同標籤的輸入不會相同：身分證號、統編、銀行帳號即使值相同也算不同的比對。
func TestIdentityInputIsDomainSeparated(t *testing.T) {
	seen := map[string]SanctionIdentityKind{}
	for _, kind := range SanctionIdentityKindValues() {
		in := string(IdentityInput(kind, "12345678"))
		if other, dup := seen[in]; dup {
			t.Fatalf("%s 與 %s 的輸入相同：%q", kind, other, in)
		}
		seen[in] = kind
	}
	if string(IdentityInput(IdentityBankAccount, BankIdentityValue("004", "123456"))) == string(IdentityInput(IdentityBankAccount, BankIdentityValue("812", "123456"))) {
		t.Fatal("銀行代碼要進 blind index 的輸入")
	}
}

func TestMasks(t *testing.T) {
	if got := MaskIDCardNo("A123456789"); got != "A1******89" {
		t.Errorf("id = %q", got)
	}
	if got := MaskBankAccount("012345678901"); got != "********8901" {
		t.Errorf("bank = %q", got)
	}
	if got := MaskBankAccount("123"); got != "***" {
		t.Errorf("short = %q", got)
	}
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

func TestNormalizeEmailAndMobile(t *testing.T) {
	if got, ok := NormalizeEmail(" Owner@Example.COM "); !ok || got != "owner@example.com" {
		t.Errorf("email = %q, %v", got, ok)
	}
	for _, raw := range []string{"", "user@localhost", "no-at.example.com", "Name <a@b.co>"} {
		if _, ok := NormalizeEmail(raw); ok {
			t.Errorf("NormalizeEmail(%q) 應該被拒絕", raw)
		}
	}
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

func TestPayUniObservationCursorValid(t *testing.T) {
	now := time.Now()
	for name, c := range map[string]PayUniObservationCursor{"zero": {}, "no time": {ObservationID: uuid.NewV7()}, "no id": {ExecutedAt: now}} {
		if c.Valid() {
			t.Errorf("%s 不是完整的位置", name)
		}
	}
	if !(PayUniObservationCursor{ExecutedAt: now, ObservationID: uuid.NewV7()}).Valid() {
		t.Error("完整的位置要有效")
	}
}
