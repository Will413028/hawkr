package registry

import (
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// categoryLabel 回傳政策分類在語系 l 的顯示文字（取自共用文案目錄，不在模組裡重寫一份對照表）與實際使用的語系。
// 目錄在 l 沒有這個分類的文字時（目前 en 只有社群類別），整則通知退回 zh-TW，不在一句英文裡夾一個中文理由。
func categoryLabel(l uitext.Locale, c policy.Category) (string, uitext.Locale, error) {
	label, ok := uitext.For(l).CommonPolicyCategory(string(c))
	if !ok {
		return "", l, fmt.Errorf("政策分類 %q 沒有顯示文字", c)
	}
	if l != uitext.ZhTW {
		if zh, _ := uitext.For(uitext.ZhTW).CommonPolicyCategory(string(c)); zh == label {
			return zh, uitext.ZhTW, nil
		}
	}
	return label, l, nil
}
