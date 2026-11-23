package admin

import (
	"context"
	"fmt"
	"unicode/utf8"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// ListStyles 列出風格（含停用的），依顯示順序；category 為 nil 是全部用途，不合法回 InvalidInput（欄位 Category）。
func (s *Service) ListStyles(ctx context.Context, category *model.StyleCategory) ([]model.AdminStyle, error) {
	filter := ""
	if category != nil {
		if !validCategory(*category) {
			return nil, apperr.InvalidInputOn([]string{"Category"}, "%s", text.StyleCategoryInvalid(loc(ctx)))
		}
		filter = string(*category)
	}
	rows, err := s.d.Q(ctx).ListStylesForAdmin(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("admin：列出風格：%w", err)
	}
	out := make([]model.AdminStyle, len(rows))
	for i, r := range rows {
		out[i] = toStyle(r)
	}
	return out, nil
}

// Style 回傳一個風格（後台），不存在回 NotFound。
func (s *Service) Style(ctx context.Context, id uuid.UUID) (model.AdminStyle, error) {
	row, err := s.d.Q(ctx).GetStyle(ctx, id)
	if notFound(err) {
		return model.AdminStyle{}, styleNotFound(ctx)
	}
	if err != nil {
		return model.AdminStyle{}, fmt.Errorf("admin：讀取風格 %s：%w", id, err)
	}
	return toStyle(row), nil
}

// validateText 檢查並正規化風格的文字欄位（頭尾空白只有 trimtext 一個定義）：label 與 prompt 去空白後不可空白，prompt 以字元計 1 到 MaxPromptRunes（A9-D40）。
func validateText(ctx context.Context, label, prompt string) (string, string, error) {
	l := loc(ctx)
	label = trimtext.Trim(label)
	if label == "" {
		return "", "", apperr.InvalidInputOn([]string{"Label"}, "%s", text.AdminStyleLabelRequired(l))
	}
	prompt = trimtext.Trim(prompt)
	if n := utf8.RuneCountInString(prompt); n == 0 || n > model.MaxPromptRunes {
		return "", "", apperr.InvalidInputOn([]string{"Prompt"}, "%s", text.TransformPromptLength(l, text.TransformPromptLengthArgs{Max: model.MaxPromptRunes}))
	}
	return label, prompt, nil
}

// CreateStyle 新增風格：縮圖必須是新上傳的圖（staged token）。設成預設時，同一個 transaction 內先取消同用途原本的預設。
// name 在用途內重複回 AlreadyExists。
func (s *Service) CreateStyle(ctx context.Context, in model.CreateStyleInput) (model.AdminStyle, error) {
	l := loc(ctx)
	if !validCategory(in.Category) {
		return model.AdminStyle{}, apperr.InvalidInputOn([]string{"Category"}, "%s", text.StyleCategoryInvalid(l))
	}
	name := trimtext.Trim(in.Name)
	if name == "" {
		return model.AdminStyle{}, apperr.InvalidInputOn([]string{"Name"}, "%s", text.AdminStyleNameRequired(l))
	}
	label, prompt, err := validateText(ctx, in.Label, in.Prompt)
	if err != nil {
		return model.AdminStyle{}, err
	}
	if in.IsDefault && !in.Enabled {
		return model.AdminStyle{}, apperr.InvalidInputOn([]string{"Enabled"}, "%s", text.AdminStyleDefaultMustBeEnabled(l))
	}
	if in.ThumbnailToken == "" {
		return model.AdminStyle{}, apperr.InvalidInputOn([]string{"ThumbnailToken"}, "%s", text.AdminStyleThumbnailRequired(l))
	}
	// 先驗證再轉正：驗證失敗時不該讓圖片掛在一個永遠不會建立的風格上。
	key, asset, err := s.d.Thumbs.Finalize(ctx, in.Actor.AdminID, in.ThumbnailToken, text.AdminStyleThumbnailUnusable(l).Text)
	if err != nil {
		return model.AdminStyle{}, err
	}
	id := s.d.NewID()
	var row store.ImageaiStyle
	err = s.d.Tx(ctx, func(ctx context.Context) error {
		now := s.d.Now().UTC()
		q := s.d.Q(ctx)
		if err := s.d.Thumbs.Reject(ctx, id, asset); err != nil {
			return err
		}
		if in.IsDefault {
			if _, err := q.ClearDefaultStyle(ctx, store.ClearDefaultStyleParams{Now: now, Category: string(in.Category), ExcludeID: id}); err != nil {
				return fmt.Errorf("admin：取消 %s 原本的預設風格：%w", in.Category, err)
			}
		}
		var err error
		row, err = q.InsertStyle(ctx, store.InsertStyleParams{
			ID: id, Name: name, Label: label, Prompt: prompt, Category: string(in.Category), ThumbnailKey: key, ThumbnailAssetID: asset,
			DisplayOrder: in.DisplayOrder, IsDefault: in.IsDefault, Enabled: in.Enabled, Now: now,
		})
		switch {
		case isUnique(err, nameConstraint):
			return apperr.AlreadyExists("%s", text.AdminStyleNameTaken(l))
		case isUnique(err, defaultConstraint):
			return styleStale(ctx)
		case err != nil:
			return fmt.Errorf("admin：新增風格：%w", err)
		}
		if err := s.d.Thumbs.Changed(ctx, now, id); err != nil {
			return err
		}
		return s.record(ctx, in.Actor, styleTarget, id.String(), map[string]any{"name": name, "category": string(in.Category), "default_style": in.IsDefault, "enabled": in.Enabled})
	})
	if err != nil {
		return model.AdminStyle{}, err
	}
	return toStyle(row), nil
}

// UpdateStyle 整筆覆寫風格可改的欄位（name、category 建立後不能改）。Version 是讀取時拿到的版本，與目前的不符回 VersionConflict、不寫入；
// 轉正新圖之前先比對一次，少一些注定被拒的轉正，最後以條件式 UPDATE 為準。
// 預設風格：default_style 只能設成 true（原本的預設在同一個 transaction 內取消），目前是預設的風格設成 false 回 InvalidInput（欄位 IsDefault），
// 停用目前的預設風格回 FailedPrecondition（A9-D39）。
func (s *Service) UpdateStyle(ctx context.Context, in model.UpdateStyleInput) (model.AdminStyle, error) {
	l := loc(ctx)
	if in.Version <= 0 {
		return model.AdminStyle{}, apperr.InvalidInputOn([]string{"Version"}, "%s", text.AdminStyleStale(l))
	}
	label, prompt, err := validateText(ctx, in.Label, in.Prompt)
	if err != nil {
		return model.AdminStyle{}, err
	}
	cur, err := s.d.Q(ctx).GetStyle(ctx, in.ID)
	if notFound(err) {
		return model.AdminStyle{}, styleNotFound(ctx)
	}
	if err != nil {
		return model.AdminStyle{}, fmt.Errorf("admin：讀取風格 %s：%w", in.ID, err)
	}
	if cur.Version != in.Version {
		return model.AdminStyle{}, styleStale(ctx)
	}
	if err := checkDefaultRules(ctx, cur.IsDefault, in.IsDefault, in.Enabled); err != nil {
		return model.AdminStyle{}, err
	}
	var newKey, newAsset string
	if in.ThumbnailToken != "" {
		newKey, newAsset, err = s.d.Thumbs.Finalize(ctx, in.Actor.AdminID, in.ThumbnailToken, text.AdminStyleThumbnailUnusable(l).Text)
		if err != nil {
			return model.AdminStyle{}, err
		}
	}
	var row store.ImageaiStyle
	err = s.d.Tx(ctx, func(ctx context.Context) error {
		now := s.d.Now().UTC()
		q := s.d.Q(ctx)
		old, err := q.LockStyle(ctx, in.ID)
		if notFound(err) {
			return styleNotFound(ctx)
		}
		if err != nil {
			return fmt.Errorf("admin：鎖定風格 %s：%w", in.ID, err)
		}
		key, asset := old.ThumbnailKey, old.ThumbnailAssetID
		if newAsset != "" {
			if err := s.d.Thumbs.Reject(ctx, in.ID, newAsset); err != nil {
				return err
			}
			key, asset = newKey, newAsset
		}
		if in.IsDefault && !old.IsDefault {
			if _, err := q.ClearDefaultStyle(ctx, store.ClearDefaultStyleParams{Now: now, Category: old.Category, ExcludeID: in.ID}); err != nil {
				return fmt.Errorf("admin：取消 %s 原本的預設風格：%w", old.Category, err)
			}
		}
		row, err = q.UpdateStyle(ctx, store.UpdateStyleParams{
			ID: in.ID, Label: label, Prompt: prompt, ThumbnailKey: key, ThumbnailAssetID: asset, DisplayOrder: in.DisplayOrder,
			IsDefault: in.IsDefault, Enabled: in.Enabled, Now: now, ExpectedVersion: in.Version,
		})
		switch {
		case notFound(err):
			return styleStale(ctx)
		case isUnique(err, defaultConstraint):
			return styleStale(ctx)
		case err != nil:
			return fmt.Errorf("admin：修改風格 %s：%w", in.ID, err)
		}
		if newAsset != "" {
			var retired []string
			if old.ThumbnailAssetID != "" && old.ThumbnailAssetID != newAsset {
				retired = []string{old.ThumbnailAssetID}
			}
			if err := s.d.Thumbs.Changed(ctx, now, in.ID, retired...); err != nil {
				return err
			}
		}
		return s.record(ctx, in.Actor, styleTarget, in.ID.String(), map[string]any{"version": row.Version, "changed": changedFields(old, row, newAsset != "")})
	})
	if err != nil {
		return model.AdminStyle{}, err
	}
	return toStyle(row), nil
}

// checkDefaultRules 是預設風格的規則（A9-D39）：default_style 只能設成 true；目前的預設不能取消或停用；預設風格必須啟用。
func checkDefaultRules(ctx context.Context, curDefault, wantDefault, wantEnabled bool) error {
	l := loc(ctx)
	switch {
	case curDefault && !wantDefault:
		return apperr.InvalidInputOn([]string{"IsDefault"}, "%s", text.AdminStyleDefaultCannotUnset(l))
	case curDefault && !wantEnabled:
		return apperr.FailedPrecondition("%s", text.AdminStyleDefaultMustBeEnabled(l))
	case wantDefault && !wantEnabled:
		return apperr.InvalidInputOn([]string{"Enabled"}, "%s", text.AdminStyleDefaultMustBeEnabled(l))
	}
	return nil
}

// changedFields 列出這次修改實際改變的欄位名（稽核的 detail；不放提示詞的內容）。
func changedFields(old, now store.ImageaiStyle, thumbnail bool) []string {
	var out []string
	for _, c := range []struct {
		name    string
		changed bool
	}{
		{"label", old.Label != now.Label}, {"prompt", old.Prompt != now.Prompt}, {"display_order", old.DisplayOrder != now.DisplayOrder},
		{"default_style", old.IsDefault != now.IsDefault}, {"enabled", old.Enabled != now.Enabled}, {"thumbnail", thumbnail},
	} {
		if c.changed {
			out = append(out, c.name)
		}
	}
	return out
}

// DeleteStyle 刪除風格並把它的縮圖記成退場、排入引用的同步。version 為 0 表示不檢查版本；不為 0 時必須與目前的相符。
// 用途的預設風格不能刪（A9-D39，FailedPrecondition）。
func (s *Service) DeleteStyle(ctx context.Context, actor model.Actor, id uuid.UUID, version int64) error {
	return s.d.Tx(ctx, func(ctx context.Context) error {
		now := s.d.Now().UTC()
		q := s.d.Q(ctx)
		old, err := q.LockStyle(ctx, id)
		if notFound(err) {
			return styleNotFound(ctx)
		}
		if err != nil {
			return fmt.Errorf("admin：鎖定風格 %s：%w", id, err)
		}
		if version != 0 && old.Version != version {
			return styleStale(ctx)
		}
		if old.IsDefault {
			return apperr.FailedPrecondition("%s", text.AdminStyleDefaultCannotDelete(loc(ctx)))
		}
		if _, err := q.DeleteStyle(ctx, id); err != nil {
			return fmt.Errorf("admin：刪除風格 %s：%w", id, err)
		}
		var retired []string
		if old.ThumbnailAssetID != "" {
			retired = []string{old.ThumbnailAssetID}
		}
		if err := s.d.Thumbs.Changed(ctx, now, id, retired...); err != nil {
			return err
		}
		return s.record(ctx, actor, styleTarget, id.String(), map[string]any{"name": old.Name, "category": old.Category})
	})
}
