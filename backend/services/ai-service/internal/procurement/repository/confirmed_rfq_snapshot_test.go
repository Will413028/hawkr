//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/stretchr/testify/require"
)

func TestRepo_InsertConfirmedRfqSnapshot_CreatesRow(t *testing.T) {
	pool, cleanup := setupProcurementTestDB(context.Background(), t)
	defer cleanup()
	r := New(pool, database.NewTxManager(pool), testIDGen())

	got, err := r.InsertConfirmedRfqSnapshot(context.Background(), InsertConfirmedRfqSnapshotInput{
		BuyerID: testBuyerID, RequestID: 1, RequestRevisionID: 1,
		PricePreviewID: 1, PricePreviewVersion: 1,
		QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN", PricingMode: "PRICEBOOK_GUIDED",
		CanonicalPayloadJSON: json.RawMessage(`{"item":"寶島梨"}`),
		PayloadHash:          "hash-snapshot", ClientRequestID: "confirm-key-1",
		ExpiresAt: refTime().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("InsertConfirmedRfqSnapshot() error = %v, want nil", err)
	}
	if got.ID == 0 {
		t.Errorf("ID = 0, want non-zero")
	}

	var count int
	err = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM procurement_confirmed_rfq_snapshots WHERE id = $1`, got.ID).Scan(&count)
	if err != nil {
		t.Fatalf("QueryRow error = %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestRepo_InsertConfirmedRfqSnapshot_ConcurrentSameClientRequestID_IsIdempotent(t *testing.T) {
	pool, cleanup := setupProcurementTestDB(context.Background(), t)
	defer cleanup()
	r := New(pool, database.NewTxManager(pool), testIDGen())

	input := InsertConfirmedRfqSnapshotInput{
		BuyerID: testBuyerID, RequestID: 1, RequestRevisionID: 1,
		PricePreviewID: 1, PricePreviewVersion: 1,
		QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN", PricingMode: "PRICEBOOK_GUIDED",
		CanonicalPayloadJSON: json.RawMessage(`{"item":"寶島梨"}`),
		PayloadHash:          "hash-snapshot", ClientRequestID: "confirm-key-race",
		ExpiresAt: refTime().Add(24 * time.Hour),
	}

	var wg sync.WaitGroup
	results := make([]ConfirmedRfqSnapshot, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = r.InsertConfirmedRfqSnapshot(context.Background(), input)
		}(i)
	}
	wg.Wait()

	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("errs = %v, %v, want both nil", errs[0], errs[1])
	}
	if results[0].ID != results[1].ID {
		t.Errorf("同 client_request_id 併發呼叫必須回傳同一個 ID，got %d and %d", results[0].ID, results[1].ID)
	}

	var count int64
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM procurement_confirmed_rfq_snapshots WHERE buyer_id = $1 AND client_request_id = $2`,
		testBuyerID, "confirm-key-race").Scan(&count)
	if err != nil {
		t.Fatalf("QueryRow error = %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1（無論哪個先寫贏，DB 只能留下一筆）", count)
	}
}

// TestRepo_InsertConfirmedRfqSnapshot_SameClientRequestIDDifferentTarget_ReturnsIdempotencyConflict
// (Task 18／Fix 7) 是 price_preview_test.go 那個回歸測試的 confirmed-snapshot 版本：
// 應用層短路檢查（commerce.ConfirmRfqSnapshotUseCase，Task 17）比對
// RequestID／PricePreviewID／PricePreviewVersion 三個欄位，但那是預先查詢、不是唯一
// 防線。這裡讓第二次呼叫的 PricePreviewID 不同，直接送進 InsertConfirmedRfqSnapshot
// 撞同一個 (buyer_id, client_request_id) 鍵，驗證撞鍵後的比對邏輯真的會發現不一致，
// 回 ErrConfirmedRfqSnapshotIdempotencyConflict，不能靜默回放第一次的結果。
func TestRepo_InsertConfirmedRfqSnapshot_SameClientRequestIDDifferentTarget_ReturnsIdempotencyConflict(t *testing.T) {
	pool, cleanup := setupProcurementTestDB(context.Background(), t)
	defer cleanup()
	r := New(pool, database.NewTxManager(pool), testIDGen())

	first := InsertConfirmedRfqSnapshotInput{
		BuyerID: testBuyerID, RequestID: 1, RequestRevisionID: 1,
		PricePreviewID: 1, PricePreviewVersion: 1,
		QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN", PricingMode: "PRICEBOOK_GUIDED",
		CanonicalPayloadJSON: json.RawMessage(`{"item":"寶島梨"}`),
		PayloadHash:          "hash-snapshot-1", ClientRequestID: "confirm-key-conflict",
		ExpiresAt: refTime().Add(24 * time.Hour),
	}
	if _, err := r.InsertConfirmedRfqSnapshot(context.Background(), first); err != nil {
		t.Fatalf("first InsertConfirmedRfqSnapshot() error = %v, want nil", err)
	}

	second := first
	second.PricePreviewID = 2 // 同一個 clientRequestID，但真的是對另一筆 price preview 的確認請求
	second.PayloadHash = "hash-snapshot-2"
	_, err := r.InsertConfirmedRfqSnapshot(context.Background(), second)
	if !errors.Is(err, ErrConfirmedRfqSnapshotIdempotencyConflict) {
		t.Errorf("err = %v, want ErrConfirmedRfqSnapshotIdempotencyConflict", err)
	}

	var count int64
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM procurement_confirmed_rfq_snapshots WHERE buyer_id = $1 AND client_request_id = $2`,
		testBuyerID, "confirm-key-conflict").Scan(&count); err != nil {
		t.Fatalf("QueryRow error = %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1（conflict 不得建立第二筆）", count)
	}
}

// TestRepo_FindLatestConfirmedRfqSnapshotByRequestID_ReturnsMostRecent 與
// TestRepo_FindLatestConfirmedRfqSnapshotByRequestID_NotFound（#2549 Phase 1 Task 2）
// 驗證新的 FindLatestConfirmedRfqSnapshotByRequestID 讀取路徑：同一個 request 可能有
// 多筆確認快照（rewind／改動後重新確認），只有「最新一筆」才是 Task 3 的
// RfqConfirmationReader 判斷「現在算不算過期」的正確依據。

func TestRepo_FindLatestConfirmedRfqSnapshotByRequestID_ReturnsMostRecent(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	req, _, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	// 同一個 request 兩筆確認快照（模擬 revision 變動後重新確認），第二筆較晚建立。
	_, err = repo.InsertConfirmedRfqSnapshot(ctx, InsertConfirmedRfqSnapshotInput{
		BuyerID: testBuyerID, RequestID: req.ID, RequestRevisionID: 1,
		PricePreviewID: 1, PricePreviewVersion: 1,
		QualityKnowledgeState: "KNOWN", PricingMode: "PRICEBOOK_GUIDED",
		CanonicalPayloadJSON: json.RawMessage(`{}`), PayloadHash: "h1",
		ClientRequestID: "cr-1", ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	second, err := repo.InsertConfirmedRfqSnapshot(ctx, InsertConfirmedRfqSnapshotInput{
		BuyerID: testBuyerID, RequestID: req.ID, RequestRevisionID: 2,
		PricePreviewID: 2, PricePreviewVersion: 1,
		QualityKnowledgeState: "KNOWN", PricingMode: "PRICEBOOK_GUIDED",
		CanonicalPayloadJSON: json.RawMessage(`{}`), PayloadHash: "h2",
		ClientRequestID: "cr-2", ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	got, err := repo.FindLatestConfirmedRfqSnapshotByRequestID(ctx, req.ID, testBuyerID)
	require.NoError(t, err)
	require.Equal(t, second.ID, got.ID)
	require.Equal(t, int64(2), got.RequestRevisionID)
}

func TestRepo_FindLatestConfirmedRfqSnapshotByRequestID_NotFound(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	_, err := repo.FindLatestConfirmedRfqSnapshotByRequestID(ctx, 999999, testBuyerID)
	require.ErrorIs(t, err, ErrConfirmedRfqSnapshotNotFound)
}
