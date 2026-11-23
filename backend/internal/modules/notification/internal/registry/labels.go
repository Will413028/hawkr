package registry

import (
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// categoryLabel 回傳政策分類在語系 l 的顯示文字（取自共用文案目錄，不在模組裡重寫一份對照表），Locale 是文字實際使用的語系：
// 目錄只有 zh-TW 與 en，不認得的語系用 zh-TW，與模組句子（internal/text）的解析一致。
// 沒有執行期退回：en 缺某個分類的文字不會被悄悄換成中文，而是由 TestEveryRenderableValueHasEnglishText 在建置時紅燈。
func categoryLabel(l uitext.Locale, c policy.Category) (uitext.Message, error) {
	used := uitext.ZhTW
	if l == uitext.EN {
		used = uitext.EN
	}
	label, ok := uitext.For(used).CommonPolicyCategory(string(c))
	if !ok {
		return uitext.Message{}, fmt.Errorf("政策分類 %q 沒有顯示文字", c)
	}
	return uitext.Message{Text: label, Locale: used}, nil
}
