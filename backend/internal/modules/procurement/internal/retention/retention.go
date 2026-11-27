// Package retention 處理買家關閉後的採購資料（A9-D19 S10）：買家資料刪除，賣家看得到的紀錄保留並把 buyer_id 設 NULL。
package retention

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Service 清除買家的採購資料。
type Service struct{ db *db.DB }

// New 建立 Service。
func New(d *db.DB) *Service { return &Service{db: d} }

type step struct {
	name   string
	tables []string
	run    func(ctx context.Context, q *store.Queries, buyerID int64) (int64, error)
}

var exempt = map[string]string{
	"turns": "對話與需求刪除時 ON DELETE CASCADE 連帶刪除（conversation_id、request_id 都是 CASCADE）",
}

var steps = []step{
	{"run", []string{"agent_runs"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) { return q.DeleteBuyerRuns(ctx, b) }},
	{"對話", []string{"conversations"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.DeleteBuyerConversations(ctx, b)
	}},
	{"報價清單", nil, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.DeleteBuyerShortlists(ctx, &b)
	}},
	{"未發布的需求", []string{"requests"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.DeleteUnpublishedBuyerRequests(ctx, &b)
	}},
	{"候選集", []string{"candidate_sets"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		n, err := q.DeleteBuyerCandidateSets(ctx, b)
		if err != nil {
			return 0, err
		}
		m, err := q.AnonymizeBuyerCandidateSets(ctx, &b)
		return n + m, err
	}},
	{"核准", []string{"rfq_publication_approvals"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.AnonymizeBuyerApprovals(ctx, &b)
	}},
	{"發布", []string{"rfq_publications"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.AnonymizeBuyerPublications(ctx, &b)
	}},
	{"接受單", []string{"buyer_quote_acceptances"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.AnonymizeBuyerAcceptances(ctx, &b)
	}},
	{"已發布的需求", []string{"requests"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.AnonymizeBuyerRequests(ctx, &b)
	}},
	{"額度", []string{"usage"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) { return q.DeleteBuyerUsage(ctx, b) }},
}

// Covered 回傳有清除步驟或明列豁免的表（含 buyer_id 的表都必須在裡面）。
func Covered() map[string]string {
	out := map[string]string{}
	for _, st := range steps {
		for _, t := range st.tables {
			out[t] = st.name
		}
	}
	for t, why := range exempt {
		out[t] = "豁免：" + why
	}
	return out
}

// OnBuyerDeleted 在 BuyerDeleted 的 transaction 內執行（單體規則 4：同 transaction 的訂閱者只做原子的狀態變更）：
//
//   - 刪除：對話與輪次、run、候選與候選清單、報價清單、沒有發布過的需求（連帶 revision、預覽、快照、核准）、額度。
//   - 保留並匿名：已發布的需求，與它下游賣家看得到的紀錄（預覽、快照、核准、發布、邀請、回價、接受單）；buyer_id 設 NULL，保留期跟詢價的生命週期。
//
// 全部冪等：重送時已沒有列可動。
func (s *Service) OnBuyerDeleted(ctx context.Context, buyerID int64) error {
	q := store.New(s.db.Conn(ctx))
	for _, st := range steps {
		if _, err := st.run(ctx, q, buyerID); err != nil {
			return fmt.Errorf("retention：清除買家 %d 的%s：%w", buyerID, st.name, err)
		}
	}
	return nil
}
