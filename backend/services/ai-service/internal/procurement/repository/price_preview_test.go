//go:build integration

package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/database"
)

func TestRepo_InsertPricePreview_CreatesRow(t *testing.T) {
	pool, cleanup := setupProcurementTestDB(context.Background(), t)
	defer cleanup()
	r := New(pool, database.NewTxManager(pool), testIDGen())

	got, err := r.InsertPricePreview(context.Background(), InsertPricePreviewInput{
		BuyerID:                 testBuyerID,
		RequestID:               1,
		RequestRevisionID:       1,
		BuyerRequestedRangeJSON: []byte(`{}`),
		ClientRequestID:         "preview-key-1",
		PreviewInputHash:        "hash-in",
		PayloadHash:             "hash-out",
		ExpiresAt:               refTime().Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("InsertPricePreview() error = %v, want nil", err)
	}
	if got.ID == 0 {
		t.Errorf("ID = 0, want non-zero")
	}

	var count int
	err = pool.QueryRow(context.Background(),
		`SELECT count(*) FROM procurement_price_previews WHERE id = $1`, got.ID).Scan(&count)
	if err != nil {
		t.Fatalf("QueryRow error = %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestRepo_InsertPricePreview_ConcurrentSameClientRequestID_IsIdempotent(t *testing.T) {
	pool, cleanup := setupProcurementTestDB(context.Background(), t)
	defer cleanup()
	r := New(pool, database.NewTxManager(pool), testIDGen())

	input := InsertPricePreviewInput{
		BuyerID: testBuyerID, RequestID: 1, RequestRevisionID: 1,
		BuyerRequestedRangeJSON: []byte(`{}`),
		ClientRequestID:         "preview-key-race",
		PreviewInputHash:        "hash-in", PayloadHash: "hash-out", ExpiresAt: refTime().Add(15 * time.Minute),
	}

	var wg sync.WaitGroup
	results := make([]PricePreview, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = r.InsertPricePreview(context.Background(), input)
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
		`SELECT count(*) FROM procurement_price_previews WHERE buyer_id = $1 AND client_request_id = $2`,
		testBuyerID, "preview-key-race").Scan(&count)
	if err != nil {
		t.Fatalf("QueryRow error = %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1（無論哪個先寫贏，DB 只能留下一筆）", count)
	}
}

// TestRepo_InsertPricePreview_SameClientRequestIDDifferentPayload_ReturnsIdempotencyConflict
// (Task 18／Fix 7) 涵蓋應用層短路檢查（pricing.PreviewRfqPriceUseCase 呼叫這支方法之前的
// FindPricePreviewByIdempotencyKey 比對，Task 15／I3）抓不到的競態視窗：兩個「同一個
// clientRequestID、不同 payload」的呼叫真的併發到都沒查到既有列、直接送進這支 INSERT，
// 撞 DB 的 uq_procurement_price_previews_idempotency 唯一索引時，撞鍵後查到的既有列
// 必須跟這次輸入的 PreviewInputHash 比對，發現不一致要回
// ErrPricePreviewIdempotencyConflict，不能靜默回放撞鍵前那一列。這裡用循序呼叫（不需要
// 真的併發）就能重現：第一次先落地，第二次帶不同 PreviewInputHash 撞同一個鍵。
func TestRepo_InsertPricePreview_SameClientRequestIDDifferentPayload_ReturnsIdempotencyConflict(t *testing.T) {
	pool, cleanup := setupProcurementTestDB(context.Background(), t)
	defer cleanup()
	r := New(pool, database.NewTxManager(pool), testIDGen())

	first := InsertPricePreviewInput{
		BuyerID: testBuyerID, RequestID: 1, RequestRevisionID: 1,
		BuyerRequestedRangeJSON: []byte(`{}`),
		ClientRequestID:         "preview-key-conflict",
		PreviewInputHash:        "hash-in-1", PayloadHash: "hash-out-1", ExpiresAt: refTime().Add(15 * time.Minute),
	}
	if _, err := r.InsertPricePreview(context.Background(), first); err != nil {
		t.Fatalf("first InsertPricePreview() error = %v, want nil", err)
	}

	second := first
	second.PreviewInputHash = "hash-in-2"
	second.PayloadHash = "hash-out-2"
	_, err := r.InsertPricePreview(context.Background(), second)
	if !errors.Is(err, ErrPricePreviewIdempotencyConflict) {
		t.Errorf("err = %v, want ErrPricePreviewIdempotencyConflict", err)
	}

	var count int64
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM procurement_price_previews WHERE buyer_id = $1 AND client_request_id = $2`,
		testBuyerID, "preview-key-conflict").Scan(&count); err != nil {
		t.Fatalf("QueryRow error = %v", err)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1（conflict 不得建立第二筆）", count)
	}
}

func TestRepo_FindPricePreviewByIDAndBuyer_WrongBuyerID_ReturnsNotFound(t *testing.T) {
	pool, cleanup := setupProcurementTestDB(context.Background(), t)
	defer cleanup()
	r := New(pool, database.NewTxManager(pool), testIDGen())

	inserted, err := r.InsertPricePreview(context.Background(), InsertPricePreviewInput{
		BuyerID:                 testBuyerID,
		RequestID:               1,
		RequestRevisionID:       1,
		BuyerRequestedRangeJSON: []byte(`{}`),
		ClientRequestID:         "preview-key-owner-check",
		PreviewInputHash:        "hash-in",
		PayloadHash:             "hash-out",
		ExpiresAt:               refTime().Add(15 * time.Minute),
	})
	if err != nil {
		t.Fatalf("InsertPricePreview() error = %v, want nil", err)
	}

	// 用另一個 buyer id 查詢同一筆 preview：必須回 ErrPricePreviewNotFound，不能回傳
	// 這筆屬於別人的資料——wrong-owner 存取跟「id 根本不存在」必須是同一種錯誤形狀。
	wrongBuyerID := testBuyerID + 1
	_, err = r.FindPricePreviewByIDAndBuyer(context.Background(), inserted.ID, wrongBuyerID)
	if err != ErrPricePreviewNotFound {
		t.Errorf("err = %v, want ErrPricePreviewNotFound", err)
	}

	// 正確的 owner 仍查得到，確認上面的 not-found 不是查詢本身壞掉。
	got, err := r.FindPricePreviewByIDAndBuyer(context.Background(), inserted.ID, testBuyerID)
	if err != nil {
		t.Fatalf("FindPricePreviewByIDAndBuyer() with correct owner error = %v, want nil", err)
	}
	if got.ID != inserted.ID {
		t.Errorf("ID = %d, want %d", got.ID, inserted.ID)
	}
}
