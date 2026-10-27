package usecase

import (
	"context"

	"github.com/Will413028/hawkr/crm-service/internal/domain"
)

// ── mockCrmRepo ───────────────────────────────────────────────────────────────

type mockCrmRepo struct {
	GetCustomerFn              func(ctx context.Context, storeID, buyerID int64) (*domain.CrmCustomer, error)
	ListCustomersFn            func(ctx context.Context, params domain.CustomerListParams) ([]*domain.CrmCustomer, error)
	CountCustomersFn           func(ctx context.Context, params domain.CustomerListParams) (int64, error)
	CreateCustomerFn           func(ctx context.Context, customer *domain.CrmCustomer) error
	UpdateCustomerTypeFn       func(ctx context.Context, storeID, buyerID int64, customerType domain.CustomerType) error
	UpsertCustomerOnOrderFn    func(ctx context.Context, id, storeID, buyerID, amount int64, nickname, phone string) error
	GetTagFn                   func(ctx context.Context, id int64) (*domain.CrmTag, error)
	GetTagByNameFn             func(ctx context.Context, storeID int64, name string) (*domain.CrmTag, error)
	ListTagsFn                 func(ctx context.Context, storeID int64) ([]*domain.CrmTag, error)
	CreateTagFn                func(ctx context.Context, tag *domain.CrmTag) error
	UpdateTagFn                func(ctx context.Context, tag *domain.CrmTag) error
	DeleteTagFn                func(ctx context.Context, storeID, tagID int64) error
	GetCustomerTagsFn          func(ctx context.Context, storeID, buyerID int64) ([]*domain.CrmTag, error)
	AddCustomerTagFn           func(ctx context.Context, storeID, buyerID, tagID int64) error
	RemoveCustomerTagFn        func(ctx context.Context, storeID, buyerID, tagID int64) error
	AppendLedgerEntryFn        func(ctx context.Context, entry domain.LedgerEntry) (bool, error)
	ApplyRefundDeltaFn         func(ctx context.Context, id, storeID, buyerID, amountDelta int64) (int64, error)
	SumLedgerAmountForOrderFn  func(ctx context.Context, orderNo string) (int64, error)
	HasFullRefundEntryFn       func(ctx context.Context, orderNo string) (bool, error)
	LockOrderForNettingFn      func(ctx context.Context, orderNo string) error
	BatchUpdateCustomerCacheFn func(ctx context.Context, storeID int64, updates []domain.CustomerCacheUpdate) error
	GetCustomerTypesByBuyerFn  func(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]string, error)
	SearchCustomersByTagIdsFn  func(ctx context.Context, storeID int64, tagIDs []int64) ([]*domain.CrmCustomer, error)
}

func (m *mockCrmRepo) GetCustomer(ctx context.Context, storeID, buyerID int64) (*domain.CrmCustomer, error) {
	return m.GetCustomerFn(ctx, storeID, buyerID)
}
func (m *mockCrmRepo) ListCustomers(ctx context.Context, params domain.CustomerListParams) ([]*domain.CrmCustomer, error) {
	return m.ListCustomersFn(ctx, params)
}
func (m *mockCrmRepo) CountCustomers(ctx context.Context, params domain.CustomerListParams) (int64, error) {
	return m.CountCustomersFn(ctx, params)
}
func (m *mockCrmRepo) CreateCustomer(ctx context.Context, customer *domain.CrmCustomer) error {
	return m.CreateCustomerFn(ctx, customer)
}
func (m *mockCrmRepo) UpdateCustomerType(ctx context.Context, storeID, buyerID int64, customerType domain.CustomerType) error {
	return m.UpdateCustomerTypeFn(ctx, storeID, buyerID, customerType)
}
func (m *mockCrmRepo) GetTag(ctx context.Context, id int64) (*domain.CrmTag, error) {
	return m.GetTagFn(ctx, id)
}
func (m *mockCrmRepo) ListTags(ctx context.Context, storeID int64) ([]*domain.CrmTag, error) {
	return m.ListTagsFn(ctx, storeID)
}
func (m *mockCrmRepo) CreateTag(ctx context.Context, tag *domain.CrmTag) error {
	return m.CreateTagFn(ctx, tag)
}
func (m *mockCrmRepo) UpdateTag(ctx context.Context, tag *domain.CrmTag) error {
	return m.UpdateTagFn(ctx, tag)
}
func (m *mockCrmRepo) DeleteTag(ctx context.Context, storeID, tagID int64) error {
	return m.DeleteTagFn(ctx, storeID, tagID)
}
func (m *mockCrmRepo) GetTagByName(ctx context.Context, storeID int64, name string) (*domain.CrmTag, error) {
	return m.GetTagByNameFn(ctx, storeID, name)
}
func (m *mockCrmRepo) GetCustomerTags(ctx context.Context, storeID, buyerID int64) ([]*domain.CrmTag, error) {
	if m.GetCustomerTagsFn == nil {
		return nil, nil
	}
	return m.GetCustomerTagsFn(ctx, storeID, buyerID)
}
func (m *mockCrmRepo) AddCustomerTag(ctx context.Context, storeID, buyerID, tagID int64) error {
	return m.AddCustomerTagFn(ctx, storeID, buyerID, tagID)
}
func (m *mockCrmRepo) RemoveCustomerTag(ctx context.Context, storeID, buyerID, tagID int64) error {
	return m.RemoveCustomerTagFn(ctx, storeID, buyerID, tagID)
}
func (m *mockCrmRepo) UpsertCustomerOnOrder(ctx context.Context, id, storeID, buyerID, amount int64, nickname, phone string) error {
	if m.UpsertCustomerOnOrderFn == nil {
		return nil
	}
	return m.UpsertCustomerOnOrderFn(ctx, id, storeID, buyerID, amount, nickname, phone)
}
func (m *mockCrmRepo) BatchUpdateCustomerCache(ctx context.Context, storeID int64, updates []domain.CustomerCacheUpdate) error {
	if m.BatchUpdateCustomerCacheFn == nil {
		return nil
	}
	return m.BatchUpdateCustomerCacheFn(ctx, storeID, updates)
}
func (m *mockCrmRepo) AppendLedgerEntry(ctx context.Context, entry domain.LedgerEntry) (bool, error) {
	if m.AppendLedgerEntryFn == nil {
		return true, nil
	}
	return m.AppendLedgerEntryFn(ctx, entry)
}
func (m *mockCrmRepo) ApplyRefundDelta(ctx context.Context, id, storeID, buyerID, amountDelta int64) (int64, error) {
	if m.ApplyRefundDeltaFn == nil {
		return 1, nil
	}
	return m.ApplyRefundDeltaFn(ctx, id, storeID, buyerID, amountDelta)
}
func (m *mockCrmRepo) SumLedgerAmountForOrder(ctx context.Context, orderNo string) (int64, error) {
	if m.SumLedgerAmountForOrderFn == nil {
		return 0, nil
	}
	return m.SumLedgerAmountForOrderFn(ctx, orderNo)
}
func (m *mockCrmRepo) HasFullRefundEntry(ctx context.Context, orderNo string) (bool, error) {
	if m.HasFullRefundEntryFn == nil {
		return false, nil
	}
	return m.HasFullRefundEntryFn(ctx, orderNo)
}
func (m *mockCrmRepo) LockOrderForNetting(ctx context.Context, orderNo string) error {
	if m.LockOrderForNettingFn == nil {
		return nil
	}
	return m.LockOrderForNettingFn(ctx, orderNo)
}

func (m *mockCrmRepo) GetCustomerTypesByBuyer(ctx context.Context, buyerID int64, storeIDs []int64) (map[int64]string, error) {
	if m.GetCustomerTypesByBuyerFn == nil {
		return nil, nil
	}
	return m.GetCustomerTypesByBuyerFn(ctx, buyerID, storeIDs)
}
func (m *mockCrmRepo) SearchCustomersByTagIds(ctx context.Context, storeID int64, tagIDs []int64) ([]*domain.CrmCustomer, error) {
	if m.SearchCustomersByTagIdsFn == nil {
		return nil, nil
	}
	return m.SearchCustomersByTagIdsFn(ctx, storeID, tagIDs)
}

// ── mockBuyerClient ───────────────────────────────────────────────────────────

type mockBuyerClient struct {
	GetDefaultAddressFn           func(ctx context.Context, buyerID int64) (string, error)
	GetBuyerMetaBatchByBuyerIDsFn func(ctx context.Context, buyerIDs []int64) (map[int64]*BuyerMeta, error)
	GetProfileByBuyerIDFn         func(ctx context.Context, buyerID int64) (*BuyerProfile, error)

	// Call recorders — the axis bugs this package guards against are silent
	// (a wrong-axis id produces an empty result, never an error), so tests
	// must assert on the id that was actually sent, not just on the outcome.
	// addressCalls is tracked separately from lastAddressQueryID so a test can
	// tell "never called" apart from "called with 0".
	lastProfileBuyerID  int64
	lastAddressQueryID  int64
	addressCalls        int
	metaBatchQueriedIDs []int64
}

func (m *mockBuyerClient) GetDefaultAddress(ctx context.Context, buyerID int64) (string, error) {
	m.lastAddressQueryID = buyerID
	m.addressCalls++
	if m.GetDefaultAddressFn == nil {
		return "", nil
	}
	return m.GetDefaultAddressFn(ctx, buyerID)
}

func (m *mockBuyerClient) GetBuyerMetaBatchByBuyerIDs(ctx context.Context, buyerIDs []int64) (map[int64]*BuyerMeta, error) {
	m.metaBatchQueriedIDs = append(m.metaBatchQueriedIDs, buyerIDs...)
	if m.GetBuyerMetaBatchByBuyerIDsFn == nil {
		return map[int64]*BuyerMeta{}, nil
	}
	return m.GetBuyerMetaBatchByBuyerIDsFn(ctx, buyerIDs)
}

func (m *mockBuyerClient) GetProfileByBuyerID(ctx context.Context, buyerID int64) (*BuyerProfile, error) {
	m.lastProfileBuyerID = buyerID
	if m.GetProfileByBuyerIDFn == nil {
		return nil, nil
	}
	return m.GetProfileByBuyerIDFn(ctx, buyerID)
}

// ── mockAccountClient ─────────────────────────────────────────────────────────

type mockAccountClient struct {
	GetAccountInfoFn      func(ctx context.Context, accountID int64) (*CustomerAccountInfo, error)
	GetAccountInfoBatchFn func(ctx context.Context, accountIDs []int64) (map[int64]*CustomerAccountInfo, error)
	lastQueriedID         int64
	calls                 int
}

func (m *mockAccountClient) GetAccountInfo(ctx context.Context, accountID int64) (*CustomerAccountInfo, error) {
	m.lastQueriedID = accountID
	m.calls++
	if m.GetAccountInfoFn == nil {
		return &CustomerAccountInfo{}, nil
	}
	return m.GetAccountInfoFn(ctx, accountID)
}

func (m *mockAccountClient) GetAccountInfoBatch(ctx context.Context, accountIDs []int64) (map[int64]*CustomerAccountInfo, error) {
	if m.GetAccountInfoBatchFn == nil {
		return map[int64]*CustomerAccountInfo{}, nil
	}
	return m.GetAccountInfoBatchFn(ctx, accountIDs)
}

// ── mockOrderClient ───────────────────────────────────────────────────────────

type mockOrderClient struct {
	GetOrderByNoFn func(ctx context.Context, orderNo string) (*OrderInfo, error)
}

func (m *mockOrderClient) GetOrderByNo(ctx context.Context, orderNo string) (*OrderInfo, error) {
	if m.GetOrderByNoFn == nil {
		return &OrderInfo{OrderNo: orderNo, StoreID: 1, BuyerID: 1}, nil
	}
	return m.GetOrderByNoFn(ctx, orderNo)
}

func (m *mockCrmRepo) RedactCustomerPIIByBuyerID(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}
