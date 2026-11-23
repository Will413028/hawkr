package images

import (
	"context"
	"slices"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/content/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type Service struct {
	Media model.Media
}

func New(mediaClient model.Media) *Service {
	return &Service{Media: mediaClient}
}

func NewSlot(field, token, current string) Slot {
	return Slot{Field: field, Token: token, Current: current}
}

// adminUploader 是管理員上傳圖片時的身分。content 的寫入都是後台操作。
func adminUploader(adminID int64) media.Uploader {
	return media.Uploader{ID: adminID, Admin: true}
}

// Slot 是一筆資料要寫入的一張圖。token 是新上傳的圖的 staged token，空字串表示沿用這筆資料
// 現在的圖（current）。field 是模組輸入的欄位名。
type Slot struct {
	Field   string
	Token   string
	Current string
}

// Resolved 是一張圖決定後的結果：Key 是寫入的 object key；AssetID 是新確認的資產，沿用現在的圖時是 nil。
type Resolved struct {
	Field   string
	Key     string
	AssetID *uuid.UUID
}

// Require 檢查每個位置都有圖（新的 token 或現有的圖），不碰 media；在 transaction 之前呼叫，驗證失敗就不開 transaction。
func Require(ctx context.Context, slots ...Slot) error {
	for _, s := range slots {
		if s.Token == "" && s.Current == "" {
			return apperr.InvalidInputOn([]string{s.Field}, "%s", media.ImageRequiredMessage(ctx))
		}
	}
	return nil
}

// Confirm 決定每張圖寫入的 object key，順序與 slots 相同；必須在寫入業務列的 transaction 內呼叫。
//
// 沿用的圖一律是模組自己讀到的現有 key：對外契約沒有 key（ADR 2026-09-25-api-image-uri-template
// D1、D5），client 只能換新或不換。有 token 的位置在這個 transaction 內 Confirm，任何一步失敗整筆回滾，新圖仍是暫存。
func (m *Service) Confirm(ctx context.Context, adminID int64, slots ...Slot) ([]Resolved, error) {
	if err := Require(ctx, slots...); err != nil {
		return nil, err
	}
	out := make([]Resolved, len(slots))
	for i, s := range slots {
		if s.Token == "" {
			out[i] = Resolved{Field: s.Field, Key: s.Current}
			continue
		}
		a, err := media.ConfirmField(ctx, m.Media, adminUploader(adminID), s.Field, s.Token, media.VisibilityPublic)
		if err != nil {
			return nil, err
		}
		id := a.ID
		out[i] = Resolved{Field: s.Field, Key: a.ObjectKey, AssetID: &id}
	}
	return out, nil
}

// HasNew 是 rs 之中有沒有新確認的資產。
func HasNew(rs ...Resolved) bool {
	return slices.ContainsFunc(rs, func(r Resolved) bool { return r.AssetID != nil })
}

// After 是寫入之後各個位置（與 rs 一一對應）指著的資產：新確認的取代 old 的同一個位置，沿用的保留 old。
func After(old []uuid.UUID, rs ...Resolved) []uuid.UUID {
	out := make([]uuid.UUID, len(rs))
	for i, r := range rs {
		out[i] = old[i]
		if r.AssetID != nil {
			out[i] = *r.AssetID
		}
	}
	return out
}

// Retired 列出退場的資產：寫入之前這個 ref 的所有資產減去寫入之後的所有資產（集合差，去重）。同一個 ref 的多個位置可以指著
// 同一個資產，所以不能逐位置比對：只換掉其中一個位置時，資產仍被別的位置指著，不退場。
func Retired(old, after []uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for _, a := range old {
		if !slices.Contains(after, a) && !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out
}
