package model

import (
	"crypto/sha256"
	"encoding/base64"
	"slices"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func strp(s string) *string { return &s }

func TestCapabilitiesAreTheClosedNine(t *testing.T) {
	want := []Capability{
		"storefront.read", "storefront.write", "products.read", "products.write", "products.publish",
		"posts.read", "posts.write", "posts.publish", "orders.read",
	}
	if got := Capabilities(); !slices.Equal(got, want) {
		t.Fatalf("Capabilities() = %v", got)
	}
	for _, c := range []Capability{"STOREFRONT.READ", "orders.write", "payments.read", ""} {
		if c.Valid() {
			t.Errorf("%q 不在封閉的九個值內", c)
		}
	}
}

func TestAuditActionRules(t *testing.T) {
	if n := len(AuditActions()); n != 19 {
		t.Fatalf("登錄的動作 %d 個，預期 19", n)
	}
	cases := []struct {
		action AuditAction
		risk   RiskLevel
		label  string
	}{
		{AuditProductUpdate, RiskMedium, "編輯商品"},
		{AuditProductPublish, RiskHigh, "上架商品"},
		{AuditOrderPIIReveal, RiskHigh, "查看訂單完整收件資訊"},
		{AuditOrderRead, RiskLow, "查看訂單（被拒絕）"},
		// 沒有登錄的動作：風險 fail closed 成 high，標籤回傳原值。
		{"ORDER_REFUND_APPROVE", RiskHigh, "ORDER_REFUND_APPROVE"},
	}
	for _, c := range cases {
		if c.action.RiskLevel() != c.risk || c.action.Label() != c.label {
			t.Errorf("%s = %s、%q，預期 %s、%q", c.action, c.action.RiskLevel(), c.action.Label(), c.risk, c.label)
		}
	}
	for _, a := range AuditActions() {
		if !a.Known() || a.Label() == string(a) {
			t.Errorf("%s 沒有標籤", a)
		}
	}
}

func TestNotifySellerIsImpossibleForReadsRevealsAndCorrections(t *testing.T) {
	never := []AuditAction{AuditStorefrontRead, AuditProductRead, AuditPostRead, AuditOrderRead, AuditOrderPIIReveal, AuditCorrection}
	for _, a := range AuditActions() {
		if want := !slices.Contains(never, a); a.MayNotifySeller() != want {
			t.Errorf("%s.MayNotifySeller() = %v，預期 %v", a, a.MayNotifySeller(), want)
		}
	}
}

func TestReasons(t *testing.T) {
	opts := DelegationReasonOptions()
	if len(opts) != 3 || opts[0].Code != ReasonSellerRequest || opts[0].Label != "賣家請求（管理員記錄）" {
		t.Fatalf("選項 = %+v", opts)
	}
	for _, o := range opts {
		if !o.CaseReferenceRequired || o.Code.Label() != o.Label || !o.Code.CaseReferenceRequired() {
			t.Errorf("%+v", o)
		}
	}
	// 不認得的理由沒有標籤（不會被當成標籤存起來），而且要案件編號（fail closed）。
	if r := DelegationReason("SELLER_REQUEST"); r.Label() != "" || !r.CaseReferenceRequired() {
		t.Fatal("不認得的理由")
	}
	if !AnyCaseReferenceRequired() {
		t.Fatal("AnyCaseReferenceRequired")
	}
}

func TestValidateCaseReference(t *testing.T) {
	ok := []*string{nil, strp("HK-1"), strp("案件 2026-0927"), strp(strings.Repeat("1", 200))}
	bad := []*string{strp(""), strp("HK1"), strp("沒有數字的編號"), strp(strings.Repeat("1", 201)), strp("HK-1\nX")}
	for _, r := range ok {
		if err := ValidateCaseReference(r); err != nil {
			t.Errorf("%v：%v", r, err)
		}
	}
	for _, r := range bad {
		if d, ok := DelegationDenialOf(ValidateCaseReference(r)); !ok || d.Denial != DenialInvalidCommand {
			t.Errorf("%q 應該是 9260", *r)
		}
	}
}

func TestHandoffAndOperationCredentialsUseIndependentRandomTokens(t *testing.T) {
	a, b := NewHandoffCode(), NewHandoffCode()
	if a == b || !WellFormedToken(a, HandoffPrefix) || WellFormedToken(a, "dops_") {
		t.Fatalf("a=%q b=%q", a, b)
	}
	credential, other := NewOperationCredential(), NewOperationCredential()
	if credential == other || !WellFormedToken(credential, CredentialPrefix) || WellFormedToken(credential, HandoffPrefix) {
		t.Fatalf("credential=%q other=%q", credential, other)
	}
	bytes, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(credential, CredentialPrefix))
	if err != nil || len(bytes) != 32 {
		t.Fatalf("credential random bytes = %d, %v", len(bytes), err)
	}
	for _, s := range []string{"", "dhof_", "dhof_" + strings.Repeat("!", 43), "dops_" + strings.Repeat("a", 43), a + "x"} {
		if WellFormedToken(s, HandoffPrefix) {
			t.Errorf("%q 不該是 handoff code", s)
		}
	}
	if h := sha256.Sum256([]byte(a)); !slices.Equal(TokenHash(a), h[:]) {
		t.Fatal("TokenHash 是 SHA-256")
	}
}

func TestPagingIsValidatedNotClamped(t *testing.T) {
	p, err := DelegationPaging{}.Normalize()
	if err != nil || p.Page != 1 || p.PageSize != 20 || p.Offset() != 0 {
		t.Fatalf("預設 = %+v, %v", p, err)
	}
	if p, _ := (DelegationPaging{Page: 3, PageSize: 50}).Normalize(); p.Offset() != 100 {
		t.Fatalf("Offset = %d", p.Offset())
	}
	for _, bad := range []DelegationPaging{{Page: -1}, {PageSize: 51}, {PageSize: -1}} {
		if _, err := bad.Normalize(); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Errorf("%+v 應該是 9260", bad)
		}
	}
}

func TestEveryDenialIsClassified(t *testing.T) {
	for d := DenialInvalidCommand; d <= DenialAdminNotAllowlisted; d++ {
		err := Deny(d, "x")
		if apperr.KindOf(err) == apperr.KindInternal {
			t.Errorf("拒絕 %d 沒有分類", d)
		}
		if got, ok := DelegationDenialOf(err); !ok || got.Denial != d || got.Message() != "x" {
			t.Errorf("DelegationDenialOf(%d) = %+v", d, got)
		}
	}
}
