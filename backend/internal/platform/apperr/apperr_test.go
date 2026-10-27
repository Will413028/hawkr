package apperr

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestKindOfFollowsTheErrorChain(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want Kind
	}{
		{"未分類", errors.New("boom"), KindInternal},
		{"nil", nil, KindInternal},
		{"InvalidInput", InvalidInput("title 必填"), KindInvalidInput},
		{"包過一層的 NotFound", fmt.Errorf("讀取 banner：%w", NotFound("banner %d 不存在", 7)), KindNotFound},
		{"VersionConflict", VersionConflict("banner 已被修改"), KindVersionConflict},
		{"AlreadyExists", AlreadyExists("門檻已經存在"), KindAlreadyExists},
		{"FailedPrecondition", FailedPrecondition("已達上限"), KindFailedPrecondition},
		{"Unauthenticated", Unauthenticated("登入已失效"), KindUnauthenticated},
		{"PermissionDenied", PermissionDenied("帳號已停權"), KindPermissionDenied},
		{"Unavailable", Unavailable("此登入方式目前無法使用"), KindUnavailable},
		{"ResourceExhausted", ResourceExhausted(time.Minute, "請稍後再試"), KindResourceExhausted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := KindOf(c.err); got != c.want {
				t.Fatalf("KindOf = %v，預期 %v", got, c.want)
			}
		})
	}
}

func TestMessageIsFormatted(t *testing.T) {
	e := NotFound("banner %d 不存在", 7)
	if e.Message != "banner 7 不存在" {
		t.Fatalf("Message = %q", e.Message)
	}
}

func TestInvalidInputOnUsesTheMessage(t *testing.T) {
	e := InvalidInputOn([]string{"PreviewImageToken", "PopupImageToken"}, "新增 banner 必須上傳預覽圖與彈窗圖")
	want := []FieldViolation{
		{Field: "PreviewImageToken", Message: e.Message},
		{Field: "PopupImageToken", Message: e.Message},
	}
	if len(e.FieldViolations()) != len(want) || e.FieldViolations()[0] != want[0] || e.FieldViolations()[1] != want[1] {
		t.Fatalf("Violations = %+v，預期 %+v", e.FieldViolations(), want)
	}
}

// InvalidInputFields 給欄位自己的訊息。
func TestInvalidInputFieldsKeepsPerFieldMessages(t *testing.T) {
	e := InvalidInputFields([]FieldViolation{{"A", "缺 A"}, {"B", "缺 B"}}, "申請資料不完整")
	if e.Kind() != KindInvalidInput {
		t.Fatalf("Kind = %v", e.Kind())
	}
	got := e.FieldViolations()
	if len(got) != 2 || got[0] != (FieldViolation{"A", "缺 A"}) || got[1] != (FieldViolation{"B", "缺 B"}) {
		t.Fatalf("FieldViolations = %+v", got)
	}
}

func TestReasonsListsEveryDeclaredReason(t *testing.T) {
	want := []Reason{ReasonVersionConflict, ReasonSellerSuspended, ReasonProspectMatchFound, ReasonProspectTaxIDHeld, ReasonAccountHasOpenStore, ReasonAgreementOutdated, ReasonSellerRequirementMissing, ReasonStoreOpeningSuspended, ReasonStoreOpeningTerminated}
	if got := Reasons(); !slices.Equal(got, want) || len(got) != 9 || got[0].String() != "VERSION_CONFLICT" || got[1].String() != "SELLER_SUSPENDED" || got[2].String() != "PROSPECT_MATCH_FOUND" || got[3].String() != "PROSPECT_TAX_ID_HELD" || got[4].String() != "ACCOUNT_HAS_OPEN_STORE" || got[5].String() != "AGREEMENT_OUTDATED" || got[6].String() != "SELLER_REQUIREMENT_MISSING" || got[7].String() != "STORE_OPENING_SUSPENDED" || got[8].String() != "STORE_OPENING_TERMINATED" {
		t.Fatalf("Reasons() = %v", got)
	}
	if (VersionConflict("x").Reason) != ReasonVersionConflict || (NotFound("x").Reason != Reason{}) || (InvalidInput("x").Reason != Reason{}) || len(InvalidInput("x").Metadata) != 0 {
		t.Fatal("VersionConflict 要帶 ReasonVersionConflict，其他分類預設沒有業務原因")
	}
}

func TestWithMetadataAccumulatesKeys(t *testing.T) {
	e := FailedPrecondition("x").WithReason(ReasonProspectMatchFound)
	if e.Metadata != nil {
		t.Fatalf("沒有設定時 Metadata 應為 nil：%v", e.Metadata)
	}
	e = e.WithMetadata("a", "1").WithMetadata("b", "2")
	if len(e.Metadata) != 2 || e.Metadata["a"] != "1" || e.Metadata["b"] != "2" {
		t.Fatalf("Metadata = %v", e.Metadata)
	}
}

func TestWithCauseKeepsPublicClassification(t *testing.T) {
	cause := PermissionDenied("private detail")
	public := Unauthenticated("public message")
	err := WithCause(public, cause)
	var got *Error
	if !errors.As(err, &got) || got != public || KindOf(err) != KindUnauthenticated || !errors.Is(err, public) || errors.Is(err, cause) {
		t.Fatalf("classification: %v", err)
	}
	if !strings.Contains(err.Error(), "private detail") {
		t.Fatal("missing diagnostic cause")
	}
}

// FieldViolations 回傳複本：改動它不會影響錯誤本身。
func TestFieldViolationsReturnsACopy(t *testing.T) {
	e := InvalidInputOn([]string{"A"}, "不合法")
	e.FieldViolations()[0].Field = "changed"
	if got := e.FieldViolations()[0].Field; got != "A" {
		t.Fatalf("Field = %q，內部欄位被改動", got)
	}
}
