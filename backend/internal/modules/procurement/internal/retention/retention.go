// Package retention 處理買家關閉後的採購資料（A9-D19 S10）：買家資料刪除，賣家看得到的紀錄保留並把 buyer_id 設 NULL。
package retention

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Service 清除買家的採購資料。
type Service struct{ db *db.DB }

// New 建立 Service。
func New(d *db.DB) *Service { return &Service{db: d} }

// OnBuyerDeleted 在 BuyerDeleted 的 transaction 內執行（單體規則 4：同 transaction 的訂閱者只做原子的狀態變更）：
//
//   - 刪除：對話與輪次、run、候選與候選清單、報價清單、沒有發布過的需求（連帶 revision、預覽、快照、核准）、額度、ADK session（graph 的 checkpoint）。
//   - 保留並匿名：已發布的需求，與它下游賣家看得到的紀錄（預覽、快照、核准、發布、邀請、回價、接受單）；buyer_id 設 NULL，保留期跟詢價的生命週期。
//
// 全部冪等：重送時已沒有列可動。
func (s *Service) OnBuyerDeleted(ctx context.Context, buyerID int64) error {
	q := store.New(s.db.Conn(ctx))
	steps := []struct {
		name string
		run  func() (int64, error)
	}{
		{"對話", func() (int64, error) { return q.DeleteBuyerConversations(ctx, buyerID) }},
		{"run", func() (int64, error) { return q.DeleteBuyerRuns(ctx, buyerID) }},
		{"候選集", func() (int64, error) { return q.DeleteBuyerCandidateSets(ctx, buyerID) }},
		{"報價清單", func() (int64, error) { return q.DeleteBuyerShortlists(ctx, &buyerID) }},
		{"未發布的需求", func() (int64, error) { return q.DeleteUnpublishedBuyerRequests(ctx, &buyerID) }},
		{"預覽", func() (int64, error) { return q.AnonymizeBuyerPricePreviews(ctx, &buyerID) }},
		{"快照", func() (int64, error) { return q.AnonymizeBuyerSnapshots(ctx, &buyerID) }},
		{"核准", func() (int64, error) { return q.AnonymizeBuyerApprovals(ctx, &buyerID) }},
		{"發布", func() (int64, error) { return q.AnonymizeBuyerPublications(ctx, &buyerID) }},
		{"接受單", func() (int64, error) { return q.AnonymizeBuyerAcceptances(ctx, &buyerID) }},
		{"已發布的需求", func() (int64, error) { return q.AnonymizeBuyerRequests(ctx, &buyerID) }},
		{"額度", func() (int64, error) { return q.DeleteBuyerUsage(ctx, buyerID) }},
		{"ADK session", func() (int64, error) { return q.DeleteBuyerSessions(ctx, strconv.FormatInt(buyerID, 10)) }},
		{"ADK user state", func() (int64, error) { return q.DeleteBuyerUserStates(ctx, strconv.FormatInt(buyerID, 10)) }},
	}
	for _, st := range steps {
		if _, err := st.run(); err != nil {
			return fmt.Errorf("retention：清除買家 %d 的%s：%w", buyerID, st.name, err)
		}
	}
	return nil
}
