package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/categoryservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/productservice"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
)

// subscribeProcurement 讓買家關閉在同一個 transaction 刪除買家的採購資料、匿名保留賣家看得到的紀錄（A9-D19 S10；單體規則 4 的原子狀態變更）。
// serve 與 worker 都要訂：BuyerDeleted 在哪個程序的 transaction 內發布，訂閱者就要在那個程序裡。
func subscribeProcurement(bus *events.Bus, d *db.DB, log *slog.Logger) {
	events.Subscribe(bus, newProcurement(d, nil, nil, bus, log).OnBuyerDeleted)
}

// newProcurement 組出採購模組；jobs 是送出與取消 run 的 River client、entitlements 是 SubmitTurn 讀額度的買家模組（只訂閱事件的呼叫端兩者都給 nil）、
// bus 是 ExecuteRfqPublication 在同 transaction 發布 RfqPublished 的事件匯流排（必給，與訂閱者同一條）。
// 沒有 LLM 與分類樹時不登記 TURN 的 executor（serve 角色不執行 run）：領到 run 的 worker 把它標為該類型的 pipeline 失敗並退額度。
func newProcurement(d *db.DB, jobs procurement.JobQueue, entitlements procurement.EntitlementReader, bus *events.Bus, log *slog.Logger) *procurement.Module {
	return procurement.New(procurement.Deps{DB: d, Now: time.Now, Logger: log, Jobs: jobs, Entitlements: entitlements, Events: bus})
}

// newProcurementWorker 組出會執行 run 的採購模組（worker 角色）：登記 TURN 的 executor，模型是採購用的 llm 角色，分類樹經舊 product-service 轉接。
func newProcurementWorker(d *db.DB, jobs procurement.JobQueue, entitlements procurement.EntitlementReader, bus *events.Bus, model llm.Model, categories *categoryservice.Client, products *productservice.Client, sellers procurement.SellerStores, log *slog.Logger) *procurement.Module {
	return procurement.New(procurement.Deps{
		DB: d, Now: time.Now, Logger: log, Jobs: jobs, Entitlements: entitlements, Events: bus, LLM: model, Taxonomy: taxonomy{categories},
		Sellers: sellers, Search: productSearch{products},
	})
}

// productSearch 把舊 product-service 的商品搜尋轉成候選搜尋的 port；一頁不滿 searchPageSize 筆就是最後一頁。catalog 搬進單體時整段刪除。
type productSearch struct{ client productSearcher }

type productSearcher interface {
	SearchProducts(ctx context.Context, q productservice.SearchQuery) ([]productservice.SearchHit, error)
}

func (p productSearch) Page(ctx context.Context, q procurement.SearchQuery, page int) (procurement.SearchPage, error) {
	hits, err := p.client.SearchProducts(ctx, productservice.SearchQuery{CategoryID: q.CategoryID, Keyword: q.Keyword, Page: page})
	if err != nil {
		return procurement.SearchPage{}, err
	}
	out := procurement.SearchPage{Hits: make([]procurement.SearchHit, 0, len(hits)), Last: len(hits) < productservice.SearchPageSize}
	for _, h := range hits {
		out.Hits = append(out.Hits, procurement.SearchHit{ProductID: h.ID, StoreID: h.StoreID})
	}
	return out, nil
}

// taxonomy 把舊 category 服務的分類樹（巢狀）轉成 resolver 讀的扁平節點；catalog 搬進單體時整段刪除。
type taxonomy struct{ client *categoryservice.Client }

func (t taxonomy) CategoryTree(ctx context.Context) ([]procurement.TaxonomyNode, error) {
	tree, err := t.client.CategoryTree(ctx)
	if err != nil {
		return nil, err
	}
	return flattenTaxonomy(nil, tree), nil
}

func flattenTaxonomy(out []procurement.TaxonomyNode, nodes []categoryservice.Node) []procurement.TaxonomyNode {
	for _, n := range nodes {
		out = append(out, procurement.TaxonomyNode{ID: n.ID, ParentID: n.ParentID, Name: n.Name})
		out = flattenTaxonomy(out, n.Children)
	}
	return out
}

// addProcurementWorkers 登記採購 run 的執行 worker、reconciler 與事件清除。
func addProcurementWorkers(workers *river.Workers, m *procurement.Module) { m.AddWorkers(workers) }

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
