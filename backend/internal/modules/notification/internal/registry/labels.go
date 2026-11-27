package registry

import (
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

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
