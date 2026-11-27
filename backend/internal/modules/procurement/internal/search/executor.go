// Package search 是 CANDIDATE_SEARCH 類型 run 的 executor 與開始搜尋的 use case（A9-D29）：輸入只取自候選集所指的需求版本的品項解析，
// 搜尋、店家過濾與分類檢查都在 transaction 外，結果在單一 transaction 內寫成候選、需求狀態與 run 的終態轉換。
package search

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/candidate"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/resolver"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/run"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

const (
	statusClarifying       = "CLARIFYING"
	statusRequirementReady = "REQUIREMENT_READY"
	statusMatching         = "MATCHING"
)

var searchable = []string{statusRequirementReady, statusMatching}

// Searchable 回傳需求狀態是否可以開始或完成搜尋；詢價發布預覽與執行用同一個判斷（A9-D35）。
func Searchable(status string) bool { return slices.Contains(searchable, status) }

// LeafChecker 以分類 ID 查分類樹：節點存在而且是末層才回 true（*resolver.Resolver 滿足它）。
// 答案來自開始讀取不早於 notBefore 且在 TTL 之內的分類樹；拿不到這樣的樹回錯誤。破壞性的決定（清買家確認的分類）只能依它。
type LeafChecker interface {
	LeafAsOf(ctx context.Context, categoryID uuid.UUID, notBefore time.Time) (resolver.Match, bool, error)
}

// ErrRevisionChanged 表示提交時需求的現行版本已不是搜尋當時的版本：結果不寫，run 以 SEARCH_FAILED 收斂，需求狀態不動。
var ErrRevisionChanged = errors.New("search：需求的現行版本已不是搜尋當時的版本")

// Executor 實作 run.Executor。
type Executor struct {
	DB       *db.DB
	Ledger   run.Ledger
	Searcher candidate.Searcher
	Open     candidate.OpenStores
	Blocks   candidate.BlockedStores
	Resolver LeafChecker
	Now      func() time.Time
	Logger   *slog.Logger
}

var _ run.Executor = Executor{}

type loaded struct {
	run      store.ProcurementAgentRun
	set      store.ProcurementCandidateSet
	revision store.ProcurementRequestRevision
}

// Execute 執行一個 CANDIDATE_SEARCH run。回傳 nil 之前 run 已 SUCCEEDED，或因分類不再是末層而 FAILED（ITEM_CATEGORY_UNAVAILABLE）；
// 其餘錯誤由 Dispatcher 轉 FAILED（SEARCH_FAILED）；ctx 結束的錯誤原樣回傳。
func (e Executor) Execute(ctx context.Context, r store.ProcurementAgentRun) error {
	ld, err := e.load(ctx, r)
	if err != nil {
		return err
	}
	q, err := queryOf(ld.revision)
	if err != nil {
		return err
	}
	f, err := e.find(ctx, r.BuyerID, q, ld.revision.CreatedAt)
	if err != nil {
		return err
	}
	if f.categoryGone {
		return e.failCategoryUnavailable(ctx, ld)
	}
	return e.commit(ctx, ld, f.hits)
}

type found struct {
	hits         []candidate.Hit
	categoryGone bool
}

func (e Executor) find(ctx context.Context, buyerID int64, q candidate.Query, notBefore time.Time) (found, error) {
	if q.CategoryID != uuid.Nil() {
		_, leaf, err := e.Resolver.LeafAsOf(ctx, q.CategoryID, notBefore)
		if err != nil {
			return found{}, ctxOr(ctx, fmt.Errorf("search：查分類 %s：%w", q.CategoryID, err))
		}
		if !leaf {
			return found{categoryGone: true}, nil
		}
	}
	hits, err := candidate.Search(ctx, e.Searcher, e.Open, e.Blocks, buyerID, q)
	if err != nil {
		return found{}, ctxOr(ctx, err)
	}
	if err := ctx.Err(); err != nil {
		return found{}, err
	}
	return found{hits: hits}, nil
}

func ctxOr(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func queryOf(rev store.ProcurementRequestRevision) (candidate.Query, error) {
	if rev.ItemResolutionKind == nil || rev.ItemResolutionLabel == nil {
		return candidate.Query{}, fmt.Errorf("search：需求版本 %s 沒有品項解析", rev.ID)
	}
	switch slots.ItemResolutionKind(*rev.ItemResolutionKind) {
	case slots.CategoryConfirmed:
		if rev.ItemResolutionCategoryID == nil || *rev.ItemResolutionCategoryID == uuid.Nil() {
			return candidate.Query{}, fmt.Errorf("search：需求版本 %s 確認了分類卻沒有分類 ID", rev.ID)
		}
		return candidate.Query{CategoryID: *rev.ItemResolutionCategoryID}, nil
	case slots.BuyerDescribed:
		return candidate.Query{Keyword: *rev.ItemResolutionLabel}, nil
	}
	return candidate.Query{}, fmt.Errorf("search：需求版本 %s 的品項解析種類 %q 不認得", rev.ID, *rev.ItemResolutionKind)
}

func (e Executor) load(ctx context.Context, r store.ProcurementAgentRun) (loaded, error) {
	if r.CandidateSetID == nil {
		return loaded{}, fmt.Errorf("search：run %s 沒有 candidate_set_id", r.ID)
	}
	q := store.New(e.DB.Conn(ctx))
	buyer := r.BuyerID
	set, err := q.GetCandidateSetForBuyer(ctx, store.GetCandidateSetForBuyerParams{ID: *r.CandidateSetID, RequestID: r.RequestID, BuyerID: buyer})
	if err != nil {
		return loaded{}, fmt.Errorf("search：讀取候選集：%w", err)
	}
	rev, err := q.GetRevision(ctx, store.GetRevisionParams{ID: set.RequestRevisionID, RequestID: r.RequestID, BuyerID: &buyer})
	if err != nil {
		return loaded{}, fmt.Errorf("search：讀取需求版本：%w", err)
	}
	return loaded{run: r, set: set, revision: rev}, nil
}

func (e Executor) lockForCommit(ctx context.Context, q *store.Queries, ld loaded, to string) (store.ProcurementRequestRevision, error) {
	cur, err := q.LockRun(ctx, ld.run.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ProcurementRequestRevision{}, fmt.Errorf("search：run %s 已不存在", ld.run.ID)
	}
	if err != nil {
		return store.ProcurementRequestRevision{}, fmt.Errorf("search：鎖 run：%w", err)
	}
	switch {
	case run.IsTerminal(cur.Status):
		return store.ProcurementRequestRevision{}, run.ErrRunTerminal
	case cur.Status != run.StatusRunning:
		return store.ProcurementRequestRevision{}, &run.InvalidTransitionError{From: cur.Status, To: to}
	}
	buyer := ld.run.BuyerID
	active, err := q.LockActiveRevision(ctx, store.LockActiveRevisionParams{RequestID: ld.run.RequestID, BuyerID: &buyer})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return store.ProcurementRequestRevision{}, fmt.Errorf("search：鎖現行版本：%w", err)
	}
	if err != nil || active.ID != ld.set.RequestRevisionID {
		return store.ProcurementRequestRevision{}, ErrRevisionChanged
	}
	return active, nil
}

func (e Executor) commit(ctx context.Context, ld loaded, hits []candidate.Hit) error {
	now := e.Now()
	return e.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(e.DB.Conn(ctx))
		if _, err := e.lockForCommit(ctx, q, ld, run.StatusSucceeded); err != nil {
			return err
		}
		for rank, h := range hits {
			if err := q.InsertCandidate(ctx, store.InsertCandidateParams{
				ID: uuid.NewV7(), CandidateSetID: ld.set.ID, StoreID: h.StoreID, SourceID: fmt.Sprint(h.ProductID), Rank: int32(rank), Now: now,
			}); err != nil {
				return fmt.Errorf("search：寫入候選：%w", err)
			}
		}
		buyer := ld.run.BuyerID
		n, err := q.SetRequestStatusFromAny(ctx, store.SetRequestStatusFromAnyParams{ToStatus: statusMatching, Now: now, ID: ld.run.RequestID, BuyerID: &buyer, FromStatuses: searchable})
		if err != nil {
			return fmt.Errorf("search：更新需求狀態：%w", err)
		}
		if n == 0 {
			return fmt.Errorf("search：需求 %s 的狀態不是 %v", ld.run.RequestID, searchable)
		}
		_, err = e.Ledger.Transition(ctx, ld.run.ID, run.StatusSucceeded, "")
		return err
	})
}

func (e Executor) failCategoryUnavailable(ctx context.Context, ld loaded) error {
	now := e.Now()
	return e.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(e.DB.Conn(ctx))
		active, err := e.lockForCommit(ctx, q, ld, run.StatusFailed)
		if err != nil {
			return err
		}
		buyer := ld.run.BuyerID
		if _, err := q.SupersedeActiveRevision(ctx, store.SupersedeActiveRevisionParams{RequestID: ld.run.RequestID, BuyerID: &buyer}); err != nil {
			return fmt.Errorf("search：取代現行版本：%w", err)
		}
		no, err := q.NextRevisionNo(ctx, store.NextRevisionNoParams{RequestID: ld.run.RequestID, BuyerID: &buyer})
		if err != nil {
			return fmt.Errorf("search：版本序號：%w", err)
		}
		n, err := q.InsertRevision(ctx, withoutResolution(active, store.InsertRevisionParams{
			ID: uuid.NewV7(), RevisionNo: no, ParentRevisionID: &active.ID, Now: now, RequestID: ld.run.RequestID, BuyerID: &buyer,
		}))
		if err != nil {
			return fmt.Errorf("search：寫入版本：%w", err)
		}
		if n == 0 {
			return fmt.Errorf("search：需求 %s 不屬於這位買家", ld.run.RequestID)
		}
		n, err = q.SetRequestStatusFromAny(ctx, store.SetRequestStatusFromAnyParams{ToStatus: statusClarifying, Now: now, ID: ld.run.RequestID, BuyerID: &buyer, FromStatuses: searchable})
		if err != nil {
			return fmt.Errorf("search：更新需求狀態：%w", err)
		}
		if n == 0 {
			return fmt.Errorf("search：需求 %s 的狀態不是 %v", ld.run.RequestID, searchable)
		}
		_, err = e.Ledger.Transition(ctx, ld.run.ID, run.StatusFailed, run.FailureItemCategoryUnavailable)
		return err
	})
}

func withoutResolution(v store.ProcurementRequestRevision, p store.InsertRevisionParams) store.InsertRevisionParams {
	p.Item, p.Quantity, p.Unit, p.Packaging, p.BudgetPerUnit = v.Item, v.Quantity, v.Unit, v.Packaging, v.BudgetPerUnit
	p.DeliveryCity, p.DeliveryDate, p.ExactMatchRequired = v.DeliveryCity, v.DeliveryDate, v.ExactMatchRequired
	return p
}
