// Package reviewqueue 是 reviewqueue 模組的公開 API。
//
// 職責、擁有的資料與不變量見同目錄的 AGENTS.md。
package reviewqueue

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue/internal/items"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Deps 是模組需要的外部依賴；時鐘不在這裡：now 是每次呼叫的參數，整個請求一個 now，由呼叫端（handler 注入的 clock）決定。
type Deps struct {
	DB *db.DB
}

// Module 是 reviewqueue 模組。
type Module struct {
	items *items.Service
}

// New 建立模組。
func New(d Deps) *Module {
	if d.DB == nil {
		panic("reviewqueue: missing dependency")
	}
	return &Module{items: items.New(items.Deps{DB: d.DB})}
}

// Kind、Item、Cursor 是待辦的種類、一列與 keyset 的位置（見 internal/model）。
type (
	Kind   = model.Kind
	Item   = model.Item
	Cursor = model.Cursor
)

// 待辦的種類：seller 發布的 12 種。
const (
	ReportTriage          = model.ReportTriage
	CaseDecision          = model.CaseDecision
	CaseInquiryOverdue    = model.CaseInquiryOverdue
	CaseFDAReferral       = model.CaseFDAReferral
	AuthorityOrder        = model.AuthorityOrder
	AuthorityTargetNotice = model.AuthorityTargetNotice
	MeasureAppeal         = model.MeasureAppeal
	MeasureLiftRequest    = model.MeasureLiftRequest
	MeasureReview         = model.MeasureReview
	FoodRegistration      = model.FoodRegistration
	KYCApplication        = model.KYCApplication
	PayUniBindRetry       = model.PayUniBindRetry
)

// Kinds 回傳全部有效的種類（新的切片，呼叫端可以改）。
func Kinds() []Kind { return append([]Kind(nil), model.Kinds...) }

// CursorOf 回傳 item 在全序 (期限, 開始, 種類, ID) 裡的位置，給下一頁當 after。
func CursorOf(item Item) Cursor { return model.CursorOf(item) }

// List 回傳在 now 這一刻、種類屬於 kinds 的待辦，依期限由急到緩（期限、開始時間、種類、ID 的全序），從 after 之後取最多 limit 件。
// kinds 為空沒有結果，limit 必須是正數，種類無效是 InvalidInput。跑在 ctx 的連線上，不開 transaction（一個語句一個快照）。
func (m *Module) List(ctx context.Context, now time.Time, kinds []Kind, after *Cursor, limit int32) ([]Item, error) {
	return m.items.List(ctx, now, kinds, after, limit)
}
