package sanction

import (
	"errors"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/social/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// checkReason 的錯誤訊息來自 social/internal/text 的目錄（後台員工受眾，只有 zh-TW）。
func TestCheckReasonMessagesComeFromCatalog(t *testing.T) {
	cases := []struct {
		name                  string
		category, reason, ref string
		admin                 int64
		want                  string
	}{
		{"category", "NOT_A_CATEGORY", "x", "", 1, "理由的類別不合法"},
		{"reason too long", "SPAM", strings.Repeat("字", model.MaxHideReasonRunes+1), "", 1, "理由不可超過 "},
		{"source ref too long", "SPAM", "x", strings.Repeat("字", model.MaxSourceRefRunes+1), 1, "來源參照不可超過 "},
		{"admin", "SPAM", "x", "", 0, "缺少管理員"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkReason(tc.category, tc.reason, tc.ref, tc.admin)
			var ae *apperr.Error
			if !errors.As(err, &ae) || ae.Kind() != apperr.KindInvalidInput || !strings.HasPrefix(ae.Message, tc.want) {
				t.Fatalf("err = %v, want prefix %q", err, tc.want)
			}
		})
	}
	if err := checkReason("SPAM", "x", "", 1); err != nil {
		t.Fatal(err)
	}
}
