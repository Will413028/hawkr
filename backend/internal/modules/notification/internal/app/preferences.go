package app

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Preference 是收件人一個 (分類, 渠道) 的偏好：Enabled 是目前的開關（沒有覆寫列是預設），CanDisable 為 false 時 Enabled 一定是 true。
type Preference struct {
	Category   model.PrefCategory
	Channel    model.Channel
	Enabled    bool
	CanDisable bool
}

// Preferences 列出收件人的偏好：只有註冊表實際會在該 (受眾, 分類, 渠道) 送達的組合（registry.Delivered），順序是分類再渠道。
func (s *Service) Preferences(ctx context.Context, r Recipient) ([]Preference, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	rows, err := store.New(s.d.DB.Conn(ctx)).PreferenceOverridesForRecipient(ctx, store.PreferenceOverridesForRecipientParams{
		AccountID: r.AccountID, Audience: string(r.Audience),
	})
	if err != nil {
		return nil, fmt.Errorf("讀取偏好：%w", err)
	}
	override := make(map[[2]string]bool, len(rows))
	for _, o := range rows {
		override[[2]string{o.Category, o.Channel}] = o.Enabled
	}
	entries := s.d.Registry.All()
	var out []Preference
	for _, c := range model.CategoriesFor(r.Audience) {
		for _, ch := range model.Channels() {
			if !registry.Delivered(entries, r.Audience, c, ch) {
				continue
			}
			out = append(out, effective(r.Audience, c, ch, override))
		}
	}
	return out, nil
}

func effective(a model.Audience, c model.PrefCategory, ch model.Channel, override map[[2]string]bool) Preference {
	p := Preference{Category: c, Channel: ch, Enabled: true, CanDisable: model.Disableable(a, c, ch)}
	if p.CanDisable {
		if v, ok := override[[2]string{string(c), string(ch)}]; ok {
			p.Enabled = v
		} else {
			p.Enabled = model.DefaultEnabled(c, ch)
		}
	}
	return p
}

// UpdatePreference 設定一個 (分類, 渠道) 的開關並回傳更新後的那一項。分類不屬於受眾、(分類, 渠道) 不是註冊表實際送達的組合，
// 或要關閉不可關閉的組合，回 invalid_argument；重複設定同一個值成功。設回預設值時不留覆寫列。
func (s *Service) UpdatePreference(ctx context.Context, r Recipient, c model.PrefCategory, ch model.Channel, enabled bool) (Preference, error) {
	if err := r.validate(); err != nil {
		return Preference{}, err
	}
	l := uitext.LocaleFrom(ctx)
	if !model.ValidCategoryFor(r.Audience, c) {
		return Preference{}, apperr.InvalidInputOn([]string{"Category"}, "%s", text.PreferenceNotAvailable(l))
	}
	if !registry.Delivered(s.d.Registry.All(), r.Audience, c, ch) {
		return Preference{}, apperr.InvalidInputOn([]string{"Channel"}, "%s", text.PreferenceNotAvailable(l))
	}
	if !model.Disableable(r.Audience, c, ch) {
		if !enabled {
			return Preference{}, apperr.InvalidInputOn([]string{"Enabled"}, "%s", text.PreferenceCannotDisable(l))
		}
		return Preference{Category: c, Channel: ch, Enabled: true}, nil
	}
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.lockAccount(ctx, r.AccountID); err != nil {
			return err
		}
		q := store.New(s.d.DB.Conn(ctx))
		if enabled == model.DefaultEnabled(c, ch) {
			return q.DeletePreferenceOverride(ctx, store.DeletePreferenceOverrideParams{
				AccountID: r.AccountID, Audience: string(r.Audience), Category: string(c), Channel: string(ch),
			})
		}
		return q.UpsertPreferenceOverride(ctx, store.UpsertPreferenceOverrideParams{
			AccountID: r.AccountID, Audience: string(r.Audience), Category: string(c), Channel: string(ch), Enabled: enabled, Now: s.d.Now(),
		})
	})
	if err != nil {
		return Preference{}, fmt.Errorf("寫入偏好：%w", err)
	}
	return Preference{Category: c, Channel: ch, Enabled: enabled, CanDisable: true}, nil
}
