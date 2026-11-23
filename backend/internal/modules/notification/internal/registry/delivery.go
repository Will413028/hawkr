package registry

import (
	"slices"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
)

// All 回傳全部型別（All 的方法形式，給需要注入註冊表的呼叫端）。
func (Registry) All() []Entry { return All() }

// Delivered 回報受眾 a 的分類 c 在渠道 ch 實際會不會有型別送達：entries 裡至少有一個型別的受眾、分類與渠道清單都對上，
// 而且 model.Delivers（分類本身允許的渠道）也放行。偏好 API 只列、只接受這樣的組合，所以沒有型別走 email 的分類沒有 email 那一項。
func Delivered(entries []Entry, a model.Audience, c model.PrefCategory, ch model.Channel) bool {
	if !model.Delivers(a, c, ch) {
		return false
	}
	return slices.ContainsFunc(entries, func(e Entry) bool {
		s := e.Spec()
		return s.Reaches(a) && s.Category == c && slices.Contains(s.Channels, ch)
	})
}

// TypeNames 回傳受眾 a 的型別名稱；category 不為 nil 時只取該分類。收件匣只顯示這些型別的列（已退場、沒有註冊的型別不顯示也不計數）。
func TypeNames(entries []Entry, a model.Audience, category *model.PrefCategory) []string {
	names := []string{}
	for _, e := range entries {
		s := e.Spec()
		if s.Reaches(a) && (category == nil || s.Category == *category) {
			names = append(names, s.Name)
		}
	}
	return names
}
