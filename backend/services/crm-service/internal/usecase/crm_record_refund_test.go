package usecase

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/crm-service/internal/domain"
)

func TestRecordRefund_Partial_NetsExactAmount(t *testing.T) {
	repo := newLedgerRepo()
	var appliedDelta int64
	repo.ApplyRefundDeltaFn = func(_ context.Context, _, storeID, buyerID, amountDelta int64) (int64, error) {
		appliedDelta = amountDelta
		return 1, nil
	}
	orderClient := &mockOrderClient{}
	uc := NewCrmUseCase(repo, nil, nil, nil, orderClient, func() int64 { return 1 }, fakeTxManager{})

	recorded, err := uc.RecordRefund(context.Background(), "evt-refund-1", "O1", "PARTIAL_REFUND", 300)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !recorded {
		t.Fatalf("expected recorded=true")
	}
	if appliedDelta != -300 {
		t.Fatalf("expected amountDelta=-300, got %d", appliedDelta)
	}
}

func TestRecordRefund_Full_NoPriorHistory_NetsToZero(t *testing.T) {
	// Order never reached order.completed (the dominant full-refund path —
	// case opened at DELIVERED(40), resolved before natural completion).
	// No prior ledger rows for this order_no, so S=0 and the correct net
	// contribution is 0 — NOT -total_actual_amount (that was attempt 3/4's
	// bug: pre-subtracting a contribution that will never exist).
	repo := newLedgerRepo()
	var appliedDelta int64
	repo.ApplyRefundDeltaFn = func(_ context.Context, _, storeID, buyerID, amountDelta int64) (int64, error) {
		appliedDelta = amountDelta
		return 1, nil
	}
	orderClient := &mockOrderClient{
		GetOrderByNoFn: func(_ context.Context, orderNo string) (*OrderInfo, error) {
			return &OrderInfo{OrderNo: orderNo, StoreID: 1, BuyerID: 1}, nil
		},
	}
	uc := NewCrmUseCase(repo, nil, nil, nil, orderClient, func() int64 { return 1 }, fakeTxManager{})

	recorded, err := uc.RecordRefund(context.Background(), "evt-refund-2", "O1", "FULL_REFUND", 999999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !recorded {
		t.Fatalf("expected recorded=true")
	}
	if appliedDelta != 0 {
		t.Fatalf("expected amountDelta=0 (S=0, order.completed will never arrive for this order), got %d", appliedDelta)
	}
}

func TestRecordRefund_Full_AfterOrderCompleted_NetsLedgerSum(t *testing.T) {
	repo := newLedgerRepo()
	ctx := context.Background()
	if _, err := repo.AppendLedgerEntryFn(ctx, domain.LedgerEntry{
		EventID: "evt-order-1", EventType: "order_completed", OrderNo: "O1",
		StoreID: 1, BuyerID: 1, AmountDelta: 1000, CountDelta: 1,
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	var appliedDelta int64
	repo.ApplyRefundDeltaFn = func(_ context.Context, _, storeID, buyerID, amountDelta int64) (int64, error) {
		appliedDelta = amountDelta
		return 1, nil
	}
	orderClient := &mockOrderClient{
		GetOrderByNoFn: func(_ context.Context, orderNo string) (*OrderInfo, error) {
			return &OrderInfo{OrderNo: orderNo, StoreID: 1, BuyerID: 1}, nil
		},
	}
	uc := NewCrmUseCase(repo, nil, nil, nil, orderClient, func() int64 { return 1 }, fakeTxManager{})

	recorded, err := uc.RecordRefund(ctx, "evt-refund-3", "O1", "FULL_REFUND", 999999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !recorded {
		t.Fatalf("expected recorded=true")
	}
	if appliedDelta != -1000 {
		t.Fatalf("expected amountDelta=-1000 (S=1000), got %d", appliedDelta)
	}
}

func TestRecordRefund_Full_AfterPriorPartialRefund_NetsRemainder(t *testing.T) {
	repo := newLedgerRepo()
	ctx := context.Background()
	if _, err := repo.AppendLedgerEntryFn(ctx, domain.LedgerEntry{
		EventID: "evt-order-1", EventType: "order_completed", OrderNo: "O1",
		StoreID: 1, BuyerID: 1, AmountDelta: 1000, CountDelta: 1,
	}); err != nil {
		t.Fatalf("setup order.completed: %v", err)
	}
	if _, err := repo.AppendLedgerEntryFn(ctx, domain.LedgerEntry{
		EventID: "evt-refund-partial", EventType: "refund_completed_partial", OrderNo: "O1",
		StoreID: 1, BuyerID: 1, AmountDelta: -200, CountDelta: 0,
	}); err != nil {
		t.Fatalf("setup partial refund: %v", err)
	}
	var appliedDelta int64
	repo.ApplyRefundDeltaFn = func(_ context.Context, _, storeID, buyerID, amountDelta int64) (int64, error) {
		appliedDelta = amountDelta
		return 1, nil
	}
	orderClient := &mockOrderClient{
		GetOrderByNoFn: func(_ context.Context, orderNo string) (*OrderInfo, error) {
			return &OrderInfo{OrderNo: orderNo, StoreID: 1, BuyerID: 1}, nil
		},
	}
	uc := NewCrmUseCase(repo, nil, nil, nil, orderClient, func() int64 { return 1 }, fakeTxManager{})

	recorded, err := uc.RecordRefund(ctx, "evt-refund-full", "O1", "FULL_REFUND", 999999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !recorded {
		t.Fatalf("expected recorded=true")
	}
	if appliedDelta != -800 {
		t.Fatalf("expected amountDelta=-800 (S=1000-200=800), got %d", appliedDelta)
	}
}

func TestFullRefundBeforeOrderCompleted_ThenOrderArrives_ContributesZero(t *testing.T) {
	// Regression test for the RecordOrder symmetric fix: order.completed
	// eventually reaches crm's ledger for an order that was already fully
	// refunded (order-service did publish it — applyFullRefund also accepts
	// COMPLETED as a predecessor — it just arrived at crm after the refund).
	// RecordOrder must not add total_actual_amount on top of an already
	// zero-netted order.
	var totalSpent int64
	var orderCount int32

	repo := newLedgerRepo()
	repo.ApplyRefundDeltaFn = func(_ context.Context, _, storeID, buyerID, amountDelta int64) (int64, error) {
		totalSpent += amountDelta
		return 1, nil
	}
	repo.UpsertCustomerOnOrderFn = func(_ context.Context, id, storeID, buyerID, amount int64, nickname, phone string) error {
		totalSpent += amount
		orderCount++
		return nil
	}
	orderClient := &mockOrderClient{
		GetOrderByNoFn: func(_ context.Context, orderNo string) (*OrderInfo, error) {
			return &OrderInfo{OrderNo: orderNo, StoreID: 1, BuyerID: 1}, nil
		},
	}
	uc := NewCrmUseCase(repo, nil, nil, nil, orderClient, func() int64 { return 1 }, fakeTxManager{})

	recorded, err := uc.RecordRefund(context.Background(), "evt-refund-early", "O1", "FULL_REFUND", 0)
	if err != nil {
		t.Fatalf("refund-before-order: unexpected error: %v", err)
	}
	if !recorded {
		t.Fatalf("refund-before-order: expected recorded=true")
	}
	if totalSpent != 0 || orderCount != 0 {
		t.Fatalf("after early full refund: expected spent=0 orders=0 (S=0, nothing to net), got spent=%d orders=%d", totalSpent, orderCount)
	}

	if _, err := uc.RecordOrder(context.Background(), "evt-order-1", "O1", 10, 20, 1000); err != nil {
		t.Fatalf("order-after-refund: unexpected error: %v", err)
	}
	if totalSpent != 0 || orderCount != 1 {
		t.Fatalf("final state: expected spent=0 (RecordOrder saw HasFullRefundEntry=true, contributed 0) orders=1, got spent=%d orders=%d", totalSpent, orderCount)
	}
}

func TestPartialThenFullRefundBeforeOrderCompleted_ThenOrderArrives_ContributesZero(t *testing.T) {
	var totalSpent int64
	var orderCount int32

	repo := newLedgerRepo()
	repo.ApplyRefundDeltaFn = func(_ context.Context, _, storeID, buyerID, amountDelta int64) (int64, error) {
		totalSpent += amountDelta
		return 1, nil
	}
	repo.UpsertCustomerOnOrderFn = func(_ context.Context, id, storeID, buyerID, amount int64, nickname, phone string) error {
		totalSpent += amount
		orderCount++
		return nil
	}
	orderClient := &mockOrderClient{
		GetOrderByNoFn: func(_ context.Context, orderNo string) (*OrderInfo, error) {
			return &OrderInfo{OrderNo: orderNo, StoreID: 1, BuyerID: 1}, nil
		},
	}
	uc := NewCrmUseCase(repo, nil, nil, nil, orderClient, func() int64 { return 1 }, fakeTxManager{})

	if _, err := uc.RecordRefund(context.Background(), "evt-refund-partial", "O1", "PARTIAL_REFUND", 200); err != nil {
		t.Fatalf("partial refund: %v", err)
	}
	if totalSpent != -200 {
		t.Fatalf("after partial refund: expected spent=-200, got %d", totalSpent)
	}

	if _, err := uc.RecordRefund(context.Background(), "evt-refund-full", "O1", "FULL_REFUND", 0); err != nil {
		t.Fatalf("full refund: %v", err)
	}
	// S so far = -200 -> delta = -(-200) = 200 -> totalSpent = -200+200 = 0
	if totalSpent != 0 {
		t.Fatalf("after full refund: expected spent=0 (fully netted from ledger history alone), got %d", totalSpent)
	}

	if _, err := uc.RecordOrder(context.Background(), "evt-order-1", "O1", 10, 20, 1000); err != nil {
		t.Fatalf("order.completed: %v", err)
	}
	if totalSpent != 0 || orderCount != 1 {
		t.Fatalf("final state: expected spent=0 orders=1, got spent=%d orders=%d", totalSpent, orderCount)
	}
}

func TestRecordRefund_Idempotent_SameEventTwice(t *testing.T) {
	repo := newLedgerRepo()
	applyCalls := 0
	repo.ApplyRefundDeltaFn = func(_ context.Context, _, storeID, buyerID, amountDelta int64) (int64, error) {
		applyCalls++
		return 1, nil
	}
	orderClient := &mockOrderClient{}
	uc := NewCrmUseCase(repo, nil, nil, nil, orderClient, func() int64 { return 1 }, fakeTxManager{})

	if _, err := uc.RecordRefund(context.Background(), "evt-refund-3", "O1", "PARTIAL_REFUND", 100); err != nil {
		t.Fatalf("first call: %v", err)
	}
	recorded2, err := uc.RecordRefund(context.Background(), "evt-refund-3", "O1", "PARTIAL_REFUND", 100)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if recorded2 {
		t.Fatalf("second call (same event_id): expected recorded=false")
	}
	if applyCalls != 1 {
		t.Fatalf("expected exactly 1 ApplyRefundDelta call, got %d", applyCalls)
	}
}

func TestRecordRefund_UnexpectedNonSingleRowFromUpsert_ReturnsError(t *testing.T) {
	// ApplyRefundDelta upserts and should always affect exactly 1 row. A
	// value other than 1 is a genuine DB-level anomaly this test simulates
	// defensively — not a "customer row missing" case (that's now expected
	// and self-corrects via the upsert; see
	// TestRefundBeforeOrderCompleted_ThenOrderArrives_ConvergesCorrectly).
	repo := newLedgerRepo()
	repo.ApplyRefundDeltaFn = func(_ context.Context, _, storeID, buyerID, amountDelta int64) (int64, error) {
		return 0, nil // simulates an unexpected upsert anomaly
	}
	orderClient := &mockOrderClient{}
	uc := NewCrmUseCase(repo, nil, nil, nil, orderClient, func() int64 { return 1 }, fakeTxManager{})

	if _, err := uc.RecordRefund(context.Background(), "evt-refund-4", "O1", "PARTIAL_REFUND", 100); err == nil {
		t.Fatalf("expected error when ApplyRefundDelta returns an unexpected row count, got nil")
	}
}

func TestRecordRefund_OrderCountNeverChanges(t *testing.T) {
	repo := newLedgerRepo()
	var gotCountDelta int32
	origFn := repo.AppendLedgerEntryFn
	repo.AppendLedgerEntryFn = func(ctx context.Context, entry domain.LedgerEntry) (bool, error) {
		gotCountDelta = entry.CountDelta
		return origFn(ctx, entry)
	}
	orderClient := &mockOrderClient{}
	uc := NewCrmUseCase(repo, nil, nil, nil, orderClient, func() int64 { return 1 }, fakeTxManager{})
	if _, err := uc.RecordRefund(context.Background(), "evt-refund-5", "O1", "PARTIAL_REFUND", 50); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCountDelta != 0 {
		t.Fatalf("expected count_delta=0 for a refund event, got %d", gotCountDelta)
	}
}

func TestRefundBeforeOrderCompleted_ThenOrderArrives_ConvergesCorrectly(t *testing.T) {
	// Simulates the real, reachable race documented in the spec: a refund
	// can complete before its order's own order.completed ever arrives,
	// because aftersales cases open at DELIVERED(40), strictly before
	// COMPLETED(50). ApplyRefundDelta must upsert rather than require a
	// pre-existing crm_customers row — this is the regression test for the
	// final-review Critical finding.
	var customerExists bool
	var totalSpent int64
	var orderCount int32

	repo := newLedgerRepo()
	repo.ApplyRefundDeltaFn = func(_ context.Context, _, storeID, buyerID, amountDelta int64) (int64, error) {
		customerExists = true
		totalSpent += amountDelta
		return 1, nil
	}
	repo.UpsertCustomerOnOrderFn = func(_ context.Context, id, storeID, buyerID, amount int64, nickname, phone string) error {
		customerExists = true
		totalSpent += amount
		orderCount++
		return nil
	}
	orderClient := &mockOrderClient{}
	uc := NewCrmUseCase(repo, nil, nil, nil, orderClient, func() int64 { return 1 }, fakeTxManager{})

	// Refund arrives FIRST — this order's own order.completed hasn't happened yet.
	recorded, err := uc.RecordRefund(context.Background(), "evt-refund-early", "O1", "PARTIAL_REFUND", 300)
	if err != nil {
		t.Fatalf("refund-before-order: unexpected error: %v", err)
	}
	if !recorded {
		t.Fatalf("refund-before-order: expected recorded=true")
	}
	if !customerExists || totalSpent != -300 || orderCount != 0 {
		t.Fatalf("after early refund: expected exists=true spent=-300 orders=0, got exists=%v spent=%d orders=%d", customerExists, totalSpent, orderCount)
	}

	// order.completed arrives afterward, for the same order, full amount 1000.
	if _, err := uc.RecordOrder(context.Background(), "evt-order-1", "O1", 10, 20, 1000); err != nil {
		t.Fatalf("order-after-refund: unexpected error: %v", err)
	}
	if totalSpent != 700 || orderCount != 1 {
		t.Fatalf("final state: expected spent=700 (1000-300) orders=1, got spent=%d orders=%d", totalSpent, orderCount)
	}
}
