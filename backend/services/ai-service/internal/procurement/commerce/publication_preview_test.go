package commerce

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

const (
	pubBuyerID     = identity.BuyerID(77)
	pubRequestID   = int64(4200)
	pubRevisionID  = int64(9)
	pubSnapshotID  = int64(5100)
	pubPreviewID   = int64(6100)
	pubCandSetID   = int64(900)
	pubCanonical   = `{"buyerRequestedRange":{"min":"80","max":"120"},"item":"高麗菜","quantity":100,"unit":"公斤","deliveryCity":"台中市"}`
	pubSnapshotSum = "snapshot-hash"
)

// ---- fakes ----

type fakePublicationApprovalRepo struct {
	insertCalls int
	byKey       map[string]repository.PublicationApproval
	lastInput   repository.InsertPublicationApprovalInput
}

func (f *fakePublicationApprovalRepo) InsertPublicationApproval(ctx context.Context, in repository.InsertPublicationApprovalInput) (repository.PublicationApproval, error) {
	f.insertCalls++
	f.lastInput = in
	if existing, ok := f.byKey[in.PreviewClientRequestID]; ok {
		if existing.SellerFacingPayloadHash != in.SellerFacingPayloadHash || existing.AudienceHash != in.AudienceHash {
			return repository.PublicationApproval{}, repository.ErrPublicationApprovalIdempotencyConflict
		}
		return existing, nil
	}
	row := repository.PublicationApproval{
		ID: 7000 + int64(f.insertCalls), Version: 1, BuyerID: in.BuyerID,
		RequestID: in.RequestID, RequestRevisionID: in.RequestRevisionID,
		AudienceMode: in.AudienceMode, AudiencePolicyID: in.AudiencePolicyID,
		AudiencePolicyVersion:   in.AudiencePolicyVersion,
		AudienceHash:            in.AudienceHash,
		SellerFacingPayloadJSON: in.SellerFacingPayloadJSON,
		SellerFacingPayloadHash: in.SellerFacingPayloadHash,
		State:                   in.State, SchemaVersion: in.SchemaVersion,
		PreviewClientRequestID: in.PreviewClientRequestID,
		GeneratedAt:            in.GeneratedAt, ExpiresAt: in.ExpiresAt,
	}
	if f.byKey == nil {
		f.byKey = map[string]repository.PublicationApproval{}
	}
	f.byKey[in.PreviewClientRequestID] = row
	return row, nil
}

func (f *fakePublicationApprovalRepo) FindPublicationApprovalByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, previewClientRequestID string) (repository.PublicationApproval, error) {
	if row, ok := f.byKey[previewClientRequestID]; ok {
		return row, nil
	}
	return repository.PublicationApproval{}, repository.ErrPublicationApprovalNotFound
}

// fakeConfirmedRfqSnapshotByIDReader 依 (id, buyerID) 查，模擬真實實作——
// wrong-owner 猜 id 必須查不到。
type fakeConfirmedRfqSnapshotByIDReader struct {
	snap repository.ConfirmedRfqSnapshot
}

func (f fakeConfirmedRfqSnapshotByIDReader) FindConfirmedRfqSnapshotByIDAndBuyer(ctx context.Context, id int64, buyerID identity.BuyerID) (repository.ConfirmedRfqSnapshot, error) {
	if id != f.snap.ID || buyerID != f.snap.BuyerID {
		return repository.ConfirmedRfqSnapshot{}, repository.ErrConfirmedRfqSnapshotNotFound
	}
	return f.snap, nil
}

type fakePricePreviewByIDReader struct {
	preview repository.PricePreview
}

func (f fakePricePreviewByIDReader) FindPricePreviewByIDAndBuyer(ctx context.Context, id int64, buyerID identity.BuyerID) (repository.PricePreview, error) {
	if id != f.preview.ID || buyerID != f.preview.BuyerID {
		return repository.PricePreview{}, repository.ErrPricePreviewNotFound
	}
	return f.preview, nil
}

// fakeCandidateSetReader 依 (candidateSetID, requestID, buyerID) 查，模擬真實
// GetCandidateSetWithCandidates——別的 request／別的買家的 candidate set 必須查不到。
type fakeCandidateSetReader struct {
	set        repository.CandidateSet
	candidates []repository.Candidate
	err        error
}

func (f fakeCandidateSetReader) GetCandidateSetWithCandidates(ctx context.Context, candidateSetID, requestID int64, buyerID identity.BuyerID) (repository.CandidateSet, []repository.Candidate, error) {
	if f.err != nil {
		return repository.CandidateSet{}, nil, f.err
	}
	if candidateSetID != f.set.ID || requestID != f.set.RequestID || buyerID != f.set.BuyerID {
		return repository.CandidateSet{}, nil, repository.ErrNotFound
	}
	return f.set, f.candidates, nil
}

// ---- fixtures ----

func newPublicationPreviewFixture() (*PublicationPreviewUseCase, *fakePublicationApprovalRepo) {
	approvals := &fakePublicationApprovalRepo{}
	uc := NewPublicationPreviewUseCase(
		approvals,
		fakeConfirmedRfqSnapshotByIDReader{snap: repository.ConfirmedRfqSnapshot{
			ID: pubSnapshotID, BuyerID: pubBuyerID, RequestID: pubRequestID,
			RequestRevisionID: pubRevisionID, PricePreviewID: pubPreviewID, PricePreviewVersion: 1,
			CanonicalPayloadJSON: json.RawMessage(pubCanonical), PayloadHash: pubSnapshotSum,
		}},
		fakePricePreviewByIDReader{preview: repository.PricePreview{
			ID: pubPreviewID, BuyerID: pubBuyerID, RequestID: pubRequestID,
			RequestRevisionID:       pubRevisionID,
			BuyerRequestedRangeJSON: json.RawMessage(`{"min":"80","max":"120"}`),
			PreviewInputHash:        "preview-input-hash", PayloadHash: "preview-payload-hash",
			Version: 1,
		}},
		fakeCandidateSetReader{
			set: repository.CandidateSet{
				ID: pubCandSetID, RequestID: pubRequestID, RequestRevisionID: pubRevisionID,
				BuyerID: pubBuyerID, PolicyVersion: 3,
			},
			candidates: []repository.Candidate{
				{ID: 11, CandidateSetID: pubCandSetID, Lane: "PLATFORM", StoreID: identity.StoreID(501)},
				{ID: 22, CandidateSetID: pubCandSetID, Lane: "PLATFORM", StoreID: identity.StoreID(502)},
				{ID: 33, CandidateSetID: pubCandSetID, Lane: "EXTERNAL", ProspectID: pubInt64Ptr(9001)},
			},
		},
		testkit.FakeRevisionReader{RevisionID: pubRevisionID},
	)
	return uc, approvals
}

func wishPoolInput() PreviewRfqPublicationInput {
	return PreviewRfqPublicationInput{
		BuyerID: pubBuyerID, RequestID: pubRequestID,
		PreviewClientRequestID: "preview-key-1",
		ExpectedRevisionID:     pubRevisionID,
		ConfirmedRfqSnapshotID: pubSnapshotID,
		AudienceMode:           "WISH_POOL",
	}
}

func directInput(selected ...int64) PreviewRfqPublicationInput {
	in := wishPoolInput()
	in.AudienceMode = "DIRECT_RECIPIENTS"
	in.CandidateSetID = pubCandSetID
	in.SelectedCandidateIDs = selected
	return in
}

// ---- tests ----

// WISH_POOL 的 audienceHash 完全由 server 的 policy 決定，且回應必須帶回
// approvalId ＋ approvalVersion——Slice 2 的 execute 拿這兩者做 CAS，回應是呼叫端
// 唯一的來源（既有 runs/{runId}/cancel 就踩過「必填 CAS 欄位在 wire 上無來源」）。
func TestPreviewRfqPublication_WishPool_ReturnsServerOwnedAudienceHashAndCASTokens(t *testing.T) {
	uc, approvals := newPublicationPreviewFixture()

	got, err := uc.PreviewRfqPublication(context.Background(), wishPoolInput())
	require.NoError(t, err)

	wantHash, err := domain.WishPoolAudience().Hash()
	require.NoError(t, err)
	require.Equal(t, wantHash, got.AudienceHash)

	require.NotZero(t, got.ApprovalID, "execute 的 CAS 需要 approvalId")
	require.Equal(t, 1, got.ApprovalVersion, "execute 的 CAS 需要 approvalVersion，不得只回 id")
	require.Equal(t, "WISH_POOL_ALL_ACTIVE_SELLERS_V1", got.AudiencePolicyID)
	require.Equal(t, 1, got.AudiencePolicyVersion)
	require.Equal(t, 0, got.RecipientCount, "WISH_POOL 沒有名單是本模式的定義")
	require.False(t, got.ExpiresAt.IsZero())

	require.Equal(t, 1, approvals.insertCalls)
	require.Equal(t, "PENDING", approvals.lastInput.State)
	require.Nil(t, approvals.lastInput.CandidateSetID, "WISH_POOL 不得寫入 candidate set")
	require.Nil(t, approvals.lastInput.CandidateSetPolicyVersion)
	require.Empty(t, approvals.lastInput.Recipients, "WISH_POOL 沒有名單是本模式的定義")
	require.Equal(t, pubSnapshotSum, approvals.lastInput.ConfirmedRfqPayloadHash)
	require.Equal(t, "preview-payload-hash", approvals.lastInput.PricePreviewPayloadHash)
	require.Equal(t, "preview-input-hash", approvals.lastInput.PricePreviewInputHash)
	require.NotEmpty(t, approvals.lastInput.BuyerRequestedRangeHash)
}

// DIRECT 的 recipient 順序由 selectedCandidateIds 決定並且有意義：換順序就是換受眾。
func TestPreviewRfqPublication_DirectRecipients_PreservesSelectionOrderInHash(t *testing.T) {
	uc, _ := newPublicationPreviewFixture()

	forward, err := uc.PreviewRfqPublication(context.Background(), directInput(11, 22))
	require.NoError(t, err)

	uc2, _ := newPublicationPreviewFixture()
	reversedIn := directInput(22, 11)
	reversedIn.PreviewClientRequestID = "preview-key-2"
	reversed, err := uc2.PreviewRfqPublication(context.Background(), reversedIn)
	require.NoError(t, err)

	require.NotEqual(t, forward.AudienceHash, reversed.AudienceHash, "換順序就是換受眾")
	require.Equal(t, 2, forward.RecipientCount)

	wantForward, err := domain.DirectRecipientsAudience(pubCandSetID, 3, []domain.PublicationRecipient{
		{StoreID: 501, Lane: domain.QuoteLanePlatform},
		{StoreID: 502, Lane: domain.QuoteLanePlatform},
	}).Hash()
	require.NoError(t, err)
	require.Equal(t, wantForward, forward.AudienceHash)
}

// approvals.lastInput.Recipients 必須與 domain.DirectRecipientsAudience(...) 用來算
// audienceHash 的那份逐項相同（含順序與 lane）——Slice 1 只把 audienceHash 寫進 DB，
// hash 不可逆，execute 要重驗「發給誰」就必須連名單本身一起持久化，不能只有 hash。
func TestPreviewRfqPublication_DirectRecipients_PersistsRecipientsMatchingAudience(t *testing.T) {
	uc, approvals := newPublicationPreviewFixture()

	_, err := uc.PreviewRfqPublication(context.Background(), directInput(11, 33, 22))
	require.NoError(t, err)

	want := []domain.PublicationRecipient{
		domain.PlatformRecipient(501),
		domain.ExternalRecipient(9001),
		domain.PlatformRecipient(502),
	}
	require.Equal(t, want, approvals.lastInput.Recipients, "順序與 lane 必須逐項相同，不是集合比對")

	wantHash, err := domain.DirectRecipientsAudience(pubCandSetID, 3, want).Hash()
	require.NoError(t, err)
	require.Equal(t, wantHash, approvals.lastInput.AudienceHash, "Recipients 必須是算出 audienceHash 的同一份，不是另外湊的")
}

// revision 漂移擋在寫入之前，零 approval 寫入。
func TestPreviewRfqPublication_StaleRevision_WritesNothing(t *testing.T) {
	uc, approvals := newPublicationPreviewFixture()

	in := wishPoolInput()
	in.ExpectedRevisionID = pubRevisionID + 1

	_, err := uc.PreviewRfqPublication(context.Background(), in)
	require.ErrorIs(t, err, ErrRevisionStale)
	require.Zero(t, approvals.insertCalls, "擋下的請求不得有部分寫入")
}

// candidate set 不屬於本 request／本買家 → 擋下，零寫入。
func TestPreviewRfqPublication_CandidateSetFromAnotherRequest_WritesNothing(t *testing.T) {
	uc, approvals := newPublicationPreviewFixture()

	in := directInput(11)
	in.CandidateSetID = pubCandSetID + 1

	_, err := uc.PreviewRfqPublication(context.Background(), in)
	require.ErrorIs(t, err, ErrCandidateSetMismatch)
	require.Zero(t, approvals.insertCalls)
}

// 選到不屬於這個 candidate set 的候選一樣是 mismatch，不得靜默略過。
func TestPreviewRfqPublication_UnknownSelectedCandidate_WritesNothing(t *testing.T) {
	uc, approvals := newPublicationPreviewFixture()

	_, err := uc.PreviewRfqPublication(context.Background(), directInput(11, 999))
	require.ErrorIs(t, err, ErrCandidateSetMismatch)
	require.Zero(t, approvals.insertCalls)
}

// confirmed snapshot 屬於別的 request → 擋下。snapshot 的 id 是呼叫端傳的，
// 不驗證歸屬等於讓買家把 A 案的定案內容發成 B 案的詢價。
func TestPreviewRfqPublication_SnapshotFromAnotherRequest_WritesNothing(t *testing.T) {
	uc, approvals := newPublicationPreviewFixture()

	in := wishPoolInput()
	in.RequestID = pubRequestID + 1

	_, err := uc.PreviewRfqPublication(context.Background(), in)
	require.Error(t, err)
	require.Zero(t, approvals.insertCalls)
}

// WISH_POOL 帶 candidate set 是形狀矛盾，在應用層就擋下（DB CHECK 是最終防線，
// 不是唯一防線）。
func TestPreviewRfqPublication_WishPoolWithCandidateSet_WritesNothing(t *testing.T) {
	uc, approvals := newPublicationPreviewFixture()

	in := wishPoolInput()
	in.CandidateSetID = pubCandSetID

	_, err := uc.PreviewRfqPublication(context.Background(), in)
	require.ErrorIs(t, err, ErrCandidateSetMismatch)
	require.Zero(t, approvals.insertCalls)
}

// EXTERNAL lane 的候選以 prospect_id 作為身分進入受眾。
//
// ⛔ 最關鍵的是**不得拿 store_id 當通用身分**：EXTERNAL 候選的 store_id 恆為 0，
// 那樣兩個不同的站外業者會算出同一個 audienceHash。這裡直接比對 domain 端用
// prospect id 建出來的期望 hash。
func TestPreviewRfqPublication_ExternalLaneRecipient_UsesProspectIdentity(t *testing.T) {
	uc, approvals := newPublicationPreviewFixture()

	got, err := uc.PreviewRfqPublication(context.Background(), directInput(33))
	require.NoError(t, err)
	require.Equal(t, 1, got.RecipientCount)

	want, err := domain.DirectRecipientsAudience(pubCandSetID, 3, []domain.PublicationRecipient{
		domain.ExternalRecipient(9001),
	}).Hash()
	require.NoError(t, err)
	require.Equal(t, want, got.AudienceHash)
	require.Equal(t, 1, approvals.insertCalls)
}

// 混合 lane 的受眾：兩條 lane 各自用自己的身分欄位，順序照 selectedCandidateIds。
func TestPreviewRfqPublication_MixedLaneRecipients(t *testing.T) {
	uc, _ := newPublicationPreviewFixture()

	got, err := uc.PreviewRfqPublication(context.Background(), directInput(11, 33))
	require.NoError(t, err)
	require.Equal(t, 2, got.RecipientCount)

	want, err := domain.DirectRecipientsAudience(pubCandSetID, 3, []domain.PublicationRecipient{
		domain.PlatformRecipient(501), domain.ExternalRecipient(9001),
	}).Hash()
	require.NoError(t, err)
	require.Equal(t, want, got.AudienceHash)
}

// 候選列的 lane 與身分欄位對不上是伺服器端資料不變式被違反，擋下且零寫入。
func TestPreviewRfqPublication_CandidateIdentityUnresolved_WritesNothing(t *testing.T) {
	approvals := &fakePublicationApprovalRepo{}
	uc := NewPublicationPreviewUseCase(
		approvals,
		fakeConfirmedRfqSnapshotByIDReader{snap: repository.ConfirmedRfqSnapshot{
			ID: pubSnapshotID, BuyerID: pubBuyerID, RequestID: pubRequestID,
			RequestRevisionID: pubRevisionID, PricePreviewID: pubPreviewID, PricePreviewVersion: 1,
			CanonicalPayloadJSON: json.RawMessage(pubCanonical), PayloadHash: pubSnapshotSum,
		}},
		fakePricePreviewByIDReader{preview: repository.PricePreview{
			ID: pubPreviewID, BuyerID: pubBuyerID, RequestID: pubRequestID,
			RequestRevisionID:       pubRevisionID,
			BuyerRequestedRangeJSON: json.RawMessage(`{"min":"80","max":"120"}`),
			PreviewInputHash:        "preview-input-hash", PayloadHash: "preview-payload-hash",
			Version: 1,
		}},
		fakeCandidateSetReader{
			set: repository.CandidateSet{
				ID: pubCandSetID, RequestID: pubRequestID, RequestRevisionID: pubRevisionID,
				BuyerID: pubBuyerID, PolicyVersion: 3,
			},
			// PLATFORM 卻沒有 store_id — 這一列本身壞掉。
			candidates: []repository.Candidate{{ID: 44, CandidateSetID: pubCandSetID, Lane: "PLATFORM"}},
		},
		testkit.FakeRevisionReader{RevisionID: pubRevisionID},
	)

	_, err := uc.PreviewRfqPublication(context.Background(), directInput(44))
	require.ErrorIs(t, err, ErrCandidateIdentityUnresolved)
	require.Zero(t, approvals.insertCalls)
}

// DIRECT 少了 selectedCandidateIds 是形狀矛盾：空名單不得冒充 Wish Pool。
func TestPreviewRfqPublication_DirectWithoutSelection_WritesNothing(t *testing.T) {
	uc, approvals := newPublicationPreviewFixture()

	_, err := uc.PreviewRfqPublication(context.Background(), directInput())
	require.ErrorIs(t, err, ErrCandidateSetMismatch)
	require.Zero(t, approvals.insertCalls)
}

// seller-facing payload 由 confirmed snapshot 編譯，買家可接受上下限不得外洩。
func TestPreviewRfqPublication_SellerFacingPayloadExcludesBuyerPrivateRange(t *testing.T) {
	uc, approvals := newPublicationPreviewFixture()

	in := wishPoolInput()
	in.ResponseDeadline = testkit.Ptr("2026-09-20")

	got, err := uc.PreviewRfqPublication(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, "高麗菜", got.SellerFacingPayload.Item)
	require.Equal(t, "2026-09-20", *got.SellerFacingPayload.ResponseDeadline)

	require.NotContains(t, string(approvals.lastInput.SellerFacingPayloadJSON), "120")
	require.NotContains(t, string(approvals.lastInput.SellerFacingPayloadJSON), "buyerRequestedRange")

	wantHash, err := got.SellerFacingPayload.Hash()
	require.NoError(t, err)
	require.Equal(t, wantHash, got.SellerFacingPayloadHash)
}

// 未知 audience mode 不得靜默落到某一支 pipeline。
func TestPreviewRfqPublication_UnknownAudienceMode_WritesNothing(t *testing.T) {
	uc, approvals := newPublicationPreviewFixture()

	in := wishPoolInput()
	in.AudienceMode = "BROADCAST"

	_, err := uc.PreviewRfqPublication(context.Background(), in)
	require.ErrorIs(t, err, ErrUnknownAudienceMode)
	require.Zero(t, approvals.insertCalls)
}

// responseDeadline 格式錯誤必須在編譯 payload 之前擋下。
//
// ⛔ 這條守的不是「輸入驗證」這種一般性理由，而是一個具體的分家：持久層對解不開的
// 字串寫 SQL NULL，但同一個字串會原樣進 seller-facing payload 並被 hash 涵蓋——
// 放行的話 response_deadline 欄位與 seller_facing_payload_hash 會對同一件事給出
// 不同答案，且沒有任何錯誤訊號。
func TestPreviewRfqPublication_MalformedResponseDeadline_WritesNothing(t *testing.T) {
	for _, bad := range []string{"2026-13-01", "20260915", "2026/09/15", "tomorrow", ""} {
		t.Run(bad, func(t *testing.T) {
			uc, approvals := newPublicationPreviewFixture()
			in := wishPoolInput()
			in.ResponseDeadline = &bad

			_, err := uc.PreviewRfqPublication(context.Background(), in)
			require.ErrorIs(t, err, ErrInvalidResponseDeadline)
			require.Zero(t, approvals.insertCalls, "擋下的請求不得有部分寫入")
		})
	}
}

// 合法格式要放行，而且寫進 DB 的值與進 hash 的值是同一個字串。
func TestPreviewRfqPublication_ValidResponseDeadline_IsPersistedAndHashed(t *testing.T) {
	uc, approvals := newPublicationPreviewFixture()
	in := wishPoolInput()
	in.ResponseDeadline = testkit.Ptr("2026-09-15")

	got, err := uc.PreviewRfqPublication(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, approvals.lastInput.ResponseDeadline)
	require.Equal(t, "2026-09-15", *approvals.lastInput.ResponseDeadline)
	require.Equal(t, "2026-09-15", *got.SellerFacingPayload.ResponseDeadline)
}

func pubInt64Ptr(v int64) *int64 { return &v }
