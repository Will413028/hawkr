// Package items 讀後台待辦：各來源發布的 pub_*_review_items 依期限由急到緩，keyset 分頁。
//
// 為什麼只有一個 SQL 語句：待辦是各模組的領域狀態推導出來的，不另存（step10e-3d-design.md §2.3）；排序在 SQL，
// 一個語句就是一個快照（E8），所以這裡不開 transaction，也不需要唯讀快照。
package items

import (
	"context"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Deps 是外部依賴。
type Deps struct {
	DB *db.DB
}

// Service 是待辦的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// List 回傳在 now 這一刻、種類屬於 kinds 的待辦，依 (期限, 開始, 種類, ID) 由小到大，從 after 之後（nil 是從頭）取最多 limit 件。
// limit 必須是正數；kinds 裡有無效的種類是 InvalidInput，kinds 是空的就沒有結果（展開成「全部」是呼叫端的事，因為它要先與權限取交集）。
// 跑在 ctx 的連線上（有 transaction 就加入，沒有就是連線池的一條連線），本身不開 transaction。
func (s *Service) List(ctx context.Context, now time.Time, kinds []model.Kind, after *model.Cursor, limit int32) ([]model.Item, error) {
	if limit <= 0 {
		return nil, apperr.InvalidInput("limit 必須是正數")
	}
	names := make([]string, 0, len(kinds))
	for _, k := range kinds {
		if !k.Valid() {
			return nil, apperr.InvalidInput("不認得的待辦種類 %q", string(k))
		}
		names = append(names, string(k))
	}
	arg := store.ListItemsParams{
		Now: now.Truncate(time.Microsecond), Kinds: names, Lim: limit,
	}
	if after != nil {
		if !after.Valid() {
			return nil, apperr.InvalidInput("游標不是有效的位置")
		}
		kind := string(after.Kind)
		arg.AfterDue, arg.AfterStart, arg.AfterKind, arg.AfterID = &after.Due, &after.Start, &kind, &after.ID
	}
	rows, err := store.New(s.d.DB.Conn(ctx)).ListItems(ctx, arg)
	if err != nil {
		return nil, fmt.Errorf("reviewqueue：讀待辦：%w", err)
	}
	out := make([]model.Item, 0, len(rows))
	for _, r := range rows {
		// 查詢的 WHERE 只放行 kinds（每一個都檢查過是有效的種類），所以回來的 kind 一定是已知的值；來源多出的種類在這裡就不會出現，
		// 不是錯誤，由 TestKindsMatchTheSource 在測試期把關。
		kind := model.Kind(r.Kind)
		item := model.Item{
			Kind: kind, ItemID: r.ItemID,
			ReportID: r.ReportID, CaseID: r.CaseID, InquiryID: r.InquiryID, AuthorityOrderID: r.AuthorityOrderID,
			TargetPosition: r.TargetPosition, AccountID: r.AccountID, MeasureID: r.MeasureID, RequestID: r.RequestID,
			KYCApplicationID: r.KycApplicationID,
			StoreID:          r.StoreID, Start: r.StartAt, Due: r.DueAt,
		}
		if r.StoreName != nil {
			item.StoreName = *r.StoreName
		}
		out = append(out, item)
	}
	return out, nil
}
