package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
)

// subscribeProcurement 讓買家關閉在同一個 transaction 刪除買家的採購資料、匿名保留賣家看得到的紀錄（A9-D19 S10；單體規則 4 的原子狀態變更）。
// serve 與 worker 都要訂：BuyerDeleted 在哪個程序的 transaction 內發布，訂閱者就要在那個程序裡。
func subscribeProcurement(bus *events.Bus, d *db.DB, log *slog.Logger) {
	events.Subscribe(bus, newProcurement(d, nil, nil, nil, bus, log).OnBuyerDeleted)
}

// newProcurement 組出採購模組；jobs 是送出與取消 run 的 River client、entitlements 是 SubmitTurn 讀額度的買家模組（只訂閱事件的呼叫端兩者都給 nil）、
// sellers 是 seller 模組：發布詢價時檢查封鎖與營業中、賣家端方法（SellerProcurementAppService）檢查賣家能力都用它（只訂閱事件的呼叫端給 nil），
// bus 是 ExecuteRfqPublication 在同 transaction 發布 RfqPublished 的事件匯流排（必給，與訂閱者同一條）。
// 沒有 LLM 與分類樹時不登記 TURN 的 executor（serve 角色不執行 run）：領到 run 的 worker 把它標為該類型的 pipeline 失敗並退額度。
func newProcurement(d *db.DB, jobs procurement.JobQueue, entitlements procurement.EntitlementReader, sellers procurement.SellerModule, bus *events.Bus, log *slog.Logger) *procurement.Module {
	return procurement.New(procurement.Deps{DB: d, Now: time.Now, Logger: log, Jobs: jobs, Entitlements: entitlements, Sellers: sellers, Stores: sellers, Events: bus})
}

// newProcurementWorker 組出會執行 run 的採購模組（worker 角色）：登記 TURN 的 executor，模型是採購用的 llm 角色，分類樹與商品搜尋讀 catalog 的公開 API，
// 接受報價後的建單經舊 order-service 的 CreateRfqOrder（orders）。
func newProcurementWorker(d *db.DB, jobs procurement.JobQueue, entitlements procurement.EntitlementReader, bus *events.Bus, model llm.Model, products *catalog.Module, sellers procurement.SellerStores, orders procurement.OrderCreator, log *slog.Logger) *procurement.Module {
	return procurement.New(procurement.Deps{
		DB: d, Now: time.Now, Logger: log, Jobs: jobs, Entitlements: entitlements, Events: bus, LLM: model, Taxonomy: taxonomy{products},
		Sellers: sellers, Search: productSearch{catalog: products, now: time.Now}, Orders: orders,
	})
}

// searchPageSize 是候選搜尋每頁向 catalog 要的筆數。
const searchPageSize = 50

// publicProducts 是 catalog 的公開商品列表（*catalog.Module 滿足）。
type publicProducts interface {
	ListPublicProducts(ctx context.Context, at time.Time, f catalog.PublicFilter, sort catalog.ProductSort, after *catalog.ListCursor, size int32) (catalog.ProductPage, error)
}

// productSearch 以 catalog 的公開商品列表實作候選搜尋的 port：依銷量排序，keyset 翻頁；沒有下一頁游標就是最後一頁。
// 只含 ACTIVE 且店家此刻可曝光的商品（now 是模組的時鐘）。
type productSearch struct {
	catalog publicProducts
	now     func() time.Time
}

func (p productSearch) Page(ctx context.Context, q procurement.SearchQuery, after procurement.SearchCursor) (procurement.SearchPage, error) {
	keyword := strings.TrimSpace(q.Keyword)
	hasCategory := q.CategoryID != uuid.Nil()
	if hasCategory == (keyword != "") {
		return procurement.SearchPage{}, errors.New("procurement：商品搜尋要分類 ID 或關鍵字其中一個")
	}
	cursor, err := decodeSearchCursor(after)
	if err != nil {
		return procurement.SearchPage{}, err
	}
	f := catalog.PublicFilter{Query: keyword}
	if hasCategory {
		f.CategoryIDs = []uuid.UUID{q.CategoryID}
	}
	pg, err := p.catalog.ListPublicProducts(ctx, p.now(), f, catalog.SortSales, cursor, searchPageSize)
	if err != nil {
		return procurement.SearchPage{}, err
	}
	out := procurement.SearchPage{Hits: make([]procurement.SearchHit, 0, len(pg.Items)), Next: encodeSearchCursor(pg.Next)}
	for _, it := range pg.Items {
		out.Hits = append(out.Hits, procurement.SearchHit{ProductID: it.ID, StoreID: it.StoreID})
	}
	return out, nil
}

// 搜尋游標的內容是 catalog 的 (排序鍵, 商品 ID)，編成 "<key>:<id>"；對候選搜尋是不透明的字串。
func encodeSearchCursor(c *catalog.ListCursor) procurement.SearchCursor {
	if c == nil {
		return ""
	}
	return procurement.SearchCursor(strconv.FormatInt(c.Key, 10) + ":" + c.ID.String())
}

func decodeSearchCursor(c procurement.SearchCursor) (*catalog.ListCursor, error) {
	if c == "" {
		return nil, nil
	}
	key, id, ok := strings.Cut(string(c), ":")
	if !ok {
		return nil, fmt.Errorf("procurement：搜尋游標 %q 格式不對", c)
	}
	k, err := strconv.ParseInt(key, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("procurement：搜尋游標 %q 的排序鍵不是整數：%w", c, err)
	}
	u, err := uuid.Parse(id)
	if err != nil || u == uuid.Nil() {
		return nil, fmt.Errorf("procurement：搜尋游標 %q 的商品 ID 不合法", c)
	}
	return &catalog.ListCursor{Key: k, ID: u}, nil
}

// categoryLister 是 catalog 的分類樹（*catalog.Module 滿足）。
type categoryLister interface {
	Categories(ctx context.Context) ([]catalog.Category, error)
}

// taxonomy 把 catalog 的平台分類（扁平，兩層）轉成 resolver 讀的節點，名稱用繁體中文。
type taxonomy struct{ catalog categoryLister }

func (t taxonomy) CategoryTree(ctx context.Context) ([]procurement.TaxonomyNode, error) {
	cats, err := t.catalog.Categories(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]procurement.TaxonomyNode, len(cats))
	for i, c := range cats {
		out[i] = procurement.TaxonomyNode{ID: c.ID, ParentID: c.ParentID, Name: c.NameZhTW}
	}
	return out, nil
}

// addProcurementWorkers 登記採購 run 的執行 worker、reconciler 與事件清除，以及接受報價後的建單 worker 與它的 reconciler。
// 建單的單一生效者是單體（舊 AI 服務 已刪）。
func addProcurementWorkers(workers *river.Workers, m *procurement.Module) {
	m.AddWorkers(workers)
	m.AddOrderWorkers(workers)
}

// startProcurementHub 啟動 WatchRun 共用的 LISTEN 連線，回傳的函式停止它並等它結束。只有 serve 角色需要（串流在那裡）。
func startProcurementHub(ctx context.Context, m *procurement.Module, log *slog.Logger) (stop func()) {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := m.RunHub(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Error("procurement：run 的 LISTEN 連線結束", "error", err)
		}
	}()
	return func() {
		cancel()
		<-done
	}
}
