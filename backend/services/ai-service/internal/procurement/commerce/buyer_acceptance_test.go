package commerce

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

type fakeBuyerStore struct {
	versions  []repository.SellerResponseVersionWithIdentity
	shortlist []repository.BuyerShortlistEntry

	identity    repository.SellerResponseIdentity
	identityErr error
	version     repository.SellerResponseVersion
	versionErr  error
	publication repository.Publication
	snapshot    repository.ConfirmedRfqSnapshot

	insertResult repository.BuyerQuoteAcceptance
	insertErr    error
	// byKey.ID 非零＝這把冪等鍵已經接受過（ACK 遺失後重送的形狀）。
	byKey   repository.BuyerQuoteAcceptance
	live    repository.BuyerQuoteAcceptance
	liveErr error

	forUpdateCalls int

	insertCalls []repository.InsertBuyerQuoteAcceptanceInput
	liveCalls   int
	putCalls    []int32

	// accepted／outbox 只記錄「已經 commit 的那些」：fakeBuyerTxRunner 在 rollback 時
	// 把本輪加進來的那幾筆逆序移除，所以斷言長度為 0 真的證得到零寫入——
	// 用 insertCalls 只證得到「呼叫過」，證不到「留下來了」。
	accepted  []repository.BuyerQuoteAcceptance
	outbox    []fakeOutboxRow
	outboxErr error
	tx        *fakeBuyerTxRunner
}

type fakeOutboxRow struct {
	acceptanceID int64
	payload      json.RawMessage
}

func (f *fakeBuyerStore) ListSellerResponseVersionsForRequest(context.Context, identity.BuyerID, int64) ([]repository.SellerResponseVersionWithIdentity, error) {
	return f.versions, nil
}

func (f *fakeBuyerStore) FindSellerResponseByID(context.Context, int64) (repository.SellerResponseIdentity, error) {
	return f.identity, f.identityErr
}

// forUpdateCalls 記錄鎖定讀被呼叫幾次：CAS 讀搬進交易後，identity 必須走這一支，
// 走回 FindSellerResponseByID 就代表鎖沒拿到。
func (f *fakeBuyerStore) FindSellerResponseByIDForUpdate(context.Context, int64) (repository.SellerResponseIdentity, error) {
	f.forUpdateCalls++
	return f.identity, f.identityErr
}

func (f *fakeBuyerStore) FindSellerResponseVersion(context.Context, int64, int32) (repository.SellerResponseVersion, error) {
	return f.version, f.versionErr
}

func (f *fakeBuyerStore) FindPublicationByID(context.Context, int64) (repository.Publication, error) {
	return f.publication, nil
}

func (f *fakeBuyerStore) FindConfirmedRfqSnapshotByIDAndBuyer(context.Context, int64, identity.BuyerID) (repository.ConfirmedRfqSnapshot, error) {
	return f.snapshot, nil
}

func (f *fakeBuyerStore) PutShortlistEntry(_ context.Context, _ identity.BuyerID, _, responseID int64, version int32) (repository.BuyerShortlistEntry, error) {
	f.putCalls = append(f.putCalls, version)
	return repository.BuyerShortlistEntry{SellerResponseID: responseID, SellerResponseVersion: version}, nil
}

func (f *fakeBuyerStore) DeleteShortlistEntry(context.Context, identity.BuyerID, int64, int64) (bool, error) {
	return true, nil
}

func (f *fakeBuyerStore) ListShortlistEntries(context.Context, identity.BuyerID, int64) ([]repository.BuyerShortlistEntry, error) {
	return f.shortlist, nil
}

func (f *fakeBuyerStore) InsertBuyerQuoteAcceptance(_ context.Context, in repository.InsertBuyerQuoteAcceptanceInput) (repository.BuyerQuoteAcceptance, error) {
	f.insertCalls = append(f.insertCalls, in)
	if f.insertErr != nil {
		return repository.BuyerQuoteAcceptance{}, f.insertErr
	}
	if f.insertResult.ID != 0 {
		return f.insertResult, nil
	}
	row := repository.BuyerQuoteAcceptance{
		ID: 700, BuyerID: in.BuyerID, RequestID: in.RequestID,
		SellerResponseID: in.SellerResponseID, SellerResponseVersion: in.SellerResponseVersion,
		AcceptedDealPrice: in.AcceptedDealPrice, Currency: in.Currency,
		State: repository.AcceptanceStateActive, OrderProvisioningState: repository.OrderProvisioningCreating,
		AcceptedAt: in.AcceptedAt,
	}
	f.accepted = append(f.accepted, row)
	if f.tx != nil {
		mark := len(f.accepted) - 1
		f.tx.onRollback(func() { f.accepted = f.accepted[:mark] })
	}
	return row, nil
}

// InsertOrderProvisioningOutbox 模擬 uq_propo_acceptance_event：同一個 acceptance
// 只留一列，重送靜默略過（query 自帶 ON CONFLICT DO NOTHING）。
func (f *fakeBuyerStore) InsertOrderProvisioningOutbox(
	_ context.Context, acceptanceID int64, payload json.RawMessage, _ time.Time,
) error {
	if f.outboxErr != nil {
		return f.outboxErr
	}
	for _, row := range f.outbox {
		if row.acceptanceID == acceptanceID {
			return nil
		}
	}
	f.outbox = append(f.outbox, fakeOutboxRow{acceptanceID: acceptanceID, payload: payload})
	if f.tx != nil {
		mark := len(f.outbox) - 1
		f.tx.onRollback(func() { f.outbox = f.outbox[:mark] })
	}
	return nil
}

func (f *fakeBuyerStore) FindBuyerQuoteAcceptanceByIdempotencyKey(context.Context, identity.BuyerID, string) (repository.BuyerQuoteAcceptance, error) {
	if f.byKey.ID == 0 {
		return repository.BuyerQuoteAcceptance{}, repository.ErrBuyerQuoteAcceptanceNotFound
	}
	return f.byKey, nil
}

func (f *fakeBuyerStore) FindLiveBuyerQuoteAcceptanceForRequest(context.Context, identity.BuyerID, int64) (repository.BuyerQuoteAcceptance, error) {
	f.liveCalls++
	return f.live, f.liveErr
}

func baseBuyerStore() *fakeBuyerStore {
	storeID := int64(77)
	return &fakeBuyerStore{
		identity: repository.SellerResponseIdentity{
			ID: 300, PublicationID: 10, BuyerID: 1234, RequestID: 5, LatestVersion: 2,
			Lane: repository.SellerResponseLanePlatform, StoreID: &storeID,
		},
		version: repository.SellerResponseVersion{
			SellerResponseID: 300, Version: 2, TotalAmount: 30800, GoodsAmount: 30000,
			ShippingAmount: 800, Currency: domain.CurrencyTWD, PayloadHash: "vhash",
			ValidUntil: sellerResponseTestNow.Add(24 * time.Hour),
		},
		publication: repository.Publication{ID: 10, ConfirmedRfqSnapshotID: 20},
		snapshot:    repository.ConfirmedRfqSnapshot{ID: 20, PayloadHash: "snaphash"},
	}
}

// fakeBuyerTxRunner 具備真的 rollback 語意，不是「呼叫 fn 就好」的空殼。
// 沒有它，「outbox 寫入失敗時 acceptance 不會留下」只能靠呼叫次數旁證。
type fakeBuyerTxRunner struct {
	calls int
	undo  []func()
}

func (f *fakeBuyerTxRunner) WithTx(ctx context.Context, fn func(context.Context) error) error {
	f.calls++
	mark := len(f.undo)
	err := fn(ctx)
	if err != nil {
		for i := len(f.undo) - 1; i >= mark; i-- {
			f.undo[i]()
		}
		f.undo = f.undo[:mark]
		return err
	}
	f.undo = f.undo[:mark]
	return nil
}

func (f *fakeBuyerTxRunner) onRollback(undo func()) { f.undo = append(f.undo, undo) }

func newBuyerUC(store *fakeBuyerStore) *BuyerAcceptanceUseCase {
	tx := &fakeBuyerTxRunner{}
	store.tx = tx
	return NewBuyerAcceptanceUseCase(store, tx, nil, sellerResponseNow)
}

func acceptInput() AcceptSellerQuoteInput {
	return AcceptSellerQuoteInput{
		BuyerID: 1234, RequestID: 5, ClientRequestID: "idem-a",
		SellerResponseID: 300, ExpectedSellerResponseVersion: 2,
		ExpectedRfqSnapshotHash: "snaphash",
	}
}

// TestAccept_PriceComesFromTheAcceptedVersion is the core invariant: the client
// sends no amount at all, and acceptedDealPrice is copied from the exact version
// being accepted.
func TestAccept_PriceComesFromTheAcceptedVersion(t *testing.T) {
	store := baseBuyerStore()
	uc := newBuyerUC(store)

	got, err := uc.AcceptSellerQuote(context.Background(), acceptInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Acceptance.AcceptedDealPrice != 30800 {
		t.Fatalf("acceptedDealPrice = %d, want 30800", got.Acceptance.AcceptedDealPrice)
	}
	if got.Replayed {
		t.Fatal("replayed = true on a fresh insert")
	}
	in := store.insertCalls[0]
	if in.SellerResponseVersion != 2 || in.SellerResponsePayloadHash != "vhash" ||
		in.ConfirmedRfqPayloadHash != "snaphash" {
		t.Fatalf("acceptance is not bound to the exact version: %+v", in)
	}
	if got.Acceptance.OrderProvisioningState != repository.OrderProvisioningCreating {
		t.Fatalf("orderProvisioningState = %q, want CREATING_ORDER", got.Acceptance.OrderProvisioningState)
	}
	if got.Acceptance.OrderNo != nil {
		t.Fatal("ACK carried an orderNo; D3 says it must not")
	}
}

func TestAccept_ReplayIsReportedAndNotReinserted(t *testing.T) {
	store := baseBuyerStore()
	store.insertResult = repository.BuyerQuoteAcceptance{
		ID: 700, AcceptedDealPrice: 30800, State: repository.AcceptanceStateActive,
		AcceptedAt: sellerResponseTestNow.Add(-time.Hour),
	}
	uc := newBuyerUC(store)

	got, err := uc.AcceptSellerQuote(context.Background(), acceptInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Replayed {
		t.Fatal("replayed = false, want true")
	}
}

// TestAccept_StaleVersionIsNeverSilentlyUpgraded pins the product rule: a buyer
// holding version 2 while the seller has moved to 3 must be told, not quietly
// switched onto the newest quote.
func TestAccept_StaleVersionIsNeverSilentlyUpgraded(t *testing.T) {
	store := baseBuyerStore()
	store.identity.LatestVersion = 3
	uc := newBuyerUC(store)

	_, err := uc.AcceptSellerQuote(context.Background(), acceptInput())
	if !errors.Is(err, repository.ErrSellerResponseStale) {
		t.Fatalf("err = %v, want ErrSellerResponseStale", err)
	}
	if len(store.insertCalls) != 0 {
		t.Fatal("wrote an acceptance despite the stale CAS")
	}
}

func TestAccept_WithdrawnVersionIsStale(t *testing.T) {
	store := baseBuyerStore()
	withdrawn := int32(2)
	store.identity.WithdrawnVersion = &withdrawn
	uc := newBuyerUC(store)

	_, err := uc.AcceptSellerQuote(context.Background(), acceptInput())
	if !errors.Is(err, repository.ErrSellerResponseStale) {
		t.Fatalf("err = %v, want ErrSellerResponseStale", err)
	}
}

func TestAccept_ExpiredVersionIsRejected(t *testing.T) {
	store := baseBuyerStore()
	store.version.ValidUntil = sellerResponseTestNow.Add(-time.Minute)
	uc := newBuyerUC(store)

	_, err := uc.AcceptSellerQuote(context.Background(), acceptInput())
	if !errors.Is(err, ErrRfqResponseExpired) {
		t.Fatalf("err = %v, want ErrRfqResponseExpired", err)
	}
}

func TestAccept_SnapshotHashMismatchIsStaleSnapshot(t *testing.T) {
	store := baseBuyerStore()
	uc := newBuyerUC(store)
	in := acceptInput()
	in.ExpectedRfqSnapshotHash = "other"

	_, err := uc.AcceptSellerQuote(context.Background(), in)
	if !errors.Is(err, ErrStaleRfqSnapshot) {
		t.Fatalf("err = %v, want ErrStaleRfqSnapshot", err)
	}
}

func TestAccept_OtherBuyersSellerResponseIs404(t *testing.T) {
	store := baseBuyerStore()
	store.identity.BuyerID = 9999
	uc := newBuyerUC(store)

	_, err := uc.AcceptSellerQuote(context.Background(), acceptInput())
	if !errors.Is(err, repository.ErrSellerResponseNotFound) {
		t.Fatalf("err = %v, want ErrSellerResponseNotFound", err)
	}
}

// TestAccept_LiveAcceptanceIsClassifiedByRequery covers §5.6: the second unique
// key (one live acceptance per request) surfaces as a named sentinel, and the
// caller must re-query to tell "go pay for the one you already accepted" from
// "this RFQ is genuinely locked".
func TestAccept_LiveAcceptanceIsClassifiedByRequery(t *testing.T) {
	for _, tc := range []struct {
		name    string
		state   string
		wantErr error
	}{
		{"active acceptance sends the buyer to checkout", repository.AcceptanceStateActive, ErrRfqCheckoutAlreadyActive},
		{"payment-finalized award is the hard block", repository.AcceptanceStatePaymentFinalized, ErrRfqAlreadyAwarded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := baseBuyerStore()
			store.insertErr = repository.ErrRequestAlreadyHasLiveAcceptance
			store.live = repository.BuyerQuoteAcceptance{ID: 800, State: tc.state}
			uc := newBuyerUC(store)

			_, err := uc.AcceptSellerQuote(context.Background(), acceptInput())
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if store.liveCalls != 1 {
				t.Fatalf("liveCalls = %d, want exactly one re-query", store.liveCalls)
			}
		})
	}
}

// TestAccept_ReleasedAcceptanceIsRetryable: if the live row is gone by the time
// we re-query, the block is not permanent — never report ALREADY_AWARDED.
func TestAccept_ReleasedAcceptanceIsRetryable(t *testing.T) {
	store := baseBuyerStore()
	store.insertErr = repository.ErrRequestAlreadyHasLiveAcceptance
	store.liveErr = repository.ErrBuyerQuoteAcceptanceNotFound
	uc := newBuyerUC(store)

	_, err := uc.AcceptSellerQuote(context.Background(), acceptInput())
	if !errors.Is(err, ErrRfqCheckoutAlreadyActive) {
		t.Fatalf("err = %v, want ErrRfqCheckoutAlreadyActive", err)
	}
}

func TestAccept_MissingIdempotencyKeyIsRejected(t *testing.T) {
	uc := newBuyerUC(baseBuyerStore())
	in := acceptInput()
	in.ClientRequestID = ""

	_, err := uc.AcceptSellerQuote(context.Background(), in)
	if !errors.Is(err, ErrIdempotencyKeyRequired) {
		t.Fatalf("err = %v, want ErrIdempotencyKeyRequired", err)
	}
}

// TestListSellerResponses_AcceptabilityIsTyped proves superseded/withdrawn/
// expired versions stay in the list (audit) with a typed reason rather than
// being filtered out or left for the client to derive.
func TestListSellerResponses_AcceptabilityIsTyped(t *testing.T) {
	storeID := int64(77)
	withdrawn := int32(9)
	id := func(latest int32, wv *int32, closed bool) repository.SellerResponseIdentity {
		out := repository.SellerResponseIdentity{
			ID: 300, BuyerID: 1234, RequestID: 5, LatestVersion: latest,
			Lane: repository.SellerResponseLanePlatform, StoreID: &storeID,
			WithdrawnVersion: wv,
		}
		if closed {
			at := sellerResponseTestNow
			out.WriteClosedAt = &at
		}
		return out
	}
	ver := func(v int32, validUntil time.Time) repository.SellerResponseVersion {
		return repository.SellerResponseVersion{SellerResponseID: 300, Version: v, ValidUntil: validUntil}
	}
	future := sellerResponseTestNow.Add(time.Hour)
	past := sellerResponseTestNow.Add(-time.Hour)

	store := baseBuyerStore()
	store.versions = []repository.SellerResponseVersionWithIdentity{
		{Identity: id(2, nil, false), Version: ver(2, future)},        // ACTIVE
		{Identity: id(2, nil, false), Version: ver(1, future)},        // SUPERSEDED
		{Identity: id(3, nil, false), Version: ver(3, past)},          // EXPIRED
		{Identity: id(9, &withdrawn, false), Version: ver(9, future)}, // WITHDRAWN
		{Identity: id(4, nil, true), Version: ver(4, future)},         // INELIGIBLE (declined)
	}
	store.shortlist = []repository.BuyerShortlistEntry{{SellerResponseID: 300, SellerResponseVersion: 2}}
	uc := newBuyerUC(store)

	got, err := uc.ListSellerResponses(context.Background(), 1234, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		domain.QuoteAcceptabilityAcceptable,
		domain.QuoteAcceptabilitySuperseded,
		domain.QuoteAcceptabilityExpired,
		domain.QuoteAcceptabilityWithdrawn,
		domain.QuoteAcceptabilityIneligible,
	}
	if len(got) != len(want) {
		t.Fatalf("rows = %d, want %d — superseded/withdrawn versions must stay in audit", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Acceptability != w {
			t.Fatalf("row %d acceptability = %q, want %q", i, got[i].Acceptability, w)
		}
	}
	if !got[0].Shortlisted || got[1].Shortlisted {
		t.Fatalf("shortlist is keyed by (responseId, version): got %v/%v", got[0].Shortlisted, got[1].Shortlisted)
	}
	if got[0].Seller.PartyType != PartyTypePlatformStore {
		t.Fatalf("partyType = %q", got[0].Seller.PartyType)
	}
}

func TestPutShortlist_RejectsAnotherBuyersResponse(t *testing.T) {
	store := baseBuyerStore()
	store.identity.BuyerID = 9999
	uc := newBuyerUC(store)

	_, err := uc.PutShortlist(context.Background(), 1234, 5, 300, 2)
	if !errors.Is(err, repository.ErrSellerResponseNotFound) {
		t.Fatalf("err = %v, want ErrSellerResponseNotFound", err)
	}
	if len(store.putCalls) != 0 {
		t.Fatal("wrote a shortlist entry for another buyer's response")
	}
}

// TestAccept_LostAckRetryReplaysEvenAfterSellerRevised：買家的第一次接受成功了，
// ACK 在路上掉了；重送前賣家又改了版。先跑 CAS 會回 STALE_SELLER_RESPONSE，但那張
// RFQ 其實已經 award——回放要回原本那筆 ACK，不是叫買家回去重看報價。
func TestAccept_LostAckRetryReplaysEvenAfterSellerRevised(t *testing.T) {
	store := baseBuyerStore()
	store.identity.LatestVersion = 3 // 賣家在兩次重試之間改到第 3 版
	// 既有列必須是真實可能存在的一列：request_id 與 confirmed_rfq_payload_hash 在
	// schema 都是 NOT NULL（db/schema/procurement.sql），留 0／空字串的 fixture 會讓
	// 「同鍵同內容」的比對失去對象。
	prior := acceptedRow()
	prior.OrderProvisioningState = repository.OrderProvisioningCreating
	store.byKey = prior
	uc := newBuyerUC(store)

	got, err := uc.AcceptSellerQuote(context.Background(), acceptInput())
	if err != nil {
		t.Fatalf("重送同一把冪等鍵應回放，卻回錯誤：%v", err)
	}
	if !got.Replayed || got.Acceptance.ID != 700 {
		t.Fatalf("replayed=%v id=%d，want true/700", got.Replayed, got.Acceptance.ID)
	}
	if len(store.insertCalls) != 0 {
		t.Fatal("回放不得再寫入")
	}
}

// TestAccept_OrderCommandIsWrittenInTheSameTransaction pins doc 64 §6.2 選項 B：
// acceptance 與建單命令原子落庫。§2-4 真正要防的是「接受成功卻永遠沒有訂單」，
// 而那個保證的第一半就是這一條。
func TestAccept_OrderCommandIsWrittenInTheSameTransaction(t *testing.T) {
	store := baseBuyerStore()
	uc := newBuyerUC(store)

	got, err := uc.AcceptSellerQuote(context.Background(), acceptInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.tx.calls != 1 {
		t.Fatalf("WithTx calls = %d, want 1", store.tx.calls)
	}
	if len(store.outbox) != 1 {
		t.Fatalf("outbox rows = %d, want 1", len(store.outbox))
	}
	if store.outbox[0].acceptanceID != got.Acceptance.ID {
		t.Fatalf("outbox is bound to acceptance %d, want %d",
			store.outbox[0].acceptanceID, got.Acceptance.ID)
	}

	var payload struct {
		SchemaVersion int    `json:"schemaVersion"`
		AcceptanceID  string `json:"acceptanceId"`
		BuyerID       string `json:"buyerId"`
		RequestID     string `json:"requestId"`
	}
	if err := json.Unmarshal(store.outbox[0].payload, &payload); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	if payload.SchemaVersion != 1 || payload.AcceptanceID != "700" ||
		payload.BuyerID != "1234" || payload.RequestID != "5" {
		t.Fatalf("payload = %+v", payload)
	}
	// ⛔ 命令只帶識別。金額出現在 payload 裡就是第二份會漂移的真相——
	// acceptance 那一列才是 acceptedDealPrice 的唯一來源。
	if bytes.Contains(store.outbox[0].payload, []byte("30800")) {
		t.Fatalf("payload carries an amount: %s", store.outbox[0].payload)
	}
}

// TestAccept_OrderCommandFailureRollsBackTheAcceptance 證的是 atomicity 的另一半：
// 建單命令寫不進去時，acceptance **不得**留下。少了這一條，買家會拿到一個成功的
// ACK、RFQ 被 award 掉，而沒有任何東西會去建那張訂單。
func TestAccept_OrderCommandFailureRollsBackTheAcceptance(t *testing.T) {
	store := baseBuyerStore()
	store.outboxErr = errors.New("boom")
	uc := newBuyerUC(store)

	if _, err := uc.AcceptSellerQuote(context.Background(), acceptInput()); err == nil {
		t.Fatal("expected an error when the order command cannot be written")
	}
	if len(store.accepted) != 0 {
		t.Fatalf("acceptance survived a failed order command: %+v", store.accepted)
	}
	if len(store.outbox) != 0 {
		t.Fatalf("outbox rows = %d, want 0", len(store.outbox))
	}
}

// TestAccept_ConcurrentInsertDoesNotCreateASecondOrderCommand 走的是「冪等回查沒命中、
// 但 insert 撞上冪等鍵回既有列」那條路（兩個請求同時到）。uq_propo_acceptance_event
// ＋ ON CONFLICT DO NOTHING 必須讓第二次不產生第二筆建單命令——否則同一筆成交會建出
// 兩張訂單。
func TestAccept_ConcurrentInsertDoesNotCreateASecondOrderCommand(t *testing.T) {
	store := baseBuyerStore()
	uc := newBuyerUC(store)

	if _, err := uc.AcceptSellerQuote(context.Background(), acceptInput()); err != nil {
		t.Fatalf("first accept failed: %v", err)
	}
	if _, err := uc.AcceptSellerQuote(context.Background(), acceptInput()); err != nil {
		t.Fatalf("second accept failed: %v", err)
	}
	if len(store.outbox) != 1 {
		t.Fatalf("outbox rows = %d, want 1", len(store.outbox))
	}
}

// acceptedRow 是「acceptInput() 那份意圖已經成功接受過」的既有列，用來驅動回放路徑。
// 每個欄位都取不同值，避免某個比較寫錯時剛好因為兩邊都是同一個數字而看起來正確。
func acceptedRow() repository.BuyerQuoteAcceptance {
	return repository.BuyerQuoteAcceptance{
		ID: 700, BuyerID: 1234, RequestID: 5, PublicationID: 10,
		SellerResponseID: 300, SellerResponseVersion: 2,
		SellerResponsePayloadHash: "vhash",
		ConfirmedRfqSnapshotID:    20, ConfirmedRfqPayloadHash: "snaphash",
		AcceptedDealPrice: 30800, Currency: domain.CurrencyTWD,
		State: repository.AcceptanceStateActive, ClientRequestID: "idem-a",
		RequestHash: "vhash",
	}
}

// TestAccept_MissingRfqSnapshotHashIsRejected：doc 64 §3.2 要求 acceptance 同時 CAS
// seller response version 與 RFQ snapshot hash，「少任何一個都有一條真實的錯配路徑」。
// 空字串目前會讓整條 snapshot CAS 被短路跳過，而不是被擋下來。
func TestAccept_MissingRfqSnapshotHashIsRejected(t *testing.T) {
	store := baseBuyerStore()
	uc := newBuyerUC(store)
	in := acceptInput()
	in.ExpectedRfqSnapshotHash = ""

	_, err := uc.AcceptSellerQuote(context.Background(), in)
	if !errors.Is(err, ErrRfqSnapshotHashRequired) {
		t.Fatalf("err = %v, want ErrRfqSnapshotHashRequired", err)
	}
	if len(store.accepted) != 0 {
		t.Fatalf("acceptance committed without a snapshot CAS token: %+v", store.accepted)
	}
	if len(store.outbox) != 0 {
		t.Fatalf("outbox rows = %d, want 0", len(store.outbox))
	}
}

// TestAccept_ReplayRequiresIdenticalIntent：doc 64 §9.2「同一 idempotency key 但輸入
// 不一致 ⇒ 零 mutation，**不得靜默回放上一次的結果**」。每個意圖欄位單獨變異一次，
// 一起變的話一個欄位比對寫對就能掩蓋其餘三個。
func TestAccept_ReplayRequiresIdenticalIntent(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*AcceptSellerQuoteInput)
	}{
		{"different request", func(in *AcceptSellerQuoteInput) { in.RequestID = 6 }},
		{"different seller response", func(in *AcceptSellerQuoteInput) { in.SellerResponseID = 301 }},
		{"different seller version", func(in *AcceptSellerQuoteInput) { in.ExpectedSellerResponseVersion = 3 }},
		{"different rfq snapshot", func(in *AcceptSellerQuoteInput) { in.ExpectedRfqSnapshotHash = "othersnap" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := baseBuyerStore()
			store.byKey = acceptedRow()
			uc := newBuyerUC(store)
			in := acceptInput()
			tc.mutate(&in)

			got, err := uc.AcceptSellerQuote(context.Background(), in)
			if !errors.Is(err, repository.ErrBuyerQuoteAcceptanceIdempotencyConflict) {
				t.Fatalf("err = %v (replayed=%v, acceptance=%d), want ErrBuyerQuoteAcceptanceIdempotencyConflict",
					err, got.Replayed, got.Acceptance.ID)
			}
			if len(store.accepted) != 0 || len(store.outbox) != 0 {
				t.Fatalf("conflicting replay mutated state: accepted=%d outbox=%d",
					len(store.accepted), len(store.outbox))
			}
		})
	}
}

// TestAccept_ReplayWithIdenticalIntentReturnsOriginal 是上一條的正控制：同鍵同內容仍
// 必須回放，而且不得因為賣家事後改版就改回 STALE——第一次其實已經 award 了。
func TestAccept_ReplayWithIdenticalIntentReturnsOriginal(t *testing.T) {
	store := baseBuyerStore()
	store.byKey = acceptedRow()
	// 賣家在兩次重試之間改了版：回放路徑不得因此翻臉。
	store.identity.LatestVersion = 3
	uc := newBuyerUC(store)

	got, err := uc.AcceptSellerQuote(context.Background(), acceptInput())
	if err != nil {
		t.Fatalf("identical replay failed: %v", err)
	}
	if !got.Replayed || got.Acceptance.ID != 700 {
		t.Fatalf("replayed=%v acceptance=%d, want true/700", got.Replayed, got.Acceptance.ID)
	}
	if len(store.accepted) != 0 || len(store.outbox) != 0 {
		t.Fatalf("replay wrote new rows: accepted=%d outbox=%d", len(store.accepted), len(store.outbox))
	}
}
