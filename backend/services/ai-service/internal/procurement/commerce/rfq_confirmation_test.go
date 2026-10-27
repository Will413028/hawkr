package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/pricing"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// validPreviewCAS 是「preview 完全不過期、完全對得上這次 confirm」的 CAS
// 欄位基準值——除了明確在測 CAS staleness（Task 14）的測試以外，其餘既有
// 測試（Task 9／10 建立）都用這組值，只是想驗證 confirm 流程本身，不是想測
// CAS，所以要讓三個新 guard（RequestID／RequestRevisionID／ExpiresAt）全部
// 通過。RequestID／RequestRevisionID 對齊這批既有測試共用的
// RequestID:1／testkit.FakeRevisionReader{RevisionID: 1}。
const validPreviewRequestID = int64(1)
const validPreviewRevisionID = int64(1)

func validPreviewExpiresAt() time.Time { return time.Now().Add(time.Hour) }

// fakeConfirmedRfqSnapshotRepo 的方法簽章逐字對齊 Task 8 定義的
// repository.Repo.InsertConfirmedRfqSnapshot／FindConfirmedRfqSnapshotByIdempotencyKey
// ——跟 Task 6 的 fakePricePreviewRepo 是同一種修正（不自訂窄型別）。
type fakeConfirmedRfqSnapshotRepo struct {
	insertCalls      int
	byIdempotencyKey map[string]repository.ConfirmedRfqSnapshot
	lastPayload      json.RawMessage
}

func (f *fakeConfirmedRfqSnapshotRepo) InsertConfirmedRfqSnapshot(ctx context.Context, in repository.InsertConfirmedRfqSnapshotInput) (repository.ConfirmedRfqSnapshot, error) {
	f.insertCalls++
	f.lastPayload = in.CanonicalPayloadJSON
	row := repository.ConfirmedRfqSnapshot{
		ID: int64(f.insertCalls), BuyerID: in.BuyerID, RequestID: in.RequestID,
		RequestRevisionID: in.RequestRevisionID, PricePreviewID: in.PricePreviewID,
		PricePreviewVersion: in.PricePreviewVersion, QualityKnowledgeState: in.QualityKnowledgeState,
		PricingMode: in.PricingMode, CanonicalPayloadJSON: in.CanonicalPayloadJSON,
		PayloadHash: in.PayloadHash, ClientRequestID: in.ClientRequestID, ExpiresAt: in.ExpiresAt,
	}
	if f.byIdempotencyKey == nil {
		f.byIdempotencyKey = map[string]repository.ConfirmedRfqSnapshot{}
	}
	f.byIdempotencyKey[in.ClientRequestID] = row
	return row, nil
}

func (f *fakeConfirmedRfqSnapshotRepo) FindConfirmedRfqSnapshotByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (repository.ConfirmedRfqSnapshot, error) {
	if existing, ok := f.byIdempotencyKey[clientRequestID]; ok {
		return existing, nil
	}
	return repository.ConfirmedRfqSnapshot{}, repository.ErrConfirmedRfqSnapshotNotFound
}

// fakePricePreviewReader 的 GetPricePreview 帶 buyerID 參數，模擬真實實作
// （Task 5 的 FindPricePreviewByIDAndBuyer）依 (id, buyer_id) 查詢——wrong-owner
// 猜 ID 必須查不到，不能只憑 id 查。
type fakePricePreviewReader struct {
	preview pricing.PricePreviewResult
	ownerID identity.BuyerID
}

func (f fakePricePreviewReader) GetPricePreview(ctx context.Context, id int64, buyerID identity.BuyerID) (pricing.PricePreviewResult, error) {
	if buyerID != f.ownerID {
		return pricing.PricePreviewResult{}, repository.ErrPricePreviewNotFound
	}
	return f.preview, nil
}

func TestConfirmRfqSnapshot_ValidInput_CreatesImmutableSnapshot(t *testing.T) {
	repo := &fakeConfirmedRfqSnapshotRepo{}
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1, PayloadHash: "preview-hash",
		RequestID: validPreviewRequestID, RequestRevisionID: validPreviewRevisionID, ExpiresAt: validPreviewExpiresAt(),
	}}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, testkit.FakeRevisionReader{RevisionID: 1})

	got, err := uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-1",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	})
	if err != nil {
		t.Fatalf("ConfirmRfqSnapshot() error = %v, want nil", err)
	}
	if got.ConfirmedRfqSnapshotID == 0 {
		t.Errorf("ConfirmedRfqSnapshotID = 0, want non-zero")
	}
}

func TestConfirmRfqSnapshot_StalePricePreviewVersion_ReturnsTypedConflict(t *testing.T) {
	repo := &fakeConfirmedRfqSnapshotRepo{}
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 2, PayloadHash: "preview-hash",
		RequestID: validPreviewRequestID, RequestRevisionID: validPreviewRevisionID, ExpiresAt: validPreviewExpiresAt(),
	}}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, testkit.FakeRevisionReader{RevisionID: 1})

	_, err := uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-2",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	})
	if !errors.Is(err, ErrStalePricePreview) {
		t.Errorf("err = %v, want ErrStalePricePreview", err)
	}
}

func TestConfirmRfqSnapshot_PayloadHashMismatch_ReturnsTypedConflict(t *testing.T) {
	repo := &fakeConfirmedRfqSnapshotRepo{}
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1, PayloadHash: "actual-hash",
		RequestID: validPreviewRequestID, RequestRevisionID: validPreviewRevisionID, ExpiresAt: validPreviewExpiresAt(),
	}}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, testkit.FakeRevisionReader{RevisionID: 1})

	_, err := uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-3",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "stale-client-side-hash",
	})
	if !errors.Is(err, ErrStalePricePreview) {
		t.Errorf("err = %v, want ErrStalePricePreview", err)
	}
}

func TestConfirmRfqSnapshot_SellerFacingProjection_ExcludesRawQuantilesAndLaneMarkup(t *testing.T) {
	repo := &fakeConfirmedRfqSnapshotRepo{}
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1, PayloadHash: "preview-hash",
		ReferencePriceRange: &pricing.ReferencePriceRange{Lower: testkit.Ptr(30.25), Midpoint: 35.5, Upper: testkit.Ptr(41.75)},
		RequestID:           validPreviewRequestID, RequestRevisionID: validPreviewRevisionID, ExpiresAt: validPreviewExpiresAt(),
	}}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, testkit.FakeRevisionReader{RevisionID: 1})

	_, err := uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-4",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	})
	if err != nil {
		t.Fatalf("ConfirmRfqSnapshot() error = %v, want nil", err)
	}

	payload := string(repo.lastPayload)
	for _, forbidden := range []string{"p10", "P10", "p90", "P90", "aggregationMethod", "AggregationMethod", "markup", "Markup", "budgetPerUnit", "BudgetPerUnit"} {
		if strings.Contains(payload, forbidden) {
			t.Errorf("canonical_payload_json 含禁止欄位 %q，內容：%s", forbidden, payload)
		}
	}
}

func TestConfirmRfqSnapshot_IdempotentReplay_SameClientRequestIDReturnsExistingSnapshot(t *testing.T) {
	repo := &fakeConfirmedRfqSnapshotRepo{}
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1, PayloadHash: "preview-hash",
		RequestID: validPreviewRequestID, RequestRevisionID: validPreviewRevisionID, ExpiresAt: validPreviewExpiresAt(),
	}}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, testkit.FakeRevisionReader{RevisionID: 1})

	in := ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-5",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	}
	first, err := uc.ConfirmRfqSnapshot(context.Background(), in)
	if err != nil {
		t.Fatalf("first call error = %v", err)
	}
	second, err := uc.ConfirmRfqSnapshot(context.Background(), in)
	if err != nil {
		t.Fatalf("second call error = %v", err)
	}
	if first.ConfirmedRfqSnapshotID != second.ConfirmedRfqSnapshotID {
		t.Errorf("ConfirmedRfqSnapshotID = %d then %d, want same", first.ConfirmedRfqSnapshotID, second.ConfirmedRfqSnapshotID)
	}
	if repo.insertCalls != 1 {
		t.Errorf("insertCalls = %d, want 1", repo.insertCalls)
	}
}

// TestConfirmRfqSnapshot_IdempotencyKeyReusedForDifferentRequest_ReturnsConflict
// (Task 17) 涵蓋 Task 15 在 PreviewRfqPriceUseCase 修過的同一種 bug，這裡是
// ConfirmRfqSnapshotUseCase 的版本：冪等鍵命中前若不比對「這次呼叫的輸入」是
// 否真的等於「當初存下 existing 時的輸入」，就會把第一次呼叫的結果錯誤地當成
// 第二次呼叫的重播結果回傳。這裡換的是 RequestID。
func TestConfirmRfqSnapshot_IdempotencyKeyReusedForDifferentRequest_ReturnsConflict(t *testing.T) {
	repo := &fakeConfirmedRfqSnapshotRepo{}
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1, PayloadHash: "preview-hash",
		RequestID: validPreviewRequestID, RequestRevisionID: validPreviewRevisionID, ExpiresAt: validPreviewExpiresAt(),
	}}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, testkit.FakeRevisionReader{RevisionID: 1})

	first, err := uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-conflict-request",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	})
	if err != nil {
		t.Fatalf("first call error = %v, want nil", err)
	}
	if first.ConfirmedRfqSnapshotID == 0 {
		t.Fatalf("first.ConfirmedRfqSnapshotID = 0, want non-zero（setup 失敗）")
	}

	// 第二次呼叫換了 RequestID，卻沿用同一個 ClientRequestID——這不是重送，
	// 是把冪等鍵當成萬用鍵重用在另一個 request 上。
	_, err = uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 2, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-conflict-request",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	})
	if !errors.Is(err, domain.ErrIdempotencyKeyConflict) {
		t.Errorf("err = %v, want domain.ErrIdempotencyKeyConflict", err)
	}
	if repo.insertCalls != 1 {
		t.Errorf("insertCalls = %d, want 1（第二次呼叫零寫入）", repo.insertCalls)
	}
}

// TestConfirmRfqSnapshot_IdempotencyKeyReusedForDifferentPricePreview_ReturnsConflict
// (Task 17) 同一種 bug，換的是同一個 RequestID 底下的 PricePreviewID——買家對
// 同一個 request 重新試算出第二版 preview（不同 pricePreviewId），卻沿用了舊的
// clientRequestId 來 confirm。
func TestConfirmRfqSnapshot_IdempotencyKeyReusedForDifferentPricePreview_ReturnsConflict(t *testing.T) {
	repo := &fakeConfirmedRfqSnapshotRepo{}
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1, PayloadHash: "preview-hash",
		RequestID: validPreviewRequestID, RequestRevisionID: validPreviewRevisionID, ExpiresAt: validPreviewExpiresAt(),
	}}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, testkit.FakeRevisionReader{RevisionID: 1})

	_, err := uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-conflict-preview",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	})
	if err != nil {
		t.Fatalf("first call error = %v, want nil", err)
	}

	_, err = uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-conflict-preview",
		ExpectedRevisionID: 1, PricePreviewID: 2, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	})
	if !errors.Is(err, domain.ErrIdempotencyKeyConflict) {
		t.Errorf("err = %v, want domain.ErrIdempotencyKeyConflict", err)
	}
	if repo.insertCalls != 1 {
		t.Errorf("insertCalls = %d, want 1（第二次呼叫零寫入）", repo.insertCalls)
	}
}

// TestConfirmRfqSnapshot_IdempotencyKeyReusedForDifferentPricePreviewVersion_ReturnsConflict
// (Task 17) 同一種 bug，換的是同一個 PricePreviewID 底下的 PricePreviewVersion。
// InsertPricePreview 目前把 Version 寫死成 1（見本檔既有測試註解），但這裡驗
// 證的是 usecase 層比對邏輯本身是否涵蓋這個欄位，不依賴上游現況是否真的會
// 產生第二個 version。
func TestConfirmRfqSnapshot_IdempotencyKeyReusedForDifferentPricePreviewVersion_ReturnsConflict(t *testing.T) {
	repo := &fakeConfirmedRfqSnapshotRepo{}
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1, PayloadHash: "preview-hash",
		RequestID: validPreviewRequestID, RequestRevisionID: validPreviewRevisionID, ExpiresAt: validPreviewExpiresAt(),
	}}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, testkit.FakeRevisionReader{RevisionID: 1})

	_, err := uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-conflict-version",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	})
	if err != nil {
		t.Fatalf("first call error = %v, want nil", err)
	}

	_, err = uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-conflict-version",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 2, PayloadHash: "preview-hash",
	})
	if !errors.Is(err, domain.ErrIdempotencyKeyConflict) {
		t.Errorf("err = %v, want domain.ErrIdempotencyKeyConflict", err)
	}
	if repo.insertCalls != 1 {
		t.Errorf("insertCalls = %d, want 1（第二次呼叫零寫入）", repo.insertCalls)
	}
}

func TestConfirmRfqSnapshot_WrongOwner_ReturnsNotFound(t *testing.T) {
	repo := &fakeConfirmedRfqSnapshotRepo{}
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1, PayloadHash: "preview-hash",
	}}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, testkit.FakeRevisionReader{RevisionID: 1})

	// BuyerID(2) 猜中屬於 BuyerID(1) 的 pricePreviewId=1，必須查不到——不能
	// 洩漏「preview 存在但不是你的」與「preview 根本不存在」的差別。
	_, err := uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(2), ClientRequestID: "confirm-6",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	})
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want a not-found error（wrong-owner 與 random ID 必須回同一種 404 shape）", err)
	}
	if repo.insertCalls != 0 {
		t.Errorf("insertCalls = %d, want 0（零 side effect）", repo.insertCalls)
	}
}

// TestConfirmRfqSnapshot_RevisionChangedAfterPreview_ReturnsStaleRevision
// (Task 14／I1) 涵蓋一個舊版 CAS 檢查漏抓的情境：preview 是針對 revision 1
// 算出來的（RequestRevisionID: 1），但這個 request 的現行 revision 之後變成
// 2（testkit.FakeRevisionReader{RevisionID: 2}）。買家帶著 ExpectedRevisionID=2——
// 跟現行 revision 一致，能通過既有的 `rev.ID != in.ExpectedRevisionID` 檢查
// ——卻拿著針對舊 revision 算出的 preview 來 confirm。Version／PayloadHash
// 故意設成跟 input 完全吻合，確保真正擋下這個情境的是新的
// RequestRevisionID guard，不是舊的 echo 檢查。
func TestConfirmRfqSnapshot_RevisionChangedAfterPreview_ReturnsStaleRevision(t *testing.T) {
	repo := &fakeConfirmedRfqSnapshotRepo{}
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1, PayloadHash: "preview-hash",
		RequestID: validPreviewRequestID, RequestRevisionID: 1, ExpiresAt: validPreviewExpiresAt(),
	}}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, testkit.FakeRevisionReader{RevisionID: 2})

	_, err := uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-revision-changed",
		ExpectedRevisionID: 2, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	})
	if !errors.Is(err, pricing.ErrStalePricePreviewRevision) {
		t.Errorf("err = %v, want ErrStalePricePreviewRevision", err)
	}
	if repo.insertCalls != 0 {
		t.Errorf("insertCalls = %d, want 0（零 side effect）", repo.insertCalls)
	}
}

// TestConfirmRfqSnapshot_CrossRequestPreviewReuse_ReturnsStaleRevision (Task
// 14／I1，reviewer 修正於 Task 14 fix round) 涵蓋
// FindPricePreviewByIDAndBuyer 刻意只用 (id, buyerID) 查、不收斂到 request
// 之後留下的漏洞：preview 是買家在 request 1 底下算出來的（RequestID: 1），
// 這次卻拿它去 confirm request 2。GetPricePreview 本身不會擋下這個情境
// （buyerID 相符），必須靠 usecase 層額外的 cross-request guard。
//
// 錯誤映射：沿用 ErrStalePricePreviewRevision（Aborted／STALE_REVISION），
// 不是 repository.ErrPricePreviewNotFound（404）——wrong-owner 404 的隱私
// 理由（不洩漏「這個 ID 存在，只是不是你的」給不同買家）在這裡不成立，
// 因為這是同一個買家的另一個 request，不是別的買家；跟緊鄰的
// revision-changed-after-preview guard 同一種成因（client 端狀態問題）與
// 同一種正確復原方式（回到現在這個 request 重新 PreviewRfqPrice 再
// confirm），見 rfq_confirmation.go 該 guard 的 doc comment。
func TestConfirmRfqSnapshot_CrossRequestPreviewReuse_ReturnsStaleRevision(t *testing.T) {
	repo := &fakeConfirmedRfqSnapshotRepo{}
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1, PayloadHash: "preview-hash",
		RequestID: 1, RequestRevisionID: validPreviewRevisionID, ExpiresAt: validPreviewExpiresAt(),
	}}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, testkit.FakeRevisionReader{RevisionID: 1})

	_, err := uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 2, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-cross-request",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	})
	if !errors.Is(err, pricing.ErrStalePricePreviewRevision) {
		t.Errorf("err = %v, want ErrStalePricePreviewRevision", err)
	}
	if repo.insertCalls != 0 {
		t.Errorf("insertCalls = %d, want 0（零 side effect）", repo.insertCalls)
	}
}

// TestConfirmRfqSnapshot_ExpiredPreview_ReturnsStalePricePreview (Task
// 14／I1) 涵蓋 pricePreviewTTL 從未被實際檢查的漏洞：preview.ExpiresAt 早於
// confirm 當下的時間。用 uc.now 注入固定時鐘（同 answer.go
// AnswerUseCase.now 的既有慣例），不依賴 sleep 或本機時區（CLAUDE.md §21）。
func TestConfirmRfqSnapshot_ExpiredPreview_ReturnsStalePricePreview(t *testing.T) {
	repo := &fakeConfirmedRfqSnapshotRepo{}
	fixedExpiry := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1, PayloadHash: "preview-hash",
		RequestID: validPreviewRequestID, RequestRevisionID: validPreviewRevisionID, ExpiresAt: fixedExpiry,
	}}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, testkit.FakeRevisionReader{RevisionID: 1})
	uc.now = func() time.Time { return fixedExpiry.Add(time.Minute) }

	_, err := uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-expired",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	})
	if !errors.Is(err, ErrStalePricePreview) {
		t.Errorf("err = %v, want ErrStalePricePreview", err)
	}
	if repo.insertCalls != 0 {
		t.Errorf("insertCalls = %d, want 0（零 side effect）", repo.insertCalls)
	}
}

// confirmSlotsFixture 是一張「報價就緒」的詢價單 slots：六個賣家報價一定要看到的
// 欄位（品項、數量、單位、包裝、配送城市、交期）都有值，另外帶買家私有／非
// seller-facing 的欄位（budget_per_unit、exact_match_required、
// item_resolution.price_per_kg），用來讓白名單測試有東西可擋。
func confirmSlotsFixture(item string, quantity int, city string) domain.Slots {
	return domain.Slots{
		Item: testkit.Ptr(item),
		ItemResolution: &domain.ItemResolution{
			Kind: domain.ItemResolvedCrop, Code: testkit.Ptr("CABBAGE_EARLY_AUTUMN"),
			Label: "甘藍-初秋", FallbackGroupCode: testkit.Ptr("CABBAGE"),
			PricePerKg: testkit.Ptr(42), Provenance: domain.ItemResolutionBySystem,
		},
		Quantity:           testkit.Ptr(quantity),
		Unit:               testkit.Ptr("KG"),
		Packaging:          testkit.Ptr("紙箱"),
		BudgetPerUnit:      testkit.Ptr(38),
		DeliveryCity:       testkit.Ptr(city),
		DeliveryDate:       testkit.Ptr("2026-09-20"),
		ExactMatchRequired: testkit.Ptr(true),
	}
}

// confirmPreviewFixture 是一組固定的定價事實——兩個測試 case 共用同一份，才能
// 證明 hash 的差異只可能來自詢價內容本身。
func confirmPreviewFixture() pricing.PricePreviewResult {
	return pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1, PayloadHash: "preview-hash",
		PricingMode:         "PRICEBOOK_GUIDED",
		ReferencePriceRange: &pricing.ReferencePriceRange{Lower: testkit.Ptr(30.25), Midpoint: 35.5, Upper: testkit.Ptr(41.75)},
		BuyerRequestedRange: pricing.BuyerRequestedRange{
			Currency: "TWD", PriceComponent: "GOODS_ONLY",
			LowerBoundAmount: "30", UpperBoundAmount: "42",
			BasisQuantity: "1", BasisUnit: "KG",
		},
		RequestID: validPreviewRequestID, RequestRevisionID: validPreviewRevisionID,
		ExpiresAt: validPreviewExpiresAt(),
	}
}

func confirmWithSlots(t *testing.T, slots domain.Slots, clientRequestID string) (ConfirmedRfqSnapshotResult, json.RawMessage) {
	t.Helper()
	repo := &fakeConfirmedRfqSnapshotRepo{}
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: confirmPreviewFixture()}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, testkit.FakeRevisionReader{RevisionID: 1, Slots: slots})

	got, err := uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: clientRequestID,
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	})
	if err != nil {
		t.Fatalf("ConfirmRfqSnapshot() error = %v, want nil", err)
	}
	return got, repo.lastPayload
}

// 這是 D1 決策 B 存在的理由：擴大 preimage 之前，兩張品項／數量／配送城市完全
// 不同的詢價單，只要 pricing mode 與價格區間相同，payload_hash 就一模一樣——
// 買家接受時拿它做 CAS，證明不了賣家報價當時看到的是哪一張詢價單。
func TestConfirmRfqSnapshot_SamePricingDifferentRequirement_ProducesDifferentPayloadHash(t *testing.T) {
	first, _ := confirmWithSlots(t, confirmSlotsFixture("高麗菜", 300, "臺中市"), "confirm-hash-a")
	second, _ := confirmWithSlots(t, confirmSlotsFixture("青江菜", 50, "臺南市"), "confirm-hash-b")

	if first.PayloadHash == second.PayloadHash {
		t.Errorf("兩張詢價內容不同的單子算出同一個 payloadHash %q——preimage 沒有涵蓋詢價內容", first.PayloadHash)
	}
}

// 逐欄位版本：每個新納入 preimage 的欄位都必須單獨有鑑別力，否則「加了欄位」
// 只是看起來有做，實際上某一欄仍然不影響 hash。
func TestConfirmRfqSnapshot_EachRequirementField_ChangesPayloadHash(t *testing.T) {
	base := confirmSlotsFixture("高麗菜", 300, "臺中市")
	baseResult, _ := confirmWithSlots(t, base, "confirm-field-base")

	cases := map[string]func(domain.Slots) domain.Slots{
		"item": func(s domain.Slots) domain.Slots { s.Item = testkit.Ptr("青江菜"); return s },
		"itemResolution": func(s domain.Slots) domain.Slots {
			r := *s.ItemResolution
			r.Code = testkit.Ptr("BOK_CHOY")
			r.Label = "青江菜"
			s.ItemResolution = &r
			return s
		},
		"quantity":     func(s domain.Slots) domain.Slots { s.Quantity = testkit.Ptr(500); return s },
		"unit":         func(s domain.Slots) domain.Slots { s.Unit = testkit.Ptr("CATTY"); return s },
		"packaging":    func(s domain.Slots) domain.Slots { s.Packaging = testkit.Ptr("網袋"); return s },
		"deliveryCity": func(s domain.Slots) domain.Slots { s.DeliveryCity = testkit.Ptr("臺南市"); return s },
		"deliveryDate": func(s domain.Slots) domain.Slots { s.DeliveryDate = testkit.Ptr("2026-10-01"); return s },
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			got, _ := confirmWithSlots(t, mutate(confirmSlotsFixture("高麗菜", 300, "臺中市")), "confirm-field-"+name)
			if got.PayloadHash == baseResult.PayloadHash {
				t.Errorf("只改 %s 之後 payloadHash 沒變（%q）——這一欄不在 preimage 內", name, got.PayloadHash)
			}
		})
	}
}

// canonical_payload_json 要能單獨編譯出 seller-facing payload（doc 64 §1.2）：
// #2450 的 publication 不該再回頭 join procurement_requests／revision 表，因為
// 那條 join 不受 payload_hash 保護。
func TestConfirmRfqSnapshot_CanonicalPayload_CarriesSellerFacingRequirement(t *testing.T) {
	_, payload := confirmWithSlots(t, confirmSlotsFixture("高麗菜", 300, "臺中市"), "confirm-payload-shape")

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("canonical_payload_json 不是合法 JSON: %v", err)
	}

	for key, want := range map[string]any{
		"item":         "高麗菜",
		"quantity":     float64(300),
		"unit":         "KG",
		"packaging":    "紙箱",
		"deliveryCity": "臺中市",
		"deliveryDate": "2026-09-20",
	} {
		if got := decoded[key]; got != want {
			t.Errorf("payload[%q] = %v, want %v；內容：%s", key, got, want, payload)
		}
	}

	resolution, ok := decoded["itemResolution"].(map[string]any)
	if !ok {
		t.Fatalf("payload 缺 itemResolution，內容：%s", payload)
	}
	if resolution["kind"] != string(domain.ItemResolvedCrop) || resolution["code"] != "CABBAGE_EARLY_AUTUMN" || resolution["label"] != "甘藍-初秋" {
		t.Errorf("itemResolution 身分欄位不符：%v", resolution)
	}
}

// 擴大 preimage 不等於把整包 slots 倒進去：買家私有與非 seller-facing 的欄位
// 一個都不准進來。budget_per_unit 是既有紅線（design spec D2 §V.7.4），
// item_resolution.price_per_kg 是買家自填的品項單價，同一條線上的東西。
func TestConfirmRfqSnapshot_ExpandedPayload_StillExcludesBuyerPrivateSlots(t *testing.T) {
	_, payload := confirmWithSlots(t, confirmSlotsFixture("高麗菜", 300, "臺中市"), "confirm-payload-whitelist")

	for _, forbidden := range []string{
		"budgetPerUnit", "budget_per_unit", "BudgetPerUnit",
		"pricePerKg", "price_per_kg",
		"exactMatchRequired", "exact_match_required",
		"provenance", "Provenance",
		"fallbackGroupCode", "fallback_group_code",
	} {
		if strings.Contains(string(payload), forbidden) {
			t.Errorf("canonical_payload_json 含非 seller-facing 欄位 %q，內容：%s", forbidden, payload)
		}
	}
}

// slots 必須取自「剛剛通過 CAS 的那個 revision」，不是另外再讀一次。再讀一次
// 會開一個 TOCTOU 窗口：兩次讀之間 revision 換版，就會寫出「內容是新版、
// request_revision_id 記舊版」的快照——正是這條線要消滅的那種不一致。
func TestConfirmRfqSnapshot_Payload_ReadsSlotsFromCASedRevisionSnapshot(t *testing.T) {
	casedRevisionSlots := confirmSlotsFixture("高麗菜", 300, "臺中市")
	newerSlots := confirmSlotsFixture("青江菜", 50, "臺南市")

	repo := &fakeConfirmedRfqSnapshotRepo{}
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: confirmPreviewFixture()}
	// slots＝GetSlots 會回的（模擬「已經換版」的較新內容）；snapshotSlots＝
	// GetSnapshot 這一次 CAS 到的 revision 真正帶的內容。
	revisions := testkit.FakeRevisionReader{RevisionID: 1, Slots: newerSlots, SnapshotSlots: &casedRevisionSlots}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, revisions)

	if _, err := uc.ConfirmRfqSnapshot(context.Background(), ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-slots-source",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	}); err != nil {
		t.Fatalf("ConfirmRfqSnapshot() error = %v, want nil", err)
	}

	if !strings.Contains(string(repo.lastPayload), "高麗菜") {
		t.Errorf("payload 沒有取自 CAS 的那個 revision 快照，內容：%s", repo.lastPayload)
	}
}

// R-01 ⑤快照追蹤的「有效至」。docx V.7.5 明列 expiry 是 snapshot 固定的欄位之一，
// 而不是 preview 的屬性——落點因此在 confirmation 回應，不在 price preview DTO。
// 冪等重放也必須帶同一個值：重放走的是既有列的還原路徑，不重算。
func TestConfirmRfqSnapshot_CarriesSnapshotExpiry(t *testing.T) {
	repo := &fakeConfirmedRfqSnapshotRepo{}
	previewReader := fakePricePreviewReader{ownerID: identity.BuyerID(1), preview: pricing.PricePreviewResult{
		PricePreviewID: 1, Version: 1, PayloadHash: "preview-hash",
		RequestID: validPreviewRequestID, RequestRevisionID: validPreviewRevisionID, ExpiresAt: validPreviewExpiresAt(),
	}}
	uc := NewConfirmRfqSnapshotUseCase(repo, previewReader, testkit.FakeRevisionReader{RevisionID: 1})
	in := ConfirmRfqSnapshotInput{
		RequestID: 1, BuyerID: identity.BuyerID(1), ClientRequestID: "confirm-expiry",
		ExpectedRevisionID: 1, PricePreviewID: 1, PricePreviewVersion: 1, PayloadHash: "preview-hash",
	}

	first, err := uc.ConfirmRfqSnapshot(context.Background(), in)
	if err != nil {
		t.Fatalf("ConfirmRfqSnapshot() error = %v, want nil", err)
	}
	if first.ExpiresAt.IsZero() {
		t.Fatalf("ExpiresAt is zero, want the snapshot expiry")
	}

	replay, err := uc.ConfirmRfqSnapshot(context.Background(), in)
	if err != nil {
		t.Fatalf("replay ConfirmRfqSnapshot() error = %v, want nil", err)
	}
	if !replay.ExpiresAt.Equal(first.ExpiresAt) {
		t.Errorf("replay ExpiresAt = %v, want %v", replay.ExpiresAt, first.ExpiresAt)
	}
}

// TestConfirmRfqSnapshot_CanonicalPayloadKeysAreCamelCaseAllTheWayDown 證明鍵名
// 真的落到 canonical_payload_json 這個欄位裡，而不只是型別自己序列化得對。
//
// 這是 #2450 編譯 seller-facing payload 時會讀到的那份 JSON——頂層鍵早就是
// camelCase，巢狀的兩個價格區間卻是 PascalCase。
func TestConfirmRfqSnapshot_CanonicalPayloadKeysAreCamelCaseAllTheWayDown(t *testing.T) {
	_, payload := confirmWithSlots(t, domain.Slots{
		Item: testkit.Ptr("高麗菜"), Quantity: testkit.Ptr(100), Unit: testkit.Ptr("KG"),
		DeliveryCity: testkit.Ptr("臺中市"), DeliveryDate: testkit.Ptr("2026-09-20"),
	}, "confirm-camelcase-keys")

	var got struct {
		BuyerRequestedRange map[string]json.RawMessage `json:"buyerRequestedRange"`
		ReferencePriceRange map[string]json.RawMessage `json:"referencePriceRange"`
	}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("反序列化 canonical payload 失敗：%v，內容：%s", err, payload)
	}

	for _, want := range []string{"currency", "priceComponent", "lowerBoundAmount", "upperBoundAmount", "basisQuantity", "basisUnit"} {
		if _, ok := got.BuyerRequestedRange[want]; !ok {
			t.Errorf("buyerRequestedRange 缺少鍵 %q，實際鍵：%v", want, keysOf(got.BuyerRequestedRange))
		}
	}
	for _, want := range []string{"lower", "midpoint", "upper"} {
		if _, ok := got.ReferencePriceRange[want]; !ok {
			t.Errorf("referencePriceRange 缺少鍵 %q，實際鍵：%v", want, keysOf(got.ReferencePriceRange))
		}
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
