package commerce

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/pricing"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

var sellerResponseTestNow = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

func sellerResponseNow() time.Time { return sellerResponseTestNow }

// passthroughTx runs fn inline: these usecases' transaction boundary is about
// atomicity in production, not about anything the fakes model.
type passthroughTx struct{}

func (passthroughTx) WithTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type fakeSellerStore struct {
	publication repository.Publication
	invite      repository.PublicationInvite
	inviteErr   error

	identity     repository.SellerResponseIdentity
	identityErr  error
	draft        repository.SellerResponseDraft
	draftErr     error
	insertResult repository.SellerResponseVersion
	insertErr    error
	bumpErr      error
	withdrawErr  error
	closeErr     error
	snapshot     repository.ConfirmedRfqSnapshot
	snapshotErr  error
	preview      repository.PricePreview
	previewErr   error

	version    repository.SellerResponseVersion
	versionErr error

	sellerView    repository.InviteSellerView
	sellerViewErr error

	// replayVersion 非零版本＝這把冪等鍵已經寫過一版（ACK 遺失後重送的形狀）。
	replayVersion repository.SellerResponseVersion

	insertCalls              []repository.InsertSellerResponseVersionInput
	commands                 []repository.SellerResponseCommand
	bumpCalls                []int32
	findPublicationByIDCalls []int64
	publicationsForStore     []repository.PublicationForStore
}

func (f *fakeSellerStore) FindPublicationByID(_ context.Context, id int64) (repository.Publication, error) {
	f.findPublicationByIDCalls = append(f.findPublicationByIDCalls, id)
	return f.publication, nil
}

func (f *fakeSellerStore) FindPublicationInviteByID(context.Context, int64) (repository.PublicationInvite, error) {
	return f.invite, f.inviteErr
}

func (f *fakeSellerStore) FindPublicationInviteForStore(context.Context, int64, int64) (repository.PublicationInvite, error) {
	return f.invite, f.inviteErr
}

func (f *fakeSellerStore) ListPublicationsForStore(context.Context, int64, int32) ([]repository.PublicationForStore, error) {
	if f.publicationsForStore != nil {
		return f.publicationsForStore, nil
	}
	return []repository.PublicationForStore{{Publication: f.publication, Invite: f.invite}}, nil
}

func (f *fakeSellerStore) FindConfirmedRfqSnapshotByIDAndBuyer(context.Context, int64, identity.BuyerID) (repository.ConfirmedRfqSnapshot, error) {
	return f.snapshot, f.snapshotErr
}

func (f *fakeSellerStore) FindPricePreviewByIDAndBuyer(context.Context, int64, identity.BuyerID) (repository.PricePreview, error) {
	return f.preview, f.previewErr
}

func (f *fakeSellerStore) EnsureSellerResponseIdentityForStore(context.Context, int64, identity.BuyerID, int64, int64) (repository.SellerResponseIdentity, error) {
	return f.identity, f.identityErr
}

func (f *fakeSellerStore) EnsureSellerResponseIdentityForProspect(context.Context, int64, identity.BuyerID, int64, int64) (repository.SellerResponseIdentity, error) {
	return f.identity, f.identityErr
}

func (f *fakeSellerStore) FindSellerResponseByID(context.Context, int64) (repository.SellerResponseIdentity, error) {
	return f.identity, f.identityErr
}

func (f *fakeSellerStore) UpsertSellerResponseDraft(_ context.Context, _ int64, payload json.RawMessage, version int32) (repository.SellerResponseDraft, error) {
	if f.draftErr != nil {
		return repository.SellerResponseDraft{}, f.draftErr
	}
	return repository.SellerResponseDraft{Payload: payload, Version: version + 1}, nil
}

func (f *fakeSellerStore) FindSellerResponseDraft(context.Context, int64) (repository.SellerResponseDraft, error) {
	return f.draft, f.draftErr
}

func (f *fakeSellerStore) FindSellerResponseVersionByIdempotencyKey(context.Context, int64, string) (repository.SellerResponseVersion, error) {
	if f.replayVersion.Version == 0 {
		return repository.SellerResponseVersion{}, repository.ErrSellerResponseNotFound
	}
	return f.replayVersion, nil
}

func (f *fakeSellerStore) GetPublicationInviteSellerView(context.Context, int64) (repository.InviteSellerView, error) {
	if f.sellerViewErr != nil {
		return repository.InviteSellerView{}, f.sellerViewErr
	}
	return f.sellerView, nil
}

func (f *fakeSellerStore) FindSellerResponseVersion(context.Context, int64, int32) (repository.SellerResponseVersion, error) {
	if f.versionErr != nil {
		return repository.SellerResponseVersion{}, f.versionErr
	}
	return f.version, nil
}

func (f *fakeSellerStore) InsertSellerResponseVersion(_ context.Context, in repository.InsertSellerResponseVersionInput) (repository.SellerResponseVersion, error) {
	f.insertCalls = append(f.insertCalls, in)
	if f.insertErr != nil {
		return repository.SellerResponseVersion{}, f.insertErr
	}
	if f.insertResult.Version != 0 {
		return f.insertResult, nil
	}
	return repository.SellerResponseVersion{
		SellerResponseID: in.SellerResponseID, Version: in.Version,
		TotalAmount: in.TotalAmount, PayloadHash: in.PayloadHash, ValidUntil: in.ValidUntil,
	}, nil
}

func (f *fakeSellerStore) BumpSellerResponseLatestVersion(_ context.Context, _ int64, _, latest int32) (repository.SellerResponseIdentity, error) {
	f.bumpCalls = append(f.bumpCalls, latest)
	if f.bumpErr != nil {
		return repository.SellerResponseIdentity{}, f.bumpErr
	}
	out := f.identity
	out.LatestVersion = latest
	return out, nil
}

func (f *fakeSellerStore) WithdrawSellerResponse(_ context.Context, _ int64, _, withdrawn int32) (repository.SellerResponseIdentity, error) {
	if f.withdrawErr != nil {
		return repository.SellerResponseIdentity{}, f.withdrawErr
	}
	out := f.identity
	out.WithdrawnVersion = &withdrawn
	return out, nil
}

func (f *fakeSellerStore) CloseSellerResponseWrite(context.Context, int64, int32, string) (repository.SellerResponseIdentity, error) {
	if f.closeErr != nil {
		return repository.SellerResponseIdentity{}, f.closeErr
	}
	out := f.identity
	at := sellerResponseTestNow
	out.WriteClosedAt = &at
	return out, nil
}

func validQuote() domain.SellerQuotePayload {
	return domain.SellerQuotePayload{
		ResponseType:        domain.SellerResponseTypeManualQuote,
		OfferedQuality:      domain.OfferedQuality{Grade: "特級", InspectionNotes: "產地直送，抽驗合格"},
		UnitBasis:           domain.UnitBasis{Quantity: "1", Unit: "BOX"},
		OfferedQuantity:     "200",
		OfferedQuantityUnit: "BOX",
		ConversionBasis:     "1 BOX = 10 KG",
		Currency:            domain.CurrencyTWD,
		GoodsAmount:         "30000",
		ShippingAmount:      "800",
		TaxAmount:           "0",
		TaxInclusionBasis:   domain.TaxInclusionBasisIncluded,
		TotalAmount:         "30800",
		ValidUntil:          sellerResponseTestNow.Add(48 * time.Hour).Format(time.RFC3339),
	}
}

func newSellerUC(store *fakeSellerStore) *SellerResponseUseCase {
	return NewSellerResponseUseCase(store, passthroughTx{}, sellerResponseNow)
}

func baseSellerStore() *fakeSellerStore {
	storeID := int64(77)
	return &fakeSellerStore{
		publication: repository.Publication{ID: 10, Version: 1, BuyerID: 1234, RequestID: 5, SellerFacingPayloadHash: "pubhash"},
		invite:      repository.PublicationInvite{ID: 99, PublicationID: 10, StoreID: storeID},
		identity: repository.SellerResponseIdentity{
			ID: 300, Version: 1, PublicationID: 10, BuyerID: 1234, RequestID: 5,
			Lane: repository.SellerResponseLanePlatform, StoreID: &storeID,
		},
		snapshotErr: repository.ErrConfirmedRfqSnapshotNotFound,
	}
}

func TestSubmit_FirstVersionBumpsIdentity(t *testing.T) {
	store := baseSellerStore()
	uc := newSellerUC(store)

	got, err := uc.Submit(context.Background(), SubmitSellerResponseInput{
		Principal:                       SellerPrincipal{StoreID: 77},
		PublicationID:                   10,
		ClientRequestID:                 "idem-1",
		ExpectedPublicationVersion:      "1",
		ExpectedSellerFacingPayloadHash: "pubhash",
		ExpectedSellerResponseVersion:   "",
		Quote:                           validQuote(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version.Version != 1 || got.Replayed {
		t.Fatalf("version = %d replayed = %v, want 1/false", got.Version.Version, got.Replayed)
	}
	if len(store.bumpCalls) != 1 || store.bumpCalls[0] != 1 {
		t.Fatalf("bumpCalls = %v, want [1]", store.bumpCalls)
	}
	if got.Status != domain.SellerResponseStatusActive {
		t.Fatalf("status = %q, want ACTIVE", got.Status)
	}
	// ⛔ 金額由 server 從 payload 轉成整數「元」後落庫，不是原樣透傳字串。
	if store.insertCalls[0].TotalAmount != 30800 || store.insertCalls[0].GoodsAmount != 30000 {
		t.Fatalf("amounts = %d/%d", store.insertCalls[0].TotalAmount, store.insertCalls[0].GoodsAmount)
	}
}

// TestSubmit_ReplayDoesNotBumpLatestVersion pins the idempotent-replay branch:
// re-sending the same Idempotency-Key returns the original version and must not
// bump latest_version again — a second bump would push the identity row to a
// number with no matching version row.
func TestSubmit_ReplayDoesNotBumpLatestVersion(t *testing.T) {
	store := baseSellerStore()
	store.identity.LatestVersion = 3
	store.insertResult = repository.SellerResponseVersion{
		SellerResponseID: 300, Version: 2, TotalAmount: 30800, ValidUntil: sellerResponseTestNow.Add(time.Hour),
	}
	uc := newSellerUC(store)

	got, err := uc.Submit(context.Background(), SubmitSellerResponseInput{
		Principal: SellerPrincipal{StoreID: 77}, PublicationID: 10, ClientRequestID: "idem-old",
		ExpectedSellerResponseVersion: "3", Quote: validQuote(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Replayed {
		t.Fatal("replayed = false, want true")
	}
	if len(store.bumpCalls) != 0 {
		t.Fatalf("bumpCalls = %v, want none on replay", store.bumpCalls)
	}
}

func TestSubmit_EmptyExpectedVersionOnExistingSubmissionIsAlreadySubmitted(t *testing.T) {
	store := baseSellerStore()
	store.identity.LatestVersion = 1
	uc := newSellerUC(store)

	_, err := uc.Submit(context.Background(), SubmitSellerResponseInput{
		Principal: SellerPrincipal{StoreID: 77}, PublicationID: 10, ClientRequestID: "idem-1",
		ExpectedSellerResponseVersion: "", Quote: validQuote(),
	})
	if !errors.Is(err, ErrSellerResponseAlreadySubmitted) {
		t.Fatalf("err = %v, want ErrSellerResponseAlreadySubmitted", err)
	}
	if len(store.insertCalls) != 0 {
		t.Fatal("wrote a version despite the CAS failure")
	}
}

func TestSubmit_WrongExpectedVersionIsStale(t *testing.T) {
	store := baseSellerStore()
	store.identity.LatestVersion = 4
	uc := newSellerUC(store)

	_, err := uc.Submit(context.Background(), SubmitSellerResponseInput{
		Principal: SellerPrincipal{StoreID: 77}, PublicationID: 10, ClientRequestID: "idem-1",
		ExpectedSellerResponseVersion: "3", Quote: validQuote(),
	})
	if !errors.Is(err, repository.ErrSellerResponseStale) {
		t.Fatalf("err = %v, want ErrSellerResponseStale", err)
	}
}

func TestSubmit_PublicationCASMismatchIsStalePublication(t *testing.T) {
	for _, tc := range []struct {
		name          string
		version, hash string
	}{
		{"version", "2", "pubhash"},
		{"payloadHash", "1", "other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := baseSellerStore()
			uc := newSellerUC(store)
			_, err := uc.Submit(context.Background(), SubmitSellerResponseInput{
				Principal: SellerPrincipal{StoreID: 77}, PublicationID: 10, ClientRequestID: "idem-1",
				ExpectedPublicationVersion: tc.version, ExpectedSellerFacingPayloadHash: tc.hash,
				Quote: validQuote(),
			})
			if !errors.Is(err, ErrStalePublication) {
				t.Fatalf("err = %v, want ErrStalePublication", err)
			}
		})
	}
}

// TestSubmit_AmountIdentityIsEnforced covers QUOTE_AMOUNT_INCONSISTENT: the
// server, not the client, decides whether total equals goods+shipping+tax.
func TestSubmit_AmountIdentityIsEnforced(t *testing.T) {
	store := baseSellerStore()
	quote := validQuote()
	quote.TotalAmount = "30000" // != 30000 + 800 + 0
	uc := newSellerUC(store)

	_, err := uc.Submit(context.Background(), SubmitSellerResponseInput{
		Principal: SellerPrincipal{StoreID: 77}, PublicationID: 10, ClientRequestID: "idem-1",
		Quote: quote,
	})
	if !errors.Is(err, domain.ErrQuoteAmountInconsistent) {
		t.Fatalf("err = %v, want ErrQuoteAmountInconsistent", err)
	}
}

// TestSubmit_QualityIsRequiredForBothResponseTypes pins the rule that
// ACCEPT_NON_PRICE_TERMS_WITH_QUOTE is not a shortcut past the quality fields.
func TestSubmit_QualityIsRequiredForBothResponseTypes(t *testing.T) {
	for _, rt := range []string{
		domain.SellerResponseTypeManualQuote,
		domain.SellerResponseTypeAcceptNonPriceTermsWithQuote,
	} {
		t.Run(rt, func(t *testing.T) {
			store := baseSellerStore()
			quote := validQuote()
			quote.ResponseType = rt
			quote.OfferedQuality = domain.OfferedQuality{}
			uc := newSellerUC(store)

			_, err := uc.Submit(context.Background(), SubmitSellerResponseInput{
				Principal: SellerPrincipal{StoreID: 77}, PublicationID: 10, ClientRequestID: "idem-1",
				Quote: quote,
			})
			if !errors.Is(err, domain.ErrQuoteQualityRequired) {
				t.Fatalf("err = %v, want ErrQuoteQualityRequired", err)
			}
		})
	}
}

func TestSubmit_MissingIdempotencyKeyIsRejected(t *testing.T) {
	uc := newSellerUC(baseSellerStore())
	_, err := uc.Submit(context.Background(), SubmitSellerResponseInput{
		Principal: SellerPrincipal{StoreID: 77}, PublicationID: 10, Quote: validQuote(),
	})
	if !errors.Is(err, ErrIdempotencyKeyRequired) {
		t.Fatalf("err = %v, want ErrIdempotencyKeyRequired", err)
	}
}

func TestSubmit_WriteClosedRecipientCannotSubmit(t *testing.T) {
	store := baseSellerStore()
	at := sellerResponseTestNow
	store.identity.WriteClosedAt = &at
	uc := newSellerUC(store)

	_, err := uc.Submit(context.Background(), SubmitSellerResponseInput{
		Principal: SellerPrincipal{StoreID: 77}, PublicationID: 10, ClientRequestID: "idem-1",
		Quote: validQuote(),
	})
	if !errors.Is(err, repository.ErrSellerResponseWriteClosed) {
		t.Fatalf("err = %v, want ErrSellerResponseWriteClosed", err)
	}
}

func TestWithdraw_SameVersionTwiceIsIdempotent(t *testing.T) {
	store := baseSellerStore()
	store.identity.LatestVersion = 2
	withdrawn := int32(2)
	store.identity.WithdrawnVersion = &withdrawn
	uc := newSellerUC(store)

	got, err := uc.Withdraw(context.Background(), WithdrawSellerResponseInput{
		Principal: SellerPrincipal{StoreID: 77}, PublicationID: 10, SellerResponseID: 300,
		ClientRequestID: "idem-w", ExpectedSellerResponseVersion: "2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.WithdrawnVersion == nil || *got.WithdrawnVersion != 2 {
		t.Fatalf("withdrawnVersion = %v", got.WithdrawnVersion)
	}
}

func TestWithdraw_OtherSellersResponseIDIs404(t *testing.T) {
	store := baseSellerStore()
	store.identity.LatestVersion = 2
	uc := newSellerUC(store)

	_, err := uc.Withdraw(context.Background(), WithdrawSellerResponseInput{
		Principal: SellerPrincipal{StoreID: 77}, PublicationID: 10, SellerResponseID: 999,
		ClientRequestID: "idem-w", ExpectedSellerResponseVersion: "2",
	})
	if !errors.Is(err, repository.ErrSellerResponseNotFound) {
		t.Fatalf("err = %v, want ErrSellerResponseNotFound", err)
	}
}

func TestDecline_AlreadyClosedReturnsExistingRow(t *testing.T) {
	store := baseSellerStore()
	at := sellerResponseTestNow
	store.identity.WriteClosedAt = &at
	store.closeErr = errors.New("must not be called")
	uc := newSellerUC(store)

	got, err := uc.Decline(context.Background(), DeclineRfqPublicationInput{
		Principal: SellerPrincipal{StoreID: 77}, PublicationID: 10, ClientRequestID: "idem-d",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.WriteClosed() {
		t.Fatal("expected the existing closed row")
	}
}

// TestOutsideBuyerRequestedRange_OnlyJudgesOnMatchingBasis pins the deliberate
// narrowness of the advisory flag: with no comparable basis the server does not
// invent a unit conversion, so the flag stays false.
func TestOutsideBuyerRequestedRange_OnlyJudgesOnMatchingBasis(t *testing.T) {
	rangeJSON := func(unit, basisQty, lower, upper string) []byte {
		b, _ := json.Marshal(pricing.BuyerRequestedRange{
			Currency: "TWD", BasisQuantity: basisQty, BasisUnit: unit,
			LowerBoundAmount: lower, UpperBoundAmount: upper,
		})
		return b
	}
	for _, tc := range []struct {
		name string
		json []byte
		want bool
	}{
		{"goods below the range", rangeJSON("BOX", "1", "200", "250"), true},
		{"goods inside the range", rangeJSON("BOX", "1", "100", "200"), false},
		{"different unit is not judged", rangeJSON("KG", "1", "1", "2"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := baseSellerStore()
			store.snapshotErr = nil
			store.snapshot = repository.ConfirmedRfqSnapshot{ID: 1, PricePreviewID: 2, PayloadHash: "snap"}
			store.preview = repository.PricePreview{BuyerRequestedRangeJSON: tc.json}
			uc := newSellerUC(store)

			if _, err := uc.Submit(context.Background(), SubmitSellerResponseInput{
				Principal: SellerPrincipal{StoreID: 77}, PublicationID: 10, ClientRequestID: "idem-1",
				Quote: validQuote(),
			}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := store.insertCalls[0].OutsideBuyerRequestedRange; got != tc.want {
				t.Fatalf("outsideBuyerRequestedRange = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestGetPublicationDetail_CarriesTheCASSources is why this endpoint exists: the
// seller app cannot construct expectedDraftVersion / expectedSellerResponseVersion
// from anywhere else, so the detail must hand back both.
func TestGetPublicationDetail_CarriesTheCASSources(t *testing.T) {
	store := baseSellerStore()
	store.identity.LatestVersion = 3
	store.draft = repository.SellerResponseDraft{SellerResponseID: 300, Version: 5, Payload: []byte(`{}`)}
	store.version = repository.SellerResponseVersion{
		SellerResponseID: 300, Version: 3, ValidUntil: sellerResponseTestNow.Add(time.Hour),
	}
	uc := newSellerUC(store)

	got, err := uc.GetPublicationDetail(context.Background(), SellerPrincipal{StoreID: 77}, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.HasDraft || got.Draft.Version != 5 {
		t.Fatalf("draft = %+v (hasDraft=%v)", got.Draft, got.HasDraft)
	}
	if !got.HasLatest || got.Latest.Version != 3 {
		t.Fatalf("latest = %+v (hasLatest=%v)", got.Latest, got.HasLatest)
	}
	if got.LatestStatus != domain.SellerResponseStatusActive {
		t.Fatalf("latestStatus = %q, want ACTIVE", got.LatestStatus)
	}
}

// TestGetPublicationDetail_NoDraftIsNotAnError: never having saved a draft is the
// normal first-visit state, not a 404.
func TestGetPublicationDetail_NoDraftIsNotAnError(t *testing.T) {
	store := baseSellerStore()
	store.draftErr = repository.ErrSellerResponseNotFound
	uc := newSellerUC(store)

	got, err := uc.GetPublicationDetail(context.Background(), SellerPrincipal{StoreID: 77}, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.HasDraft || got.HasLatest {
		t.Fatalf("hasDraft=%v hasLatest=%v, want both false", got.HasDraft, got.HasLatest)
	}
}

// TestGetPublicationDetail_CarriesTheSellerFacingPayload：沒有需求內容的回價表單
// 是一張空白表，所以詳情必須帶著它。
func TestGetPublicationDetail_CarriesTheSellerFacingPayload(t *testing.T) {
	store := baseSellerStore()
	store.draftErr = repository.ErrSellerResponseNotFound
	store.sellerView = repository.InviteSellerView{Item: "高麗菜", Quantity: "200", Unit: "BOX"}
	uc := newSellerUC(store)

	got, err := uc.GetPublicationDetail(context.Background(), SellerPrincipal{StoreID: 77}, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SellerView.Item != "高麗菜" || got.SellerView.Quantity != "200" {
		t.Fatalf("sellerView = %+v", got.SellerView)
	}
}

// TestGetPublicationDetail_MissingSellerViewDoesNotFailTheCall：少一張需求卡仍可
// 報價，整支 500 就不行。
func TestGetPublicationDetail_MissingSellerViewDoesNotFailTheCall(t *testing.T) {
	store := baseSellerStore()
	store.draftErr = repository.ErrSellerResponseNotFound
	store.sellerViewErr = repository.ErrInviteNotFound
	uc := newSellerUC(store)

	got, err := uc.GetPublicationDetail(context.Background(), SellerPrincipal{StoreID: 77}, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SellerView.Item != "" {
		t.Fatalf("sellerView = %+v, want zero value", got.SellerView)
	}
}

// TestSubmit_LostAckRetryReplaysInsteadOf409 是 2026-09-13 dev e2e 逮到的迴歸：
// 第一次送出成功後 latest_version 已經是 1，重送同一把 Idempotency-Key 時若先跑
// CAS 守門，會回 RFQ_RESPONSE_ALREADY_SUBMITTED——與「有人搶先改版」同一個碼，
// 呼叫端分不出自己其實已經成功。契約 §5.1 要求同 key 同內容回放同一筆結果。
func TestSubmit_LostAckRetryReplaysInsteadOf409(t *testing.T) {
	store := baseSellerStore()
	store.identity.LatestVersion = 1 // 第一次送出已經把它推上去了
	quote := validQuote()
	hash, err := quote.Hash()
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	store.replayVersion = repository.SellerResponseVersion{
		SellerResponseID: 300, Version: 1, PayloadHash: hash,
		ValidUntil: sellerResponseTestNow.Add(time.Hour),
	}
	uc := newSellerUC(store)

	got, err := uc.Submit(context.Background(), SubmitSellerResponseInput{
		Principal: SellerPrincipal{StoreID: 77}, PublicationID: 10, ClientRequestID: "idem-1",
		ExpectedSellerResponseVersion: "", Quote: quote,
	})
	if err != nil {
		t.Fatalf("重送同一把冪等鍵應回放，卻回錯誤：%v", err)
	}
	if !got.Replayed || got.Version.Version != 1 {
		t.Fatalf("replayed=%v version=%d，want true/1", got.Replayed, got.Version.Version)
	}
	if len(store.insertCalls) != 0 || len(store.bumpCalls) != 0 {
		t.Fatalf("回放不得再寫入：insert=%v bump=%v", store.insertCalls, store.bumpCalls)
	}
}

// TestSubmit_SameKeyDifferentContentIsConflict：同一把 key 換內容不是回放。
func TestSubmit_SameKeyDifferentContentIsConflict(t *testing.T) {
	store := baseSellerStore()
	store.identity.LatestVersion = 1
	store.replayVersion = repository.SellerResponseVersion{
		SellerResponseID: 300, Version: 1, PayloadHash: "another-hash",
		ValidUntil: sellerResponseTestNow.Add(time.Hour),
	}
	uc := newSellerUC(store)

	_, err := uc.Submit(context.Background(), SubmitSellerResponseInput{
		Principal: SellerPrincipal{StoreID: 77}, PublicationID: 10, ClientRequestID: "idem-1",
		ExpectedSellerResponseVersion: "", Quote: validQuote(),
	})
	if !errors.Is(err, repository.ErrSellerResponseIdempotencyConflict) {
		t.Fatalf("err = %v, want ErrSellerResponseIdempotencyConflict", err)
	}
}

func TestUpsertDraft_ExternalDerivesPublicationFromInvite(t *testing.T) {
	store := baseSellerStore()
	store.invite = repository.PublicationInvite{ID: 4242, PublicationID: 777}
	uc := newSellerUC(store)

	_, err := uc.UpsertDraft(context.Background(), UpsertSellerResponseDraftInput{
		Principal: SellerPrincipal{InviteID: 4242}, PublicationID: 0,
		ExpectedDraftVersion: "", Draft: validQuote(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.findPublicationByIDCalls) != 1 || store.findPublicationByIDCalls[0] != 777 {
		t.Fatalf("FindPublicationByID calls = %v, want exactly [777]", store.findPublicationByIDCalls)
	}
}

func TestUpsertDraft_ExternalStillCrossChecksWhenPublicationGiven(t *testing.T) {
	store := baseSellerStore()
	store.invite = repository.PublicationInvite{ID: 4242, PublicationID: 777}
	uc := newSellerUC(store)

	_, err := uc.UpsertDraft(context.Background(), UpsertSellerResponseDraftInput{
		Principal: SellerPrincipal{InviteID: 4242}, PublicationID: 999,
		ExpectedDraftVersion: "", Draft: validQuote(),
	})
	if !errors.Is(err, repository.ErrInviteNotFound) {
		t.Fatalf("err = %v, want ErrInviteNotFound", err)
	}
}

func TestListPublications_DerivesPerRowStatus(t *testing.T) {
	validUntil := sellerResponseTestNow.Add(time.Hour)
	expired := sellerResponseTestNow.Add(-time.Hour)
	withdrawn := int32(2)
	cases := []struct {
		name      string
		row       repository.PublicationForStore
		wantState string
		wantDraft bool
	}{
		{"尚未回價但存過草稿", repository.PublicationForStore{HasDraft: true}, "", true},
		{"從未動過", repository.PublicationForStore{}, "", false},
		{"有效", repository.PublicationForStore{
			LatestVersion: ptr32(2), LatestVersionValidUntil: &validUntil,
		}, domain.SellerResponseStatusActive, false},
		{"已逾有效期", repository.PublicationForStore{
			LatestVersion: ptr32(2), LatestVersionValidUntil: &expired,
		}, domain.SellerResponseStatusExpired, false},
		{"已撤回", repository.PublicationForStore{
			LatestVersion: ptr32(2), WithdrawnVersion: &withdrawn, LatestVersionValidUntil: &validUntil,
		}, domain.SellerResponseStatusWithdrawn, false},
		{"latest_version=0 不得推導出狀態", repository.PublicationForStore{
			LatestVersion: ptr32(0), LatestVersionValidUntil: &validUntil,
		}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := baseSellerStore()
			store.publicationsForStore = []repository.PublicationForStore{tc.row}
			uc := newSellerUC(store)
			got, err := uc.ListPublications(context.Background(), 77, 50)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("len = %d, want 1", len(got))
			}
			if got[0].LatestResponseStatus != tc.wantState {
				t.Fatalf("status = %q, want %q", got[0].LatestResponseStatus, tc.wantState)
			}
			if got[0].HasDraft != tc.wantDraft {
				t.Fatalf("hasDraft = %v, want %v", got[0].HasDraft, tc.wantDraft)
			}
		})
	}
}

func ptr32(v int32) *int32 { return &v }
