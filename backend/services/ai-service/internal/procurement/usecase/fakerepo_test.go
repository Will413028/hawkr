package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// ptr is a generic pointer-of helper shared by every test file in this
// package (moved here from the deleted turn_test.go — Task 6a of the
// procurement-graph-cutover plan — since it has no single natural "owner"
// file: it is used across streak/rewind/answer/crop/extract test files that
// are unrelated to each other).
func ptr[T any](v T) *T { return &v }

// fakeExtractor is the usecase.SlotExtractor test double shared across the
// package (moved here from the deleted turn_test.go alongside ptr — see its
// doc comment above for why).
type fakeExtractor struct {
	patch domain.SlotPatch
	err   error
	// attempt/maxAttempts default to zero for every existing caller (none
	// of them assert on ExtractResult.Attempt/MaxAttempts — turn.go's
	// legacy path only reads .Patch, see turn.go's Extract call site).
	// graph_test.go's Activity-event test is the first caller that needs
	// a realistic (non-zero) Attempt, matching what the real
	// retryingExtractor wrapper always sets in production
	// (extractor_provider.go).
	attempt     int
	maxAttempts int
	// shortIntent defaults to "" for every existing caller (none of them
	// assert on ExtractResult.ShortIntent). graph_test.go's Activity-event
	// test is the first caller that sets a realistic sentence, to prove
	// the emitted event's Text actually carries the model's ShortIntent
	// through guardShortIntent, not just a hardcoded StepLabels fallback.
	shortIntent string
}

func (f fakeExtractor) Extract(_ context.Context, _ string, _ domain.Slots) (ExtractResult, error) {
	return ExtractResult{Patch: f.patch, ShortIntent: f.shortIntent, Attempt: f.attempt, MaxAttempts: f.maxAttempts}, f.err
}

// readyTurnPatch is "all six required-adjacent slots filled" — moved here
// from the deleted turn_test.go since serializer_golden_test.go (the graph
// port of TestTurnAnswerRewind_SameSerializerShape, Task 6a) still needs it.
func readyTurnPatch() domain.SlotPatch {
	deliveryDate := time.Now().UTC().AddDate(0, 0, 14).Format(domain.DateLayout)
	return domain.SlotPatch{
		Item: ptr("寶島梨禮盒"), Quantity: ptr(50), Unit: ptr("KG"),
		BudgetPerUnit: ptr(800), DeliveryCity: ptr("桃園市"), DeliveryDate: ptr(deliveryDate),
	}
}

// fakePricePreviewChecker is the usecase.PricePreviewChecker test double —
// moved here from the deleted turn_test.go; still used by
// graph_terminal_test.go/rewind_test.go/answer_test.go/serializer_golden_test.go.
type fakePricePreviewChecker struct {
	hasPreview bool
	err        error
}

func (f fakePricePreviewChecker) HasPricePreview(_ context.Context, _ int64) (bool, error) {
	return f.hasPreview, f.err
}

// fakeEntitlementClient is the usecase.EntitlementClient test double —
// moved here from the deleted turn_test.go; still used by
// graph_dispatch_test.go/graph_dispatch_integration_test.go. calls lets
// tests assert an idempotent replay path never queries buyer-service.
type fakeEntitlementClient struct {
	entitlement Entitlement
	err         error
	calls       int
}

func (f *fakeEntitlementClient) GetBuyerEntitlement(_ context.Context, _ identity.BuyerID) (Entitlement, error) {
	f.calls++
	return f.entitlement, f.err
}

// fakeQuotaReserver is the usecase.QuotaReserver test double — moved here
// from the deleted turn_test.go; still used by graph_dispatch_test.go.
// Deliberately records call counts rather than a bare bool: distinguishing
// "double-reserved" from "leaked a release" needs counts, not a flag.
type fakeQuotaReserver struct {
	reservationID   int64
	reserved        bool
	confirmedBefore int
	err             error

	reserveCalls          int
	lastPeriodStart       time.Time
	lastLimit             int
	releaseCalls          int
	releasedReservationID int64
}

func (f *fakeQuotaReserver) ReserveQuota(_ context.Context, _ identity.BuyerID, periodStart time.Time, limit int) (int64, bool, int, error) {
	f.reserveCalls++
	f.lastPeriodStart, f.lastLimit = periodStart, limit
	return f.reservationID, f.reserved, f.confirmedBefore, f.err
}

func (f *fakeQuotaReserver) ReleaseQuota(_ context.Context, reservationID int64, _ identity.BuyerID) {
	f.releaseCalls++
	f.releasedReservationID = reservationID
}

// failingAppendRepo makes AppendRevision fail unconditionally, embedding
// (not overriding) the rest of fakeRepo's real behavior — moved here from
// the deleted turn_test.go; still used by graph_dispatch_test.go to verify
// a reserved-but-unpersisted quota gets released.
type failingAppendRepo struct {
	*fakeRepo
	err error
}

func (f *failingAppendRepo) AppendRevision(_ context.Context, _ repository.AppendInput) (repository.Revision, error) {
	return repository.Revision{}, f.err
}

// recordingAppendRepo keeps the last AppendInput it saw — moved here from
// the deleted turn_test.go; still used by graph_dispatch_test.go to assert
// a quota reservation actually reaches repository.AppendInput.
type recordingAppendRepo struct {
	*fakeRepo
	lastAppend repository.AppendInput
}

func (r *recordingAppendRepo) AppendRevision(ctx context.Context, in repository.AppendInput) (repository.Revision, error) {
	r.lastAppend = in
	return r.fakeRepo.AppendRevision(ctx, in)
}

// testEntitlement is a stock Entitlement fixture — moved here from the
// deleted turn_test.go; still used by graph_dispatch_test.go/
// graph_dispatch_integration_test.go.
func testEntitlement() Entitlement {
	periodStart := time.Now().UTC().AddDate(0, 0, -3).Truncate(time.Second)
	return Entitlement{
		PlanCode: "PRO", MonthlyAnswerLimit: 100,
		PeriodStart: periodStart, PeriodEnd: periodStart.AddDate(0, 1, 0),
	}
}

// fakePatchKey 是冪等鍵，與 DB 唯一索引 (buyer_id, client_request_id) 同一組欄位。
// 替身若只用 client_request_id 當 key，就模擬不出「跨買家同一個 key 互不相干」。
type fakePatchKey struct {
	buyerID         identity.BuyerID
	clientRequestID string
}

// fakeRepo 是 RevisionStore 的記憶體實作，讓 turn 的編排邏輯可以不碰 DB 單測。
type fakeRepo struct {
	nextID    int64
	requests  map[int64]identity.BuyerID // requestID -> buyerID
	slots     map[int64]domain.Slots
	revisions map[int64]int64 // requestID -> revisionID
	patches   map[fakePatchKey]repository.IdempotentResult
	statuses  map[int64]string // requestID -> status，SetRequestStatus 斷言用
	// history 是每個 request 的線性版本鏈（root-first、tip-last，比照真正
	// repository.ListBranchRevisions 的順序保證），CreateRequest／AppendRevision／
	// RewindTo 各自往同一個 slice 追加——Task 6 加入：resolveRewindTarget 要在
	// 單元測試裡真的找得到「slotKey 最後一次被寫入之前的版本」，沒有真實鏈就無從
	// 測起（先前版本這個方法永遠回傳空鏈，turn/answer 既有測試不受影響的理由見
	// ListBranchRevisions 下方註解）。仍不模擬分支/supersede 這類只有整合測試
	// （Task 4 的 repository/rewind_test.go）才驗證得到的持久層細節——這裡永遠是
	// 一條沒有分岔的序列。
	history map[int64][]repository.RevisionWithPatch

	// conversations mirrors the real repository's "CreateRequest always
	// creates the conversation row in the same transaction" guarantee
	// (revision.go) — assigned once per requestID at CreateRequest time,
	// surfaced on every Revision this fake returns afterwards (both
	// CreateRequest's own and AppendRevision's). Task 15 write-path tests
	// (TestExecuteTurnPipeline_NewConversation_BackfillsRunConversationID)
	// need a real, non-zero ConversationID to exercise
	// executeTurnPipeline's backfill call — existing tests never assert on
	// this field, so giving it a real value is purely additive fidelity.
	conversations map[int64]int64 // requestID -> conversationID

	createRequestCalls int // 冪等重播不得再建 request，斷言用
	setStatusCalls     int // 獨立 SetRequestStatus 方法被呼叫次數；turn.go 正常流程
	// 現在把狀態併進 AppendRevision 同一次呼叫，不該再獨立呼叫這個方法——
	// 這個計數器就是用來在測試裡抓「有沒有偷偷退回舊的兩段式寫法」的迴歸哨兵。
	// 只看 f.statuses 的最終內容分辨不出兩種寫法：新舊路徑都會把同一個值寫進
	// 同一個 map，內容看起來一模一樣。
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		nextID:        1,
		requests:      map[int64]identity.BuyerID{},
		slots:         map[int64]domain.Slots{},
		revisions:     map[int64]int64{},
		patches:       map[fakePatchKey]repository.IdempotentResult{},
		statuses:      map[int64]string{},
		history:       map[int64][]repository.RevisionWithPatch{},
		conversations: map[int64]int64{},
	}
}

func (f *fakeRepo) id() int64 { f.nextID++; return f.nextID }

func (f *fakeRepo) FindByIdempotencyKey(_ context.Context, buyerID identity.BuyerID, clientRequestID string) (repository.IdempotentResult, error) {
	got, ok := f.patches[fakePatchKey{buyerID, clientRequestID}]
	if !ok {
		return repository.IdempotentResult{}, repository.ErrNotFound
	}
	return got, nil
}

func (f *fakeRepo) CreateRequest(_ context.Context, buyerID identity.BuyerID) (repository.Request, repository.Revision, error) {
	f.createRequestCalls++
	reqID, revID, convID := f.id(), f.id(), f.id()
	f.requests[reqID] = buyerID
	f.slots[reqID] = domain.Slots{}
	f.revisions[reqID] = revID
	f.conversations[reqID] = convID
	rootSlotsJSON, _ := json.Marshal(domain.Slots{})
	f.history[reqID] = []repository.RevisionWithPatch{
		{Revision: repository.Revision{ID: revID, RequestID: reqID, RevisionNo: 1, BranchID: revID, Status: "ACTIVE", SlotsJSON: rootSlotsJSON, ConversationID: convID}},
	}
	return repository.Request{ID: reqID, BuyerID: buyerID, ActiveRevisionID: revID},
		repository.Revision{ID: revID, RequestID: reqID, RevisionNo: 1, BranchID: revID, Status: "ACTIVE", ConversationID: convID}, nil
}

// AppendRevision 的假實作把 in.RequestStatus 一併寫進 f.statuses，模擬真實
// repository 把狀態寫進同一個交易的行為（見 revision.go AppendRevision 註解：
// 狀態寫入不再是呼叫端事後另一次呼叫，而是這次呼叫本身的一部分）。同一 call 內的
// 冪等命中分支（同 client_request_id 重送）刻意不覆寫狀態——真正的 repository
// 在那個分支也是直接回傳既有 revision、不重新執行任何寫入。
func (f *fakeRepo) AppendRevision(_ context.Context, in repository.AppendInput) (repository.Revision, error) {
	key := fakePatchKey{in.BuyerID, in.ClientRequestID}
	if prev, ok := f.patches[key]; ok {
		return repository.Revision{ID: prev.RevisionID, RequestID: prev.RequestID, Status: "ACTIVE", ConversationID: f.conversations[prev.RequestID]}, nil
	}
	if owner, ok := f.requests[in.RequestID]; !ok || owner != in.BuyerID {
		return repository.Revision{}, repository.ErrNotFound
	}
	revID := f.id()
	f.slots[in.RequestID] = in.Slots
	f.revisions[in.RequestID] = revID
	f.statuses[in.RequestID] = in.RequestStatus
	slotsJSON, _ := json.Marshal(in.Slots)
	f.history[in.RequestID] = append(f.history[in.RequestID], repository.RevisionWithPatch{
		Revision:         repository.Revision{ID: revID, RequestID: in.RequestID, Status: "ACTIVE", SlotsJSON: slotsJSON, ConversationID: f.conversations[in.RequestID]},
		PatchSource:      in.Source,
		PatchPayloadJSON: in.PayloadJSON,
	})
	f.patches[key] = repository.IdempotentResult{
		RequestID: in.RequestID, RevisionID: revID, Slots: in.Slots, PayloadJSON: in.PayloadJSON,
		ConversationID: f.conversations[in.RequestID],
	}
	return repository.Revision{ID: revID, RequestID: in.RequestID, Status: "ACTIVE", ConversationID: f.conversations[in.RequestID]}, nil
}

func (f *fakeRepo) GetSlots(_ context.Context, requestID int64, buyerID identity.BuyerID) (domain.Slots, error) {
	if owner, ok := f.requests[requestID]; !ok || owner != buyerID {
		return domain.Slots{}, repository.ErrNotFound
	}
	return f.slots[requestID], nil
}

func (f *fakeRepo) SetRequestStatus(_ context.Context, requestID int64, status string) error {
	f.setStatusCalls++
	f.statuses[requestID] = status
	return nil
}

// GetSnapshot 讓 fakeRepo 額外滿足 usecase.AnswerStore（Task 5）——回傳「目前」
// 狀態當作 active revision，跟本檔其餘方法一樣不模擬真正的版本鏈（見
// ListBranchRevisions 的既有註解）。
func (f *fakeRepo) GetSnapshot(_ context.Context, requestID int64, buyerID identity.BuyerID) (repository.Request, repository.Revision, error) {
	owner, ok := f.requests[requestID]
	if !ok || owner != buyerID {
		return repository.Request{}, repository.Revision{}, repository.ErrNotFound
	}
	slots := f.slots[requestID]
	slotsJSON, _ := json.Marshal(slots)
	revID := f.revisions[requestID]
	return repository.Request{
			ID: requestID, BuyerID: buyerID, ActiveRevisionID: revID,
		},
		repository.Revision{ID: revID, RequestID: requestID, SlotsJSON: slotsJSON, Status: "ACTIVE"}, nil
}

// ListBranchRevisions 的假實作：回傳 f.history 追蹤的真實線性鏈。turn_test.go／
// answer_test.go 既有案例不受影響——ListBranchRevisions 在 Handle() 內一律發生在
// 「這一輪」自己的 AppendRevision 之前，讀到的鏈本來就不含這一輪；對於單輪、
// 全新對話的情境（既有測試的絕大多數），鏈裡只有一個沒有 patch 的 root 版本，
// deriveQuestionHistory 對它的行為跟舊版「永遠回傳空鏈」一樣是 skipped=空、
// streak=0。若某個測試只需要驗證 skip／連續上限的衍生邏輯本身，可以繼續改寫在
// usecase/question_history_test.go 用手造的 []repository.RevisionWithPatch 直接測
// deriveQuestionHistory，不必透過 fakeRepo。
func (f *fakeRepo) ListBranchRevisions(_ context.Context, requestID int64, buyerID identity.BuyerID) ([]repository.RevisionWithPatch, error) {
	if owner, ok := f.requests[requestID]; !ok || owner != buyerID {
		return nil, repository.ErrNotFound
	}
	return f.history[requestID], nil
}

// RewindTo 的假實作：f.history 是真正的線性鏈，因此可以比照真正的
// repository.RewindTo，從鏈上找出 target revision 的 slots 內容，把它當成 rewind
// 之後的「目前」狀態原子性地反映到 f.slots／f.revisions／f.history／f.patches
// ——不能只挪 f.revisions、留著 f.slots／f.history 是 rewind 前的舊值，否則接在
// rewind 後面的任何一次讀取（GetSnapshot／下一輪 Handle／FindByIdempotencyKey 重播）
// 都會看到跟這次回應內容矛盾的「目前」狀態。擁有權檢查比照 AppendRevision（用
// owner != in.BuyerID，不是只檢查 request 存在），理由相同：不能讓別的買家的
// rewind 打中這個 request。分支/supersede 這類只有整合測試（Task 4 的
// repository/rewind_test.go）才驗證得到的持久層細節仍不模擬。
func (f *fakeRepo) RewindTo(_ context.Context, in repository.RewindInput) (repository.Revision, error) {
	key := fakePatchKey{in.BuyerID, in.ClientRequestID}
	if prev, ok := f.patches[key]; ok {
		return repository.Revision{ID: prev.RevisionID, RequestID: prev.RequestID, Status: "ACTIVE", ConversationID: f.conversations[prev.RequestID]}, nil
	}
	if owner, ok := f.requests[in.RequestID]; !ok || owner != in.BuyerID {
		return repository.Revision{}, repository.ErrNotFound
	}

	var targetSlots domain.Slots
	found := false
	for _, h := range f.history[in.RequestID] {
		if h.ID == in.TargetRevisionID {
			if err := json.Unmarshal(h.SlotsJSON, &targetSlots); err != nil {
				return repository.Revision{}, err
			}
			found = true
			break
		}
	}
	if !found {
		return repository.Revision{}, repository.ErrNotFound
	}

	revID := f.id()
	f.slots[in.RequestID] = targetSlots
	f.revisions[in.RequestID] = revID
	slotsJSON, _ := json.Marshal(targetSlots)
	f.history[in.RequestID] = append(f.history[in.RequestID], repository.RevisionWithPatch{
		Revision:         repository.Revision{ID: revID, RequestID: in.RequestID, Status: "ACTIVE", SlotsJSON: slotsJSON},
		PatchSource:      "REWIND",
		PatchPayloadJSON: in.PayloadJSON,
	})
	f.patches[key] = repository.IdempotentResult{
		RequestID: in.RequestID, RevisionID: revID, Slots: targetSlots, PayloadJSON: in.PayloadJSON,
		ConversationID: f.conversations[in.RequestID],
	}
	return repository.Revision{ID: revID, RequestID: in.RequestID, Status: "ACTIVE", ConversationID: f.conversations[in.RequestID]}, nil
}
