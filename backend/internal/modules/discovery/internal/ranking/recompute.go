package ranking

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/modules/discovery/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/discovery/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// Result 是一次重算的結果。
type Result struct {
	Candidates int  // 合格店家數（有公開商品且可曝光），也是這次算出的列數
	Written    bool // 結果有沒有寫進快照；false 表示已有更晚開始的重算成功，這次的結果被丟棄（不是錯誤）
}

// Recompute 重算全部合格店家的分數並整批取代快照。流程：
//  1. 記下這次嘗試的開始時間（recompute_state）；
//  2. product port 取有公開商品的店家全集，與 seller 的曝光資格（分批，每批 seller.MaxBatchSize）取交集；
//  3. order port 一次取近 30 天的統計；
//  4. review port 批次取評價（每批 MaxRatingBatch 家）；
//  5. 以公式算分（internal/model），一個 transaction 鎖住狀態列、確認這次比目前快照的那次更晚開始，才刪除並寫入整份快照。
//
// 外部呼叫都在 transaction 之外。任何上游失敗都回 error（River 重試），上一份快照原封不動，失敗的時間與原因記進
// recompute_state；不把失敗的評價或統計換成 0 或預設值。上游回傳違反契約的資料（沒問的店家、不合法的數值）同樣讓整次
// 失敗，不靜默排除單一店家：批次的上游沒有「某一家失敗」的情況，違反契約代表上游有 bug，要大聲失敗，舊快照照舊服務。
// 重疊或重試時，開始得比目前快照更早的結果會被丟棄（見 replace）。log 在 error 層級並標明是哪個上游。
func (s *Service) Recompute(ctx context.Context) (Result, error) {
	started := s.d.Now().UTC().Truncate(time.Microsecond)
	if err := store.New(s.d.DB.Conn(ctx)).RecordRecomputeAttempt(ctx, store.RecordRecomputeAttemptParams{Job: scoresJob, StartedAt: started}); err != nil {
		return Result{}, s.fail(ctx, started, "state", err)
	}

	withProducts, err := s.d.Products.StoresWithPublicProducts(ctx, started)
	if err != nil {
		return Result{}, s.fail(ctx, started, "product", err)
	}
	publishedAt, err := s.exposable(ctx, withProducts)
	if err != nil {
		return Result{}, s.fail(ctx, started, "seller", err)
	}
	ids := make([]int64, 0, len(publishedAt))
	for id := range publishedAt {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	orderStats, err := s.d.Orders.StoreOrderStats(ctx, model.StatsWindowDays)
	if err != nil {
		return Result{}, s.fail(ctx, started, "order", err)
	}
	statsByStore := make(map[int64]model.OrderStats, len(orderStats))
	for _, o := range orderStats {
		statsByStore[o.StoreID] = o
	}

	ratings, err := s.ratings(ctx, ids)
	if err != nil {
		return Result{}, s.fail(ctx, started, "review", err)
	}

	inputs := make([]model.Input, len(ids))
	for i, id := range ids {
		o := statsByStore[id]
		inputs[i] = model.Input{
			StoreID: id, PublishedAt: publishedAt[id],
			DistinctBuyers: o.DistinctBuyers, QualifyingOrders: o.QualifyingOrders, SellerCancelledOrders: o.SellerCancelledOrders,
			RatingCount: ratings[i].Count, RatingAverage: ratings[i].Average,
		}
	}
	scores, err := model.Compute(inputs, started)
	if err != nil {
		return Result{}, s.fail(ctx, started, "compute", err)
	}
	written, err := s.replace(ctx, scores, started)
	if err != nil {
		return Result{}, s.fail(ctx, started, "store", err)
	}
	if !written {
		s.d.Logger.WarnContext(ctx, "discovery：重算的結果被丟棄，已有更晚開始的重算成功", "started_at", started)
	}
	return Result{Candidates: len(scores), Written: written}, nil
}

const failureRecordTimeout = 5 * time.Second

const maxErrorLen = 500

func (s *Service) fail(ctx context.Context, started time.Time, upstream string, err error) error {
	s.d.Logger.ErrorContext(ctx, "discovery：重算推薦賣家失敗，保留上一份快照", "upstream", upstream, "error", err)
	wrapped := fmt.Errorf("discovery：重算（%s）：%w", upstream, err)
	failedAt := s.d.Now().UTC().Truncate(time.Microsecond)
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failureRecordTimeout)
	defer cancel()
	if recErr := store.New(s.d.DB.Conn(rctx)).RecordRecomputeFailure(rctx, store.RecordRecomputeFailureParams{
		Job: scoresJob, StartedAt: started, FailedAt: &failedAt, Error: truncate(wrapped.Error(), maxErrorLen),
	}); recErr != nil {
		s.d.Logger.ErrorContext(ctx, "discovery：記錄重算失敗時再失敗", "upstream", upstream, "error", recErr)
	}
	return wrapped
}

func truncate(text string, n int) string {
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	return string([]rune(text)[:n-1]) + "…"
}

func (s *Service) exposable(ctx context.Context, ids []int64) (map[int64]time.Time, error) {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	asked := make(map[int64]bool, len(ids))
	for _, id := range ids {
		asked[id] = true
	}
	out := make(map[int64]time.Time, len(ids))
	for batch := range slices.Chunk(ids, seller.MaxBatchSize) {
		got, err := s.d.Stores.ExposableStores(ctx, batch)
		if err != nil {
			return nil, err
		}
		for _, e := range got {
			if !asked[e.StoreID] {
				return nil, fmt.Errorf("seller 回傳了沒有詢問的店家 %d", e.StoreID)
			}
			out[e.StoreID] = e.PublishedAt
		}
	}
	return out, nil
}

func (s *Service) ratings(ctx context.Context, ids []int64) ([]model.RatingStats, error) {
	out := make([]model.RatingStats, len(ids))
	position := make(map[int64]int, len(ids))
	for i, id := range ids {
		position[id] = i
	}
	for batch := range slices.Chunk(ids, MaxRatingBatch) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		got, err := s.d.Reviews.StoreRatingStats(ctx, batch)
		if err != nil {
			return nil, err
		}
		inBatch := make(map[int64]bool, len(batch))
		for _, id := range batch {
			inBatch[id] = true
		}
		for id, r := range got {
			if !inBatch[id] {
				return nil, fmt.Errorf("review 回傳了沒有詢問的店家 %d", id)
			}
			out[position[id]] = r
		}
	}
	return out, nil
}

type scoreJSON struct {
	StoreID               int64     `json:"store_id"`
	Score                 float64   `json:"score"`
	Demand                float64   `json:"demand"`
	Quality               float64   `json:"quality"`
	ReliabilityPenalty    float64   `json:"reliability_penalty"`
	Newness               float64   `json:"newness"`
	DistinctBuyers        int64     `json:"distinct_buyers"`
	QualifyingOrders      int64     `json:"qualifying_orders"`
	SellerCancelledOrders int64     `json:"seller_cancelled_orders"`
	RatingCount           int64     `json:"rating_count"`
	RatingAverage         *float64  `json:"rating_average"`
	PublishedAt           time.Time `json:"published_at"`
	FormulaVersion        int       `json:"formula_version"`
	ComputedAt            time.Time `json:"computed_at"`
}

func (s *Service) replace(ctx context.Context, scores []model.Score, started time.Time) (bool, error) {
	rows := make([]scoreJSON, len(scores))
	for i, sc := range scores {
		var avg *float64
		if sc.RatingCount > 0 {
			avg = &sc.RatingAverage
		}
		rows[i] = scoreJSON{
			StoreID: sc.StoreID, Score: sc.Score, Demand: sc.Demand, Quality: sc.Quality,
			ReliabilityPenalty: sc.ReliabilityPenalty, Newness: sc.Newness, DistinctBuyers: sc.DistinctBuyers,
			QualifyingOrders: sc.QualifyingOrders, SellerCancelledOrders: sc.SellerCancelledOrders, RatingCount: sc.RatingCount,
			RatingAverage: avg, PublishedAt: sc.PublishedAt.UTC(), FormulaVersion: sc.FormulaVersion, ComputedAt: sc.ComputedAt,
		}
	}
	payload, err := json.Marshal(rows)
	if err != nil {
		return false, err
	}
	written := false
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		state, err := q.LockRecomputeState(ctx, scoresJob)
		if err != nil {
			return err
		}
		if last := state.LastSucceededStartedAt; last != nil && !started.After(*last) {
			return nil
		}
		if err := q.DeleteSellerScores(ctx); err != nil {
			return err
		}
		if err := q.InsertSellerScores(ctx, payload); err != nil {
			return err
		}
		finished := s.finishedAt(started)
		if err := q.MarkRecomputeSucceeded(ctx, store.MarkRecomputeSucceededParams{Job: scoresJob, StartedAt: &started, FinishedAt: &finished}); err != nil {
			return err
		}
		written = true
		return nil
	})
	return written, err
}

func (s *Service) finishedAt(started time.Time) time.Time {
	return later(s.d.Now().UTC().Truncate(time.Microsecond), started)
}

func later(a, b time.Time) time.Time {
	if a.Before(b) {
		return b
	}
	return a
}
