// Package read 是 crm 的讀取路徑：客戶列表與詳情、標籤、VIP 狀態。讀取方法不寫任何狀態。
//
// 暱稱與頭像不在 crm：呼叫端（httpapi）以 buyer.BatchDisplay 補；Customer 沒有電話、Email、地址欄位（A6-D8）。
package read

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/crm/internal/text"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// Stores 是讀取的賣家能力來源（seller 公開 API）。
type Stores interface {
	Store(ctx context.Context, storeID int64) (seller.Store, error)
}

// Deps 是讀取路徑的依賴。
type Deps struct {
	DB     *db.DB
	Stores Stores
}

// Service 是讀取路徑的 use case。
type Service struct{ d Deps }

// New 建立讀取路徑。
func New(d Deps) *Service { return &Service{d: d} }

func (s *Service) q(ctx context.Context) *store.Queries { return store.New(s.d.DB.Conn(ctx)) }

func (s *Service) authorize(ctx context.Context, storeID int64, notFound func(context.Context) error) error {
	st, err := s.d.Stores.Store(ctx, storeID)
	if err == nil {
		err = seller.Authorize(st, model.RequiredCapability)
	}
	if apperr.KindOf(err) == apperr.KindNotFound {
		return notFound(ctx)
	}
	return err
}

func customerNotFound(ctx context.Context) error {
	return model.WithReason(apperr.NotFound("%s", text.CrmCustomerNotFound(uitext.LocaleFrom(ctx))), model.ReasonCustomerNotFound)
}

func tagNotFound(ctx context.Context) error {
	return model.WithReason(apperr.NotFound("%s", text.CrmTagNotFound(uitext.LocaleFrom(ctx))), model.ReasonTagNotFound)
}

func pageSize(size int32) int32 {
	switch {
	case size <= 0:
		return model.DefaultPageSize
	case size > model.MaxPageSize:
		return model.MaxPageSize
	}
	return size
}

type row struct {
	StoreID      int64
	BuyerID      int64
	CustomerType string
	TotalSpent   int64
	OrderCount   int32
	FirstOrderAt time.Time
}

func toCustomer(r row) model.Customer {
	return model.Customer{
		StoreID: r.StoreID, BuyerID: r.BuyerID, Type: model.CustomerType(r.CustomerType),
		TotalSpent: r.TotalSpent, OrderCount: r.OrderCount, FirstOrderAt: r.FirstOrderAt, Tags: []model.Tag{},
	}
}

// ListCustomers 是店家的客戶，依 (排序欄位, buyer_id) 做 keyset；不回總數。同值的列以 buyer_id 定序，所以跨頁不重複、不遺漏。
func (s *Service) ListCustomers(ctx context.Context, in model.CustomerListInput) (model.CustomerPage, error) {
	if in.StoreID <= 0 {
		return model.CustomerPage{}, fmt.Errorf("crm：店家 %d 不是正整數", in.StoreID)
	}
	if !in.Sort.Valid() {
		return model.CustomerPage{}, fmt.Errorf("crm：未知的排序欄位 %d", in.Sort)
	}
	if err := s.authorize(ctx, in.StoreID, customerNotFound); err != nil {
		return model.CustomerPage{}, err
	}
	var typ *string
	if in.Type != nil {
		if !in.Type.Valid() {
			return model.CustomerPage{}, apperr.InvalidInputOn([]string{"CustomerType"}, "%s", text.CrmCustomerTypeInvalid(uitext.LocaleFrom(ctx)))
		}
		v := string(*in.Type)
		typ = &v
	}
	if in.After != nil && in.After.BuyerID <= 0 {
		return model.CustomerPage{}, fmt.Errorf("crm：游標的 buyer_id %d 不是正整數", in.After.BuyerID)
	}
	n := pageSize(in.Size)
	rows, err := s.list(ctx, in, typ, n+1)
	if err != nil {
		return model.CustomerPage{}, err
	}
	var next *model.CustomerCursor
	if len(rows) > int(n) {
		rows = rows[:n]
		last := rows[n-1]
		next = &model.CustomerCursor{BuyerID: last.BuyerID}
		switch in.Sort {
		case model.SortTotalSpent:
			next.Number = last.TotalSpent
		case model.SortOrderCount:
			next.Number = int64(last.OrderCount)
		case model.SortFirstOrderAt:
			next.At = last.FirstOrderAt
		}
	}
	page := model.CustomerPage{Customers: make([]model.Customer, len(rows)), Next: next}
	buyerIDs := make([]int64, len(rows))
	index := make(map[int64]int, len(rows))
	for i, r := range rows {
		page.Customers[i] = toCustomer(r)
		buyerIDs[i] = r.BuyerID
		index[r.BuyerID] = i
	}
	if len(rows) == 0 {
		return page, nil
	}
	tags, err := s.q(ctx).ListCustomerTags(ctx, store.ListCustomerTagsParams{StoreID: in.StoreID, BuyerIds: buyerIDs})
	if err != nil {
		return model.CustomerPage{}, err
	}
	for _, t := range tags {
		i := index[t.BuyerID]
		page.Customers[i].Tags = append(page.Customers[i].Tags, model.Tag{ID: t.ID, Name: t.Name})
	}
	return page, nil
}

func (s *Service) list(ctx context.Context, in model.CustomerListInput, typ *string, limit int32) ([]row, error) {
	q := s.q(ctx)
	var afterBuyer *int64
	var afterNum *int64
	var afterAt *time.Time
	if in.After != nil {
		b := in.After.BuyerID
		afterBuyer = &b
		afterNum = &in.After.Number
		afterAt = &in.After.At
	}
	switch in.Sort {
	case model.SortTotalSpent:
		if in.Desc {
			rs, err := q.ListCustomersBySpentDesc(ctx, store.ListCustomersBySpentDescParams{StoreID: in.StoreID, CustomerType: typ, TagID: in.TagID, AfterKey: afterNum, AfterBuyer: afterBuyer, MaxRows: limit})
			return convert(rs, err, func(r store.ListCustomersBySpentDescRow) row { return row(r) })
		}
		rs, err := q.ListCustomersBySpentAsc(ctx, store.ListCustomersBySpentAscParams{StoreID: in.StoreID, CustomerType: typ, TagID: in.TagID, AfterKey: afterNum, AfterBuyer: afterBuyer, MaxRows: limit})
		return convert(rs, err, func(r store.ListCustomersBySpentAscRow) row { return row(r) })
	case model.SortOrderCount:
		if in.Desc {
			rs, err := q.ListCustomersByOrdersDesc(ctx, store.ListCustomersByOrdersDescParams{StoreID: in.StoreID, CustomerType: typ, TagID: in.TagID, AfterKey: afterNum, AfterBuyer: afterBuyer, MaxRows: limit})
			return convert(rs, err, func(r store.ListCustomersByOrdersDescRow) row { return row(r) })
		}
		rs, err := q.ListCustomersByOrdersAsc(ctx, store.ListCustomersByOrdersAscParams{StoreID: in.StoreID, CustomerType: typ, TagID: in.TagID, AfterKey: afterNum, AfterBuyer: afterBuyer, MaxRows: limit})
		return convert(rs, err, func(r store.ListCustomersByOrdersAscRow) row { return row(r) })
	case model.SortFirstOrderAt:
		if in.Desc {
			rs, err := q.ListCustomersByFirstOrderDesc(ctx, store.ListCustomersByFirstOrderDescParams{StoreID: in.StoreID, CustomerType: typ, TagID: in.TagID, AfterKey: afterAt, AfterBuyer: afterBuyer, MaxRows: limit})
			return convert(rs, err, func(r store.ListCustomersByFirstOrderDescRow) row { return row(r) })
		}
		rs, err := q.ListCustomersByFirstOrderAsc(ctx, store.ListCustomersByFirstOrderAscParams{StoreID: in.StoreID, CustomerType: typ, TagID: in.TagID, AfterKey: afterAt, AfterBuyer: afterBuyer, MaxRows: limit})
		return convert(rs, err, func(r store.ListCustomersByFirstOrderAscRow) row { return row(r) })
	}
	return nil, fmt.Errorf("crm：未知的排序欄位 %d", in.Sort)
}

func convert[R any](rs []R, err error, f func(R) row) ([]row, error) {
	if err != nil {
		return nil, err
	}
	out := make([]row, len(rs))
	for i, r := range rs {
		out[i] = f(r)
	}
	return out, nil
}

// GetCustomer 是一位客戶與他的標籤；店家沒有這位客戶回 NotFound。
func (s *Service) GetCustomer(ctx context.Context, storeID, buyerID int64) (model.Customer, error) {
	if storeID <= 0 || buyerID <= 0 {
		return model.Customer{}, fmt.Errorf("crm：店家 %d 或買家 %d 不是正整數", storeID, buyerID)
	}
	if err := s.authorize(ctx, storeID, customerNotFound); err != nil {
		return model.Customer{}, err
	}
	q := s.q(ctx)
	r, err := q.GetCustomer(ctx, store.GetCustomerParams{StoreID: storeID, BuyerID: buyerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Customer{}, customerNotFound(ctx)
	}
	if err != nil {
		return model.Customer{}, err
	}
	c := toCustomer(row(r))
	tags, err := q.ListCustomerTags(ctx, store.ListCustomerTagsParams{StoreID: storeID, BuyerIds: []int64{buyerID}})
	if err != nil {
		return model.Customer{}, err
	}
	for _, t := range tags {
		c.Tags = append(c.Tags, model.Tag{ID: t.ID, Name: t.Name})
	}
	return c, nil
}

// CustomersAmong 回傳 buyerIDs 裡是店家客戶的集合：一次授權、一次 SQL。店家已關閉或不存在與 GetCustomer 一致回 NotFound。
func (s *Service) CustomersAmong(ctx context.Context, storeID int64, buyerIDs []int64) (map[int64]bool, error) {
	if storeID <= 0 {
		return nil, fmt.Errorf("crm：店家 %d 不是正整數", storeID)
	}
	if len(buyerIDs) > model.MaxBuyerIDs {
		return nil, fmt.Errorf("crm：買家 %d 位超過上限 %d", len(buyerIDs), model.MaxBuyerIDs)
	}
	for _, id := range buyerIDs {
		if id <= 0 {
			return nil, fmt.Errorf("crm：買家 %d 不是正整數", id)
		}
	}
	if err := s.authorize(ctx, storeID, customerNotFound); err != nil {
		return nil, err
	}
	out := make(map[int64]bool, len(buyerIDs))
	if len(buyerIDs) == 0 {
		return out, nil
	}
	ids, err := s.q(ctx).ListCustomerBuyerIDs(ctx, store.ListCustomerBuyerIDsParams{StoreID: storeID, BuyerIds: buyerIDs})
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

// ListTags 是店家全部的標籤（名稱升序）；數量受 MaxTagsPerStore 約束，所以不分頁。
func (s *Service) ListTags(ctx context.Context, storeID int64) ([]model.Tag, error) {
	if storeID <= 0 {
		return nil, fmt.Errorf("crm：店家 %d 不是正整數", storeID)
	}
	if err := s.authorize(ctx, storeID, tagNotFound); err != nil {
		return nil, err
	}
	rs, err := s.q(ctx).ListTags(ctx, storeID)
	if err != nil {
		return nil, err
	}
	out := make([]model.Tag, len(rs))
	for i, r := range rs {
		out[i] = model.Tag{ID: r.ID, Name: r.Name}
	}
	return out, nil
}

// VIPStatuses 回傳買家在各店是否是 VIP：每個請求的店家都有一筆（沒有客戶列的店是 false）。空清單回空 map；
// 超過 MaxStoreIDs 或含非正整數回 InvalidInput。會被別的模組的呼叫路徑使用，所以不開 InReadSnapshot。
func (s *Service) VIPStatuses(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]bool, error) {
	if len(storeIDs) == 0 {
		return map[int64]bool{}, nil
	}
	if buyerID <= 0 {
		return nil, fmt.Errorf("crm：買家 %d 不是正整數", buyerID)
	}
	invalid := apperr.InvalidInputOn([]string{"StoreIDs"}, "%s", text.CrmStoreIdsInvalid(uitext.LocaleFrom(ctx), text.CrmStoreIdsInvalidArgs{Max: model.MaxStoreIDs}))
	if len(storeIDs) > model.MaxStoreIDs {
		return nil, invalid
	}
	out := make(map[int64]bool, len(storeIDs))
	for _, id := range storeIDs {
		if id <= 0 {
			return nil, invalid
		}
		out[id] = false
	}
	vip, err := s.q(ctx).ListVIPStores(ctx, store.ListVIPStoresParams{BuyerID: buyerID, StoreIds: storeIDs})
	if err != nil {
		return nil, err
	}
	for _, id := range vip {
		out[id] = true
	}
	return out, nil
}
