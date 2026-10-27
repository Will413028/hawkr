package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Will413028/hawkr/crm-service/internal/domain"
)

// fakeTxManager runs fn inline without a real transaction (unit-test double for
// database.TxManager). The dedup guarantee under test lives in the claim/skip
// logic, not in the pgx transaction, so inline execution is sufficient here.
type fakeTxManager struct{}

func (fakeTxManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// ledgerRepo simulates crm_customer_ledger's event_id PK + ON CONFLICT DO
// NOTHING: the first AppendLedgerEntry for an event_id returns inserted=true,
// subsequent calls with the same event_id return inserted=false. It also
// tallies how many times UpsertCustomerOnOrder actually ran.
type ledgerRepo struct {
	mockCrmRepo
	mu             sync.Mutex
	seen           map[string]bool
	upserts        int
	ledgerByOrder  map[string]int64
	fullRefundSeen map[string]bool
}

func newLedgerRepo() *ledgerRepo {
	r := &ledgerRepo{
		seen:           map[string]bool{},
		ledgerByOrder:  map[string]int64{},
		fullRefundSeen: map[string]bool{},
	}
	r.AppendLedgerEntryFn = func(_ context.Context, entry domain.LedgerEntry) (bool, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.seen[entry.EventID] {
			return false, nil
		}
		r.seen[entry.EventID] = true
		r.ledgerByOrder[entry.OrderNo] += entry.AmountDelta
		if entry.EventType == "refund_completed_full" {
			r.fullRefundSeen[entry.OrderNo] = true
		}
		return true, nil
	}
	r.UpsertCustomerOnOrderFn = func(_ context.Context, id, storeID, buyerID, amount int64, nickname, phone string) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.upserts++
		return nil
	}
	r.SumLedgerAmountForOrderFn = func(_ context.Context, orderNo string) (int64, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.ledgerByOrder[orderNo], nil
	}
	r.HasFullRefundEntryFn = func(_ context.Context, orderNo string) (bool, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.fullRefundSeen[orderNo], nil
	}
	return r
}

func TestRecordOrder_EnrichesViaBuyerIDAxis(t *testing.T) {
	const (
		storeID = int64(2683638500)
		buyerID = int64(7484500295840492062)
		realAcc = int64(2627714723)
	)

	// 兩邊的 phone 刻意給不同值，否則分不出寫進 DB 的 phone 是哪一邊來的。
	buyer := &mockBuyerClient{
		GetProfileByBuyerIDFn: func(_ context.Context, id int64) (*BuyerProfile, error) {
			if id != buyerID {
				return nil, nil
			}
			return &BuyerProfile{AccountID: realAcc, Nickname: "分身買家", Phone: "0900000000"}, nil
		},
	}
	account := &mockAccountClient{
		GetAccountInfoFn: func(_ context.Context, id int64) (*CustomerAccountInfo, error) {
			if id != realAcc {
				// account-service does not error on an unknown id shaped like a
				// buyer_id — it simply has no such account. Mirror that: silent
				// empty, which is exactly what made the bug invisible.
				return &CustomerAccountInfo{}, nil
			}
			return &CustomerAccountInfo{Phone: "0912345678"}, nil
		},
	}

	var gotNickname, gotPhone string
	repo := newLedgerRepo()
	repo.UpsertCustomerOnOrderFn = func(_ context.Context, _, _, _, _ int64, nickname, phone string) error {
		gotNickname, gotPhone = nickname, phone
		return nil
	}

	uc := NewCrmUseCase(repo, account, buyer, nil, nil, func() int64 { return 1 }, fakeTxManager{})

	if _, err := uc.RecordOrder(context.Background(), "evt-1", "O-1", storeID, buyerID, 276); err != nil {
		t.Fatalf("RecordOrder: %v", err)
	}

	if gotNickname != "分身買家" {
		t.Errorf("nickname = %q, want %q (buyer_id axis lookup)", gotNickname, "分身買家")
	}
	if gotPhone != "0912345678" {
		t.Errorf("phone = %q, want %q (account-service's value via the resolved account_id, not the buyer profile's %q)", gotPhone, "0912345678", "0900000000")
	}
	if buyer.lastProfileBuyerID != buyerID {
		t.Errorf("buyer-service profile queried with %d, want buyer_id %d", buyer.lastProfileBuyerID, buyerID)
	}
	if account.lastQueriedID != realAcc {
		t.Errorf("account-service queried with %d, want resolved account_id %d", account.lastQueriedID, realAcc)
	}
}

// TestRecordOrder_AccountServiceFailure_FallsBackToProfilePhone 釘住：
// account-service 是登入電話的權威來源，但它掛掉時不該寫入空字串——手上
// 已經有 buyer profile 的 phone，退回用它。寫空字串的後果是該顧客在賣家後台
// 的 keyword 搜尋中查不到（搜尋條件直接吃 crm_customers.phone）。
func TestRecordOrder_AccountServiceFailure_FallsBackToProfilePhone(t *testing.T) {
	const (
		buyerID = int64(7484500295840492062)
		realAcc = int64(2627714723)
	)

	buyer := &mockBuyerClient{
		GetProfileByBuyerIDFn: func(_ context.Context, _ int64) (*BuyerProfile, error) {
			return &BuyerProfile{AccountID: realAcc, Nickname: "分身買家", Phone: "0900000000"}, nil
		},
	}

	for _, tc := range []struct {
		name string
		fn   func(context.Context, int64) (*CustomerAccountInfo, error)
	}{
		{"account-service errors", func(context.Context, int64) (*CustomerAccountInfo, error) {
			return nil, errors.New("account-service unavailable")
		}},
		{"account-service returns no phone", func(context.Context, int64) (*CustomerAccountInfo, error) {
			return &CustomerAccountInfo{}, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &mockAccountClient{GetAccountInfoFn: tc.fn}

			var gotPhone, gotNickname string
			repo := newLedgerRepo()
			repo.UpsertCustomerOnOrderFn = func(_ context.Context, _, _, _, _ int64, nickname, phone string) error {
				gotNickname, gotPhone = nickname, phone
				return nil
			}

			uc := NewCrmUseCase(repo, account, buyer, nil, nil, func() int64 { return 1 }, fakeTxManager{})

			if _, err := uc.RecordOrder(context.Background(), "evt-1", "O-1", 10, buyerID, 276); err != nil {
				t.Fatalf("RecordOrder: %v", err)
			}
			if gotPhone != "0900000000" {
				t.Errorf("phone = %q, want %q (buyer profile fallback)", gotPhone, "0900000000")
			}
			if gotNickname != "分身買家" {
				t.Errorf("nickname = %q, want %q — an account-service failure must not lose the nickname too", gotNickname, "分身買家")
			}
		})
	}
}

func TestRecordOrder_ProfileMissing_LeavesEnrichmentEmpty(t *testing.T) {
	const buyerID = int64(7484500295840492062)

	buyer := &mockBuyerClient{
		GetProfileByBuyerIDFn: func(_ context.Context, _ int64) (*BuyerProfile, error) {
			return nil, nil // not found — buyer-service returns Code=0 with no Data
		},
	}
	account := &mockAccountClient{}

	var gotNickname, gotPhone string
	repo := newLedgerRepo()
	repo.UpsertCustomerOnOrderFn = func(_ context.Context, _, _, _, _ int64, nickname, phone string) error {
		gotNickname, gotPhone = nickname, phone
		return nil
	}

	uc := NewCrmUseCase(repo, account, buyer, nil, nil, func() int64 { return 1 }, fakeTxManager{})

	if _, err := uc.RecordOrder(context.Background(), "evt-1", "O-1", 10, buyerID, 276); err != nil {
		t.Fatalf("RecordOrder: %v", err)
	}

	if gotNickname != "" {
		t.Errorf("nickname = %q, want empty when profile is missing", gotNickname)
	}
	if gotPhone != "" {
		t.Errorf("phone = %q, want empty when profile is missing", gotPhone)
	}
	if account.calls != 0 {
		t.Errorf("account-service called %d times with an unresolved id, want 0", account.calls)
	}
}

// TestRecordOrder_BuyerServiceError_LeavesEnrichmentEmpty 釘住外呼失敗時不得
// 塞入錯值，且訂單本身仍要記帳（enrichment 是 best-effort）。
func TestRecordOrder_BuyerServiceError_LeavesEnrichmentEmpty(t *testing.T) {
	buyer := &mockBuyerClient{
		GetProfileByBuyerIDFn: func(_ context.Context, _ int64) (*BuyerProfile, error) {
			return nil, errors.New("buyer-service unavailable")
		},
	}
	account := &mockAccountClient{}

	var gotNickname, gotPhone string
	repo := newLedgerRepo()
	repo.UpsertCustomerOnOrderFn = func(_ context.Context, _, _, _, _ int64, nickname, phone string) error {
		gotNickname, gotPhone = nickname, phone
		return nil
	}

	uc := NewCrmUseCase(repo, account, buyer, nil, nil, func() int64 { return 1 }, fakeTxManager{})

	recorded, err := uc.RecordOrder(context.Background(), "evt-1", "O-1", 10, 20, 276)
	if err != nil {
		t.Fatalf("RecordOrder: %v", err)
	}
	if !recorded {
		t.Error("recorded = false, want true — enrichment failure must not block the ledger write")
	}
	if gotNickname != "" || gotPhone != "" {
		t.Errorf("nickname/phone = %q/%q, want both empty on buyer-service error", gotNickname, gotPhone)
	}
	if account.calls != 0 {
		t.Errorf("account-service called %d times without a resolved account_id, want 0", account.calls)
	}
}

func TestRecordOrder_Idempotent_SameEventTwice(t *testing.T) {
	repo := newLedgerRepo()
	uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, fakeTxManager{})

	recorded1, err := uc.RecordOrder(context.Background(), "evt-1", "O1", 10, 20, 1000)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if !recorded1 {
		t.Fatalf("first call: expected recorded=true")
	}

	recorded2, err := uc.RecordOrder(context.Background(), "evt-1", "O1", 10, 20, 1000)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if recorded2 {
		t.Fatalf("second call (same event_id): expected recorded=false")
	}

	if repo.upserts != 1 {
		t.Fatalf("expected exactly 1 UpsertCustomerOnOrder call, got %d", repo.upserts)
	}
}

func TestRecordOrder_DifferentEventSameOrderNo_BothRecord(t *testing.T) {
	// Defensive-in-depth check: dedup is keyed on event_id, not order_no — if
	// order-service ever redelivers order.completed for the same order under a
	// new event_id, this design would double-count. That's an accepted,
	// documented risk (see spec: order-service is expected to send exactly one
	// event_id per order.completed), not something this test asserts against;
	// it exists to make the actual dedup key explicit and prevent a future
	// accidental switch back to order_no-based dedup from going unnoticed.
	repo := newLedgerRepo()
	uc := NewCrmUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 }, fakeTxManager{})

	if _, err := uc.RecordOrder(context.Background(), "evt-1", "O1", 10, 20, 1000); err != nil {
		t.Fatalf("evt-1: %v", err)
	}
	if _, err := uc.RecordOrder(context.Background(), "evt-2", "O1", 10, 20, 1000); err != nil {
		t.Fatalf("evt-2: %v", err)
	}
	if repo.upserts != 2 {
		t.Fatalf("expected 2 upserts (dedup is per event_id, not per order_no), got %d", repo.upserts)
	}
}

func (m *ledgerRepo) RedactCustomerPIIByBuyerID(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}
