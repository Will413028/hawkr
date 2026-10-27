package images

import (
	"context"
	"fmt"
	"slices"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type Service struct {
	Media media.Media
}

func New(mediaClient media.Media) *Service {
	return &Service{Media: mediaClient}
}

func NewSlot(field, label, token, current string) Slot {
	return Slot{Field: field, Label: label, Token: token, Current: current}
}

// adminUploader 是管理員上傳圖片時的身分。content 的寫入都是後台操作。
func adminUploader(adminID int64) media.Uploader {
	return media.Uploader{ID: adminID, Admin: true}
}

// Slot 是一筆資料要寫入的一張圖。token 是新上傳的圖的 staged token，空字串表示沿用這筆資料
// 現在的圖（current）。field 是模組輸入的欄位名，label 是給使用者看的名稱。
type Slot struct {
	Field   string
	Label   string
	Token   string
	Current string
}

// Resolved 是一張圖決定後的結果：Key 是寫入的 object key；Asset 是新轉正的資產，沿用現在的圖時是 nil。
type Resolved struct {
	Field string
	Key   string
	Asset *media.Asset
}

// Resolve 決定每張圖寫入的 object key，順序與 slots 相同。
//
// 沿用的圖一律是模組自己讀到的現有 key：對外契約沒有 key（ADR 2026-09-25-api-image-uri-template
// D1、D5），client 只能換新或不換。所有沒有圖的欄位先檢查完，才轉正新上傳的圖，不會轉正到一半才因為
// 另一張不合法而失敗。轉正不建立引用：新圖的引用由寫入業務列的 transaction 排入的同步 job 登記（mediaref）。
func (m *Service) Resolve(ctx context.Context, adminID int64, refType string, slots ...Slot) ([]Resolved, error) {
	for _, s := range slots {
		if s.Token == "" && s.Current == "" {
			return nil, apperr.InvalidInputOn([]string{s.Field}, "%s 必須上傳", s.Label)
		}
	}
	out := make([]Resolved, len(slots))
	for i, s := range slots {
		if s.Token == "" {
			out[i] = Resolved{Field: s.Field, Key: s.Current}
			continue
		}
		// 給使用者的文字與欄位由知道是哪一張圖的這一層寫；media 的原因留在錯誤的文字裡，只進 log。
		a, err := media.FinalizeField(ctx, m.Media, adminUploader(adminID), media.Field{Name: s.Field, Message: s.Label + "無法使用，請重新上傳"}, s.Token, refType)
		if err != nil {
			return nil, fmt.Errorf("content：轉正 %s：%w", s.Field, err)
		}
		out[i] = Resolved{Field: s.Field, Key: a.ObjectKey, Asset: &a}
	}
	return out, nil
}

// AssetPtr 是新轉正的資產的 ID；沿用現在的圖時是 nil（業務列保留原本的 asset ID）。
func (r Resolved) AssetPtr() *string {
	if r.Asset == nil {
		return nil
	}
	return &r.Asset.ID
}

// NewCandidates 列出 rs 之中新轉正的資產與它們的輸入欄位，給 mediaref.Reject。
func NewCandidates(rs ...Resolved) []media.SyncCandidate {
	var out []media.SyncCandidate
	for _, r := range rs {
		if r.Asset != nil {
			out = append(out, media.SyncCandidate{Field: r.Field, AssetID: r.Asset.ID})
		}
	}
	return out
}

// NewAssetIDs 列出 rs 之中新轉正的資產 ID。
func NewAssetIDs(rs ...Resolved) []string {
	var out []string
	for _, r := range rs {
		if r.Asset != nil {
			out = append(out, r.Asset.ID)
		}
	}
	return out
}

// After 是寫入之後各個位置（與 rs 一一對應）指著的資產：新轉正的取代 old 的同一個位置，沿用的保留 old。
func After(old []string, rs ...Resolved) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = old[i]
		if r.Asset != nil {
			out[i] = r.Asset.ID
		}
	}
	return out
}

// Retired 列出退場的資產：寫入之前這個 ref 的所有資產減去寫入之後的所有資產（集合差，去重）。同一個 ref 的多個位置可以指著
// 同一個資產，所以不能逐位置比對：只換掉其中一個位置時，資產仍被別的位置指著，不退場。
func Retired(old, after []string) []string {
	var out []string
	for _, a := range old {
		if !slices.Contains(after, a) && !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out
}
