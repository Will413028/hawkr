// Package style 是賣家看到的修圖風格：某個用途的啟用中風格與預設提示詞。後台的維護不在這裡。
package style

import (
	"context"
	"fmt"
	"slices"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Service 讀風格。
type Service struct{ db *db.DB }

// New 建立 Service。
func New(d *db.DB) *Service { return &Service{db: d} }

// List 列出 category 用途的啟用中風格（依顯示順序，預設風格不在清單裡），並帶預設風格的提示詞。
// category 不是 PRODUCT 或 POST 回 InvalidInput（欄位 Category）；用途沒有啟用中的預設風格是 schema 不變量被破壞，回沒有分類的錯誤。
// 沒有快取：一次索引查詢，多個 pod 之間不會不一致。
func (s *Service) List(ctx context.Context, category model.StyleCategory) (model.StyleList, error) {
	if !slices.Contains([]model.StyleCategory{model.CategoryProduct, model.CategoryPost}, category) {
		return model.StyleList{}, apperr.InvalidInputOn([]string{"Category"}, "%s", text.StyleCategoryInvalid(uitext.LocaleFrom(ctx)))
	}
	rows, err := store.New(s.db.Conn(ctx)).ListEnabledStyles(ctx, string(category))
	if err != nil {
		return model.StyleList{}, fmt.Errorf("style：讀 %s 的風格：%w", category, err)
	}
	var out model.StyleList
	haveDefault := false
	for _, r := range rows {
		if r.IsDefault {
			out.DefaultPrompt, haveDefault = r.Prompt, true
			continue
		}
		out.Styles = append(out.Styles, model.Style{
			ID: r.ID, Name: r.Name, Label: r.Label, Prompt: r.Prompt, ThumbnailKey: r.ThumbnailKey, DisplayOrder: r.DisplayOrder,
		})
	}
	if !haveDefault {
		return model.StyleList{}, fmt.Errorf("style：用途 %s 沒有啟用中的預設風格（schema 不變量被破壞）", category)
	}
	return out, nil
}
