// Package ranking 是推薦賣家排行（ADR 2026-10-03-recommended-sellers-scoring）：排程重算每家合格店家的分數並整批
// 取代快照，讀取時依分數取出、再即時檢查曝光資格。來源是 seller-service 的 GetRecommendedSellers（遷移計畫 A3b 步驟 7d）。
//
// 為什麼分成兩段：分數需要時間窗統計（Java 的 order）與評價（review-service），任一個不通都不該讓每個讀取請求失敗，
// 所以分數由排程算好存起來（最多落後一個週期）；曝光資格（停權、KYC）要立即生效，所以讀取時才問 seller。
package ranking

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/discovery/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

const (
	// MaxLimit 是 Recommended 一次最多回傳的店家數。
	MaxLimit = 50
	// pageSize 是讀取時一頁讀的快照列數，也是一次問 seller 資格的店家數（seller.MaxBatchSize）。
	pageSize = seller.MaxBatchSize
	// scoresJob 是快照的重算工作在 recompute_state 的鍵。
	scoresJob = "seller_scores"
)

// Deps 是外部依賴；clock 由組裝點注入。
type Deps struct {
	DB       *db.DB
	Orders   Orders
	Products Products
	Reviews  Reviews
	Stores   Stores
	Now      func() time.Time
	Logger   *slog.Logger
}

// Service 是排行的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{d: d}
}

// Ranked 是排行的一列：店家 ID，與排名用到的訂單事實（和排名一致，不是另一次查詢的結果）。排名用的評價只留在快照內，
// 不對外輸出：顯示用的評價由 internal/httpapi 批次讀取，跟著店家卡片一起組裝。
type Ranked struct {
	StoreID      int64
	RecentOrders int64 // 近 30 天的成立訂單數（不暴露營收）
}

// Recommendation 是一次讀取的結果。
type Recommendation struct {
	Sellers []Ranked
	// ComputedAt 是快照那次重算的開始時間；從來沒有成功重算過是 nil。nil 與「沒有合格店家」（空的 Sellers、
	// ComputedAt 有值）是兩回事：呼叫端靠它分辨「還沒算過」與「算過了，沒有店」，也能判斷快照有多舊。
	ComputedAt *time.Time
}

// Recommended 依分數由高到低回傳最多 limit 家目前可以公開曝光的店家（limit 必須是 1 到 MaxLimit）。
// 分數取自最近一次成功的重算；每一頁都即時問 seller 資格，停權或失去資格的店家立刻略過，並往後補到 limit 或快照的盡頭，
// 所以被過濾掉的名額由排名更後面的店補上，不是由另一條路徑補位。
//
// 整個讀取（狀態、每一頁、seller 的資格查詢）在同一個唯讀的 REPEATABLE READ transaction 內：重算在讀取中途提交，
// 讀取仍然只看見同一份快照，不會因為換了快照而重複或漏掉店家，ComputedAt 也一定屬於讀到的那份快照。seller 的查詢走
// 同一個 transaction（同一條連線），所以不會在連線池滿時互相等待。呼叫端的 ctx 不可已經在 transaction 內（InReadSnapshot 會拒絕）。
func (s *Service) Recommended(ctx context.Context, limit int) (Recommendation, error) {
	if limit < 1 || limit > MaxLimit {
		return Recommendation{}, apperr.InvalidInput("limit 必須是 1 到 %d", MaxLimit)
	}
	var rec Recommendation
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		state, err := q.GetRecomputeState(ctx, scoresJob)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // 還沒有任何一次嘗試：沒有快照。
		}
		if err != nil {
			return err
		}
		if state.LastSucceededStartedAt == nil {
			return nil // 嘗試過但從沒成功：沒有快照。
		}
		computedAt := state.LastSucceededStartedAt.UTC()
		rec.ComputedAt = &computedAt
		sellers, err := s.read(ctx, q, limit)
		rec.Sellers = sellers
		return err
	})
	if err != nil {
		return Recommendation{}, err
	}
	if rec.Sellers == nil {
		rec.Sellers = []Ranked{}
	}
	return rec, nil
}

// read 在快照裡逐頁取出，每頁問 seller 資格，直到湊滿 limit 或快照的盡頭。
func (s *Service) read(ctx context.Context, q *store.Queries, limit int) ([]Ranked, error) {
	out := make([]Ranked, 0, limit)
	var cursor *store.NextSellerScoresParams
	for len(out) < limit {
		page, err := s.page(ctx, q, cursor)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		ids := make([]int64, len(page))
		for i, row := range page {
			ids[i] = row.StoreID
		}
		exposable, err := s.d.Stores.ExposableStores(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("discovery：確認店家的曝光資格：%w", err)
		}
		ok := make(map[int64]bool, len(exposable))
		for _, e := range exposable {
			ok[e.StoreID] = true
		}
		for _, row := range page {
			if ok[row.StoreID] && len(out) < limit {
				out = append(out, Ranked{StoreID: row.StoreID, RecentOrders: row.QualifyingOrders})
			}
		}
		last := page[len(page)-1]
		if len(page) < pageSize {
			break
		}
		cursor = &store.NextSellerScoresParams{AfterScore: last.Score, AfterPublishedAt: last.PublishedAt, AfterStoreID: last.StoreID, PageSize: pageSize}
	}
	return out, nil
}

type scoreRow struct {
	StoreID          int64
	QualifyingOrders int64
	RatingCount      int64
	RatingAverage    *float64
	Score            float64
	PublishedAt      time.Time
}

func (s *Service) page(ctx context.Context, q *store.Queries, cursor *store.NextSellerScoresParams) ([]scoreRow, error) {
	if cursor == nil {
		rows, err := q.FirstSellerScores(ctx, pageSize)
		if err != nil {
			return nil, err
		}
		out := make([]scoreRow, len(rows))
		for i, r := range rows {
			out[i] = scoreRow(r)
		}
		return out, nil
	}
	rows, err := q.NextSellerScores(ctx, *cursor)
	if err != nil {
		return nil, err
	}
	out := make([]scoreRow, len(rows))
	for i, r := range rows {
		out[i] = scoreRow(r)
	}
	return out, nil
}
