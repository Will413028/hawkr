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

func subscribeProcurement(bus *events.Bus, d *db.DB, log *slog.Logger) {
	events.Subscribe(bus, newProcurement(d, nil, nil, nil, bus, log).OnBuyerDeleted)
}

func newProcurement(d *db.DB, jobs procurement.JobQueue, entitlements procurement.EntitlementReader, sellers procurement.SellerModule, bus *events.Bus, log *slog.Logger) *procurement.Module {
	return procurement.New(procurement.Deps{DB: d, Now: time.Now, Logger: log, Jobs: jobs, Entitlements: entitlements, Sellers: sellers, Stores: sellers, Events: bus})
}

func newProcurementWorker(d *db.DB, jobs procurement.JobQueue, entitlements procurement.EntitlementReader, bus *events.Bus, model llm.Model, products *catalog.Module, sellers procurement.SellerStores, orders procurement.OrderCreator, log *slog.Logger) *procurement.Module {
	return procurement.New(procurement.Deps{
		DB: d, Now: time.Now, Logger: log, Jobs: jobs, Entitlements: entitlements, Events: bus, LLM: model, Taxonomy: taxonomy{products},
		Sellers: sellers, Search: productSearch{catalog: products, now: time.Now}, Orders: orders,
	})
}

const searchPageSize = 50

type publicProducts interface {
	ListPublicProducts(ctx context.Context, at time.Time, f catalog.PublicFilter, sort catalog.ProductSort, after *catalog.ListCursor, size int32) (catalog.ProductPage, error)
}

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

type categoryLister interface {
	Categories(ctx context.Context) ([]catalog.Category, error)
}

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

func addProcurementWorkers(workers *river.Workers, m *procurement.Module) {
	m.AddWorkers(workers)
	m.AddOrderWorkers(workers)
}

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
