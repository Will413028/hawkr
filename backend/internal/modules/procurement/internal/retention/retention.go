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

// step 是清除的一步；tables 是它處理的含 buyer_id 的表（守門測試用它對 schema 檢查）。
type step struct {
	name   string
	tables []string
	run    func(ctx context.Context, q *store.Queries, buyerID int64) (int64, error)
}

// exempt 是含 buyer_id、但不需要自己的清除步驟的表，值是理由。
var exempt = map[string]string{
	"turns": "對話與需求刪除時 ON DELETE CASCADE 連帶刪除（conversation_id、request_id 都是 CASCADE）",
}

func sessionKey(buyerID int64) string { return strconv.FormatInt(buyerID, 10) }

var steps = []step{
	// run 最先刪（A9-D20 R4）：子表（輸入、事件、產出、interrupt）隨 run CASCADE；agent_runs.buyer_id 維持 NOT NULL，
	// 所以不能留到匿名步驟，agent_runs.publication_id 是 ON DELETE SET NULL，之後的步驟不再碰到 run。
	{"run", []string{"agent_runs"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) { return q.DeleteBuyerRuns(ctx, b) }},
	{"對話", []string{"conversations"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.DeleteBuyerConversations(ctx, b)
	}},
	{"候選集", []string{"candidate_sets"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.DeleteBuyerCandidateSets(ctx, b)
	}},
	{"報價清單", nil, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.DeleteBuyerShortlists(ctx, &b)
	}},
	{"未發布的需求", []string{"requests"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.DeleteUnpublishedBuyerRequests(ctx, &b)
	}},
	{"預覽", []string{"price_previews"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.AnonymizeBuyerPricePreviews(ctx, &b)
	}},
	{"快照", []string{"confirmed_rfq_snapshots"}, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.AnonymizeBuyerSnapshots(ctx, &b)
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
	{"ADK session", nil, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.DeleteBuyerSessions(ctx, sessionKey(b))
	}},
	{"ADK user state", nil, func(ctx context.Context, q *store.Queries, b int64) (int64, error) {
		return q.DeleteBuyerUserStates(ctx, sessionKey(b))
	}},
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
//   - 刪除：對話與輪次、run、候選與候選清單、報價清單、沒有發布過的需求（連帶 revision、預覽、快照、核准）、額度、ADK session（graph 的 checkpoint）。
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
