package audit

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/delegation/internal/model"
)

func TestChangedFieldsRoundTrip(t *testing.T) {
	in := []model.ChangedField{{Field: "title", Label: "標題", Before: "", After: "梨山水蜜桃"}, {Field: "price", Label: "價格", Before: "100", After: "120"}}
	raw, err := encodeChangedFields(in)
	if err != nil {
		t.Fatal(err)
	}
	// 空的 before 也要留著：「原本是空的」正是差異的一部分。
	if !strings.Contains(string(raw), `"before":""`) {
		t.Fatalf("%s", raw)
	}
	if got := decodeChangedFields(raw); !slices.Equal(got, in) {
		t.Fatalf("= %+v", got)
	}
	if got := decodeChangedFields([]byte(`[{"field":"","label":"x"},{"field":"a","label":"A","before":"1","after":"2"}]`)); len(got) != 1 || got[0].Field != "a" {
		t.Fatalf("沒有欄位名的項目要略過：%+v", got)
	}
	if decodeChangedFields([]byte(`{"not":"an array"}`)) != nil || decodeChangedFields(nil) != nil {
		t.Fatal("解析不了的列回空的")
	}
	if raw, _ := encodeChangedFields(nil); raw != nil {
		t.Fatal("沒有變更時存 NULL")
	}
}

func TestResourceNamesAreTruncatedByRunes(t *testing.T) {
	if got := truncateRunes(strings.Repeat("桃", 200), model.ResourceNameMaxRunes); got != strings.Repeat("桃", 160) {
		t.Fatalf("= %d 字", len([]rune(got)))
	}
	if truncateRunes("梨山水蜜桃", model.ResourceNameMaxRunes) != "梨山水蜜桃" {
		t.Fatal("短的名稱不變")
	}
}

// 不合 CHECK 規則的 failure_reason 在到資料庫之前就被擋成 INVALID_COMMAND（驗證發生在任何查詢之前，所以 q 可以是 nil）。
func TestConfirmRejectsAnIllegalFailureReasonBeforeTheDatabase(t *testing.T) {
	for _, fr := range []string{"lower", "HAS SPACE", "UNSPECIFIED", "_X", "A__B", "9X"} {
		_, err := (&Module{}).confirmAudit(context.Background(), nil, model.AuditConfirmInput{Result: model.AuditFailed, FailureReason: fr}, time.Time{})
		d, ok := model.DelegationDenialOf(err)
		if !ok || d.Denial != model.DenialInvalidCommand {
			t.Errorf("%q：%v，預期 INVALID_COMMAND", fr, err)
		}
	}
}
