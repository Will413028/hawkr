//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
)

const (
	acceptRequestID     = int64(9500)
	acceptPublicationID = int64(9600)
	acceptSellerRespID  = int64(9700)
	acceptStoreID       = int64(9800)
)

func newAcceptanceRepo(t *testing.T) (*Repo, context.Context) {
	t.Helper()
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	t.Cleanup(cleanup)
	return New(pool, database.NewTxManager(pool), testIDGen()), ctx
}

func acceptanceInput(clientRequestID, requestHash string) InsertBuyerQuoteAcceptanceInput {
	storeID := acceptStoreID
	return InsertBuyerQuoteAcceptanceInput{
		BuyerID:                   testBuyerID,
		RequestID:                 acceptRequestID,
		PublicationID:             acceptPublicationID,
		SellerResponseID:          acceptSellerRespID,
		SellerResponseVersion:     1,
		SellerResponsePayloadHash: "resp-hash-v1",
		ConfirmedRfqSnapshotID:    9900,
		ConfirmedRfqPayloadHash:   "snapshot-hash",
		Lane:                      SellerResponseLanePlatform,
		StoreID:                   &storeID,
		AcceptedDealPrice:         30800,
		Currency:                  "TWD",
		GoodsAmount:               30000,
		ShippingAmount:            800,
		TaxAmount:                 0,
		ClientRequestID:           clientRequestID,
		RequestHash:               requestHash,
		AcceptedAt:                refTime(),
	}
}

func TestRepo_Shortlist_PutIsIdempotentAndRepins(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	first, err := r.PutShortlistEntry(ctx, testBuyerID, acceptRequestID, acceptSellerRespID, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, first.SellerResponseVersion)

	again, err := r.PutShortlistEntry(ctx, testBuyerID, acceptRequestID, acceptSellerRespID, 1)
	require.NoError(t, err)
	require.Equal(t, first.ID, again.ID)

	repinned, err := r.PutShortlistEntry(ctx, testBuyerID, acceptRequestID, acceptSellerRespID, 2)
	require.NoError(t, err)
	require.Equal(t, first.ID, repinned.ID)
	require.EqualValues(t, 2, repinned.SellerResponseVersion)

	entries, err := r.ListShortlistEntries(ctx, testBuyerID, acceptRequestID)
	require.NoError(t, err)
	require.Len(t, entries, 1, "一個賣家在比較表上只該有一列")
}

func TestRepo_Shortlist_DeleteIsIdempotent(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	_, err := r.PutShortlistEntry(ctx, testBuyerID, acceptRequestID, acceptSellerRespID, 1)
	require.NoError(t, err)

	removed, err := r.DeleteShortlistEntry(ctx, testBuyerID, acceptRequestID, acceptSellerRespID)
	require.NoError(t, err)
	require.True(t, removed)

	removedAgain, err := r.DeleteShortlistEntry(ctx, testBuyerID, acceptRequestID, acceptSellerRespID)
	require.NoError(t, err, "重複移除不得是錯誤")
	require.False(t, removedAgain)
}

func TestRepo_Shortlist_ScopedToBuyer(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)
	const otherBuyer = testBuyerID + 1

	_, err := r.PutShortlistEntry(ctx, testBuyerID, acceptRequestID, acceptSellerRespID, 1)
	require.NoError(t, err)

	entries, err := r.ListShortlistEntries(ctx, otherBuyer, acceptRequestID)
	require.NoError(t, err)
	require.Empty(t, entries, "別的買家讀不到")

	removed, err := r.DeleteShortlistEntry(ctx, otherBuyer, acceptRequestID, acceptSellerRespID)
	require.NoError(t, err)
	require.False(t, removed, "別的買家刪不掉")

	mine, err := r.ListShortlistEntries(ctx, testBuyerID, acceptRequestID)
	require.NoError(t, err)
	require.Len(t, mine, 1, "原主人的清單不受影響")
}

func TestRepo_InsertBuyerQuoteAcceptance_AckHasNoOrderYet(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	got, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-1"))
	require.NoError(t, err)
	require.Equal(t, AcceptanceStateActive, got.State)
	require.Equal(t, OrderProvisioningCreating, got.OrderProvisioningState)
	require.Nil(t, got.OrderNo, "ACK 當下訂單還沒建")
	require.Nil(t, got.PaymentDeadline)
	require.EqualValues(t, 30800, got.AcceptedDealPrice)
	require.True(t, got.Live())
}

func TestRepo_InsertBuyerQuoteAcceptance_SameKeySamePayloadReplays(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	first, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-1"))
	require.NoError(t, err)
	replay, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-1"))
	require.NoError(t, err)
	require.Equal(t, first.ID, replay.ID)
}

// TestRepo_InsertBuyerQuoteAcceptance_AcceptedAtRoundTripsExactly pins what the
// usecase's `replayed` flag rests on: a fresh insert hands back the very
// timestamp it was given (Postgres timestamptz is microsecond precision, so a
// microsecond-truncated value survives untouched), while a replay hands back the
// ORIGINAL row's timestamp and ignores the one this call passed. Without both
// halves, "replayed" would be indistinguishable from "clock drifted".
func TestRepo_InsertBuyerQuoteAcceptance_AcceptedAtRoundTripsExactly(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	first := acceptanceInput("accept-rt", "req-hash-rt")
	first.AcceptedAt = refTime().Add(1234 * time.Microsecond)
	inserted, err := r.InsertBuyerQuoteAcceptance(ctx, first)
	require.NoError(t, err)
	require.True(t, inserted.AcceptedAt.Equal(first.AcceptedAt),
		"acceptedAt 必須原值往返，否則 usecase 會把每一次新建都判成 replay：got %v want %v",
		inserted.AcceptedAt, first.AcceptedAt)

	second := acceptanceInput("accept-rt", "req-hash-rt")
	second.AcceptedAt = first.AcceptedAt.Add(time.Hour)
	replay, err := r.InsertBuyerQuoteAcceptance(ctx, second)
	require.NoError(t, err)
	require.Equal(t, inserted.ID, replay.ID)
	require.True(t, replay.AcceptedAt.Equal(first.AcceptedAt),
		"回放要回既有列的 acceptedAt，不是這次帶進來的")
	require.False(t, replay.AcceptedAt.Equal(second.AcceptedAt))
}

func TestRepo_InsertBuyerQuoteAcceptance_SameKeyDifferentPayloadConflicts(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	first, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-1"))
	require.NoError(t, err)

	_, err = r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-DIFFERENT"))
	require.ErrorIs(t, err, ErrBuyerQuoteAcceptanceIdempotencyConflict)

	unchanged, err := r.FindBuyerQuoteAcceptanceByIDAndBuyer(ctx, first.ID, testBuyerID)
	require.NoError(t, err)
	require.Equal(t, "req-hash-1", unchanged.RequestHash, "衝突必須零 mutation")
}

func TestRepo_InsertBuyerQuoteAcceptance_SecondLiveOnSameRequestRejected(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	_, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-1"))
	require.NoError(t, err)

	second := acceptanceInput("accept-2", "req-hash-2")
	second.SellerResponseID = acceptSellerRespID + 1
	_, err = r.InsertBuyerQuoteAcceptance(ctx, second)
	require.ErrorIs(t, err, ErrRequestAlreadyHasLiveAcceptance)
	require.NotErrorIs(t, err, ErrBuyerQuoteAcceptanceIdempotencyConflict,
		"這兩種失敗的復原方式不同，不得混為一談")
}

func TestRepo_ReleaseAcceptance_FreesTheSlot(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	first, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-1"))
	require.NoError(t, err)

	released, err := r.ReleaseAcceptance(ctx, first.ID, first.Version, AcceptanceStateCancelled)
	require.NoError(t, err)
	require.Equal(t, AcceptanceStateCancelled, released.State)
	require.False(t, released.Live())

	second := acceptanceInput("accept-2", "req-hash-2")
	second.SellerResponseID = acceptSellerRespID + 1
	again, err := r.InsertBuyerQuoteAcceptance(ctx, second)
	require.NoError(t, err, "取消之後同一 request 必須可以再接受")
	require.Equal(t, AcceptanceStateActive, again.State)

	live, err := r.FindLiveBuyerQuoteAcceptanceForRequest(ctx, testBuyerID, acceptRequestID)
	require.NoError(t, err)
	require.Equal(t, again.ID, live.ID, "live 查詢必須指向新那筆")
}

func TestRepo_ReleaseAcceptance_CannotReleasePaymentFinalized(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	a, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-1"))
	require.NoError(t, err)
	withOrder, err := r.AttachOrderToAcceptance(ctx, a.ID, a.Version, "ORD-1", refTime().Add(12*time.Hour))
	require.NoError(t, err)
	finalized, err := r.MarkAcceptancePaymentFinalized(ctx, withOrder.ID, withOrder.Version)
	require.NoError(t, err)
	require.Equal(t, AcceptanceStatePaymentFinalized, finalized.State)

	_, err = r.ReleaseAcceptance(ctx, finalized.ID, finalized.Version, AcceptanceStateCancelled)
	require.ErrorIs(t, err, ErrBuyerQuoteAcceptanceStale, "成交終態不得被釋放")

	second := acceptanceInput("accept-2", "req-hash-2")
	second.SellerResponseID = acceptSellerRespID + 1
	_, err = r.InsertBuyerQuoteAcceptance(ctx, second)
	require.ErrorIs(t, err, ErrRequestAlreadyHasLiveAcceptance)
}

func TestRepo_AttachOrderToAcceptance_SetsBothTogether(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	a, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-1"))
	require.NoError(t, err)

	deadline := refTime().Add(12 * time.Hour)
	withOrder, err := r.AttachOrderToAcceptance(ctx, a.ID, a.Version, "ORD-1", deadline)
	require.NoError(t, err)
	require.Equal(t, OrderProvisioningCreated, withOrder.OrderProvisioningState)
	require.NotNil(t, withOrder.OrderNo)
	require.Equal(t, "ORD-1", *withOrder.OrderNo)
	require.NotNil(t, withOrder.PaymentDeadline)

	_, err = r.AttachOrderToAcceptance(ctx, a.ID, a.Version, "ORD-2", deadline)
	require.ErrorIs(t, err, ErrBuyerQuoteAcceptanceStale)

	reread, err := r.FindBuyerQuoteAcceptanceByIDAndBuyer(ctx, a.ID, testBuyerID)
	require.NoError(t, err)
	require.Equal(t, "ORD-1", *reread.OrderNo, "重放不得改寫既有單號")
}

func TestRepo_AttachOrderToAcceptance_OrderNoIsUnique(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	first, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-1"))
	require.NoError(t, err)
	_, err = r.AttachOrderToAcceptance(ctx, first.ID, first.Version, "ORD-SHARED", refTime())
	require.NoError(t, err)

	other := acceptanceInput("accept-2", "req-hash-2")
	other.RequestID = acceptRequestID + 1
	second, err := r.InsertBuyerQuoteAcceptance(ctx, other)
	require.NoError(t, err)

	_, err = r.AttachOrderToAcceptance(ctx, second.ID, second.Version, "ORD-SHARED", refTime())
	require.Error(t, err, "同一個 orderNo 不得對應兩筆 acceptance")
}

func TestRepo_MarkAcceptanceOrderFailed_KeepsAcceptanceIntact(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	a, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-1"))
	require.NoError(t, err)

	failed, err := r.MarkAcceptanceOrderFailed(ctx, a.ID, a.Version, "ORDER_SERVICE_REJECTED")
	require.NoError(t, err)
	require.Equal(t, OrderProvisioningFailed, failed.OrderProvisioningState)
	require.Equal(t, AcceptanceStateActive, failed.State, "建單失敗不改變接受本身")
	require.EqualValues(t, 30800, failed.AcceptedDealPrice, "成交價仍然成立")
	require.Nil(t, failed.OrderNo)
	// doc 64 §6.4.14：失敗必須分得出成因，否則「order-service 拒絕」與「worker 自己
	// 出錯」在 acceptance 這一層長得一模一樣。
	require.NotNil(t, failed.OrderFailureReason)
	require.Equal(t, "ORDER_SERVICE_REJECTED", *failed.OrderFailureReason)
}

// 空成因會讓本欄退化成「有值但沒資訊」，那與沒有這一欄等價 ⇒ 擋在 repository 層。
func TestRepo_MarkAcceptanceOrderFailed_RejectsEmptyReason(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	a, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-1"))
	require.NoError(t, err)

	_, err = r.MarkAcceptanceOrderFailed(ctx, a.ID, a.Version, "")
	require.Error(t, err)

	// 仍是 CREATING_ORDER：擋下來的寫入不得留下半套狀態。
	unchanged, err := r.FindBuyerQuoteAcceptanceByIDAndBuyer(ctx, a.ID, testBuyerID)
	require.NoError(t, err)
	require.Equal(t, OrderProvisioningCreating, unchanged.OrderProvisioningState)
}

func TestRepo_MarkAcceptancePaymentFinalized_RequiresOrder(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	a, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-1"))
	require.NoError(t, err)

	_, err = r.MarkAcceptancePaymentFinalized(ctx, a.ID, a.Version)
	require.ErrorIs(t, err, ErrBuyerQuoteAcceptanceStale, "CREATING_ORDER 不得直接跳到 payment finalized")
}

func TestRepo_FindBuyerQuoteAcceptance_ScopedToBuyer(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	a, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-1"))
	require.NoError(t, err)

	_, err = r.FindBuyerQuoteAcceptanceByIDAndBuyer(ctx, a.ID, testBuyerID+1)
	require.ErrorIs(t, err, ErrBuyerQuoteAcceptanceNotFound, "別的買家讀不到，且與 not-found 同一種外形")

	_, err = r.FindLiveBuyerQuoteAcceptanceForRequest(ctx, testBuyerID+1, acceptRequestID)
	require.ErrorIs(t, err, ErrBuyerQuoteAcceptanceNotFound)
}

func TestRepo_FindLiveAcceptance_NoneIsNotAnError(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	_, err := r.FindLiveBuyerQuoteAcceptanceForRequest(ctx, testBuyerID, acceptRequestID)
	require.ErrorIs(t, err, ErrBuyerQuoteAcceptanceNotFound)

	_, err = r.FindBuyerQuoteAcceptanceByIdempotencyKey(ctx, testBuyerID, "never-used")
	require.ErrorIs(t, err, ErrBuyerQuoteAcceptanceNotFound)
}

func TestRepo_ReleaseAcceptance_RejectsArbitraryState(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	a, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("accept-1", "req-hash-1"))
	require.NoError(t, err)

	_, err = r.ReleaseAcceptance(ctx, a.ID, a.Version, AcceptanceStatePaymentFinalized)
	require.Error(t, err)

	unchanged, err := r.FindBuyerQuoteAcceptanceByIDAndBuyer(ctx, a.ID, testBuyerID)
	require.NoError(t, err)
	require.Equal(t, AcceptanceStateActive, unchanged.State)
}
