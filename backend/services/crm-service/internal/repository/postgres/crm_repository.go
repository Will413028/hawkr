package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/crm-service/internal/domain"
	"github.com/Will413028/hawkr/crm-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/go-pkg/database"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/pagination"
)

const (
	defaultPageSize int32 = 20
	// maxPageSize bounds ListCustomers: this is an admin/CRM console query,
	// so the cap is wider than a consumer-facing list, but still bounded —
	// without one, page_size is an unbounded full-table read.
	maxPageSize int32 = 100
)

type crmRepository struct {
	pool *pgxpool.Pool
}

func NewCrmRepository(pool *pgxpool.Pool) domain.CrmRepository {
	return &crmRepository{pool: pool}
}

func (r *crmRepository) GetCustomer(ctx context.Context, storeID, buyerID int64) (*domain.CrmCustomer, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetCustomer(ctx, gen.GetCustomerParams{
		StoreID: storeID,
		BuyerID: buyerID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("customer not found")
		}
		return nil, err
	}
	return toDomainCustomer(row), nil
}

func (r *crmRepository) ListCustomers(ctx context.Context, params domain.CustomerListParams) ([]*domain.CrmCustomer, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))

	_, pageSize, offset := pagination.NormalizePage(params.Page, params.PageSize, defaultPageSize, maxPageSize)

	rows, err := q.ListCustomers(ctx, gen.ListCustomersParams{
		StoreID: params.StoreID,
		Column2: params.CustomerType,
		Column3: params.Keyword,
		Column4: params.TagID,
		Column5: params.SortBy,
		Column6: params.SortOrder,
		Limit:   pageSize,
		Offset:  offset,
	})
	if err != nil {
		return nil, err
	}

	result := make([]*domain.CrmCustomer, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainCustomer(row))
	}
	return result, nil
}

func (r *crmRepository) CountCustomers(ctx context.Context, params domain.CustomerListParams) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.CountCustomers(ctx, gen.CountCustomersParams{
		StoreID: params.StoreID,
		Column2: params.CustomerType,
		Column3: params.Keyword,
		Column4: params.TagID,
	})
}

func (r *crmRepository) CreateCustomer(ctx context.Context, c *domain.CrmCustomer) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.CreateCustomer(ctx, gen.CreateCustomerParams{
		ID:           c.ID,
		StoreID:      c.StoreID,
		BuyerID:      c.BuyerID,
		CustomerType: string(c.CustomerType),
		TotalSpent:   c.TotalSpent,
		OrderCount:   int32(c.OrderCount),
		Nickname:     c.Nickname,
		Phone:        c.Phone,
	})
	if err != nil {
		return err
	}
	c.ID = row.ID
	return nil
}

func (r *crmRepository) UpdateCustomerType(ctx context.Context, storeID, buyerID int64, customerType domain.CustomerType) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpdateCustomerStatus(ctx, gen.UpdateCustomerStatusParams{
		StoreID:      storeID,
		BuyerID:      buyerID,
		CustomerType: string(customerType),
	})
}

func (r *crmRepository) GetTag(ctx context.Context, id int64) (*domain.CrmTag, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetTag(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("tag not found")
		}
		return nil, err
	}
	return toDomainTag(row), nil
}

func (r *crmRepository) GetTagByName(ctx context.Context, storeID int64, name string) (*domain.CrmTag, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetTagByName(ctx, gen.GetTagByNameParams{
		StoreID: storeID,
		Name:    name,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pkgerrors.NewNotFound("tag not found")
		}
		return nil, err
	}
	return toDomainTag(row), nil
}

func (r *crmRepository) ListTags(ctx context.Context, storeID int64) ([]*domain.CrmTag, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.ListTags(ctx, storeID)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.CrmTag, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainTag(row))
	}
	return result, nil
}

func (r *crmRepository) CreateTag(ctx context.Context, t *domain.CrmTag) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.CreateTag(ctx, gen.CreateTagParams{
		ID:      t.ID,
		StoreID: t.StoreID,
		Name:    t.Name,
	})
	if err != nil {
		return err
	}
	t.ID = row.ID
	return nil
}

func (r *crmRepository) UpdateTag(ctx context.Context, t *domain.CrmTag) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpdateTag(ctx, gen.UpdateTagParams{
		ID:      t.ID,
		Name:    t.Name,
		StoreID: t.StoreID,
	})
}

func (r *crmRepository) DeleteTag(ctx context.Context, storeID, tagID int64) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.DeleteTag(ctx, gen.DeleteTagParams{
		ID:      tagID,
		StoreID: storeID,
	})
}

func (r *crmRepository) SearchCustomersByTagIds(ctx context.Context, storeID int64, tagIDs []int64) ([]*domain.CrmCustomer, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.SearchCustomersByTagIds(ctx, gen.SearchCustomersByTagIdsParams{
		StoreID:  storeID,
		TagIds:   tagIDs,
		TagCount: int64(len(tagIDs)),
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.CrmCustomer, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainCustomer(row))
	}
	return result, nil
}

func (r *crmRepository) GetCustomerTags(ctx context.Context, storeID, buyerID int64) ([]*domain.CrmTag, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.GetCustomerTags(ctx, gen.GetCustomerTagsParams{
		StoreID: storeID,
		BuyerID: buyerID,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*domain.CrmTag, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainTag(row))
	}
	return result, nil
}

func (r *crmRepository) AddCustomerTag(ctx context.Context, storeID, buyerID, tagID int64) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.AddCustomerTag(ctx, gen.AddCustomerTagParams{
		StoreID: storeID,
		BuyerID: buyerID,
		TagID:   tagID,
	})
}

func (r *crmRepository) RemoveCustomerTag(ctx context.Context, storeID, buyerID, tagID int64) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.RemoveCustomerTag(ctx, gen.RemoveCustomerTagParams{
		StoreID: storeID,
		BuyerID: buyerID,
		TagID:   tagID,
	})
}

func (r *crmRepository) UpsertCustomerOnOrder(ctx context.Context, id, storeID, buyerID, amount int64, nickname, phone string) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpsertCustomerOnOrder(ctx, gen.UpsertCustomerOnOrderParams{
		ID:         id,
		StoreID:    storeID,
		BuyerID:    buyerID,
		TotalSpent: amount,
		Nickname:   nickname,
		Phone:      phone,
	})
}

func (r *crmRepository) BatchUpdateCustomerCache(ctx context.Context, storeID int64, updates []domain.CustomerCacheUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	q := gen.New(database.GetDBTX(ctx, r.pool))
	for _, u := range updates {
		if err := q.UpdateCustomerCache(ctx, gen.UpdateCustomerCacheParams{
			StoreID:  storeID,
			BuyerID:  u.BuyerID,
			Nickname: u.Nickname,
			Phone:    u.Phone,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *crmRepository) GetCustomerTypesByBuyer(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]string, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.GetCustomerTypesByBuyer(ctx, gen.GetCustomerTypesByBuyerParams{
		BuyerID:   buyerID,
		SellerIds: storeIDs,
	})
	if err != nil {
		return nil, err
	}
	result := make(map[int64]string, len(rows))
	for _, row := range rows {
		result[row.StoreID] = row.CustomerType
	}
	return result, nil
}

func (r *crmRepository) AppendLedgerEntry(ctx context.Context, entry domain.LedgerEntry) (bool, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.AppendLedgerEntry(ctx, gen.AppendLedgerEntryParams{
		EventID:     entry.EventID,
		StoreID:     entry.StoreID,
		BuyerID:     entry.BuyerID,
		EventType:   entry.EventType,
		OrderNo:     entry.OrderNo,
		AmountDelta: entry.AmountDelta,
		CountDelta:  entry.CountDelta,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func (r *crmRepository) ApplyRefundDelta(ctx context.Context, id, storeID, buyerID, amountDelta int64) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.ApplyRefundDelta(ctx, gen.ApplyRefundDeltaParams{
		ID:         id,
		StoreID:    storeID,
		BuyerID:    buyerID,
		TotalSpent: amountDelta,
	})
}

func (r *crmRepository) RedactCustomerPIIByBuyerID(ctx context.Context, buyerID int64) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.RedactCustomerPIIByBuyerID(ctx, buyerID)
}

func (r *crmRepository) SumLedgerAmountForOrder(ctx context.Context, orderNo string) (int64, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.SumLedgerAmountForOrder(ctx, orderNo)
}

func (r *crmRepository) HasFullRefundEntry(ctx context.Context, orderNo string) (bool, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.HasFullRefundEntry(ctx, orderNo)
}

func (r *crmRepository) LockOrderForNetting(ctx context.Context, orderNo string) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.LockOrderForNetting(ctx, orderNo)
}
