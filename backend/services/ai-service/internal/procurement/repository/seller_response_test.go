//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
)

const (
	testPublicationID = int64(9100)
	testRequestID     = int64(9200)
	testStoreID       = int64(9300)
	testProspectID    = int64(9400)
)

func newSellerResponseRepo(t *testing.T) (*Repo, context.Context) {
	t.Helper()
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	t.Cleanup(cleanup)
	return New(pool, database.NewTxManager(pool), testIDGen()), ctx
}

func validVersionInput(sellerResponseID int64, version int32, clientRequestID, payloadHash string) InsertSellerResponseVersionInput {
	return InsertSellerResponseVersionInput{
		SellerResponseID:              sellerResponseID,
		Version:                       version,
		ResponseType:                  "MANUAL_QUOTE",
		OfferedQualityGrade:           "A 級",
		OfferedQualityInspectionNotes: "外觀無壓傷，冷鏈配送",
		UnitBasisQuantity:             "1",
		UnitBasisUnit:                 "BOX",
		OfferedQuantity:               "30",
		OfferedQuantityUnit:           "BOX",
		ConversionBasis:               "1 BOX = 10 KG",
		Currency:                      "TWD",
		GoodsAmount:                   30000,
		ShippingAmount:                800,
		TaxAmount:                     0,
		TaxInclusionBasis:             "TAX_INCLUDED",
		TotalAmount:                   30800,
		ValidUntil:                    refTime().Add(48 * time.Hour),
		TermsNote:                     "冷藏配送",
		PayloadHash:                   payloadHash,
		PublicationVersion:            1,
		SellerFacingPayloadHash:       "sfp-1",
		ClientRequestID:               clientRequestID,
		SubmittedAt:                   refTime(),
	}
}

func TestRepo_EnsureSellerResponseIdentityForStore_IsIdempotent(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)

	first, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)
	require.Equal(t, SellerResponseLanePlatform, first.Lane)
	require.NotNil(t, first.StoreID)
	require.Nil(t, first.ProspectID)
	require.EqualValues(t, 0, first.LatestVersion, "剛建立時還沒有正式回價")
	require.False(t, first.HasSubmission())

	second, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID, "同一個 (publication, store) 必須回同一列")
}

func TestRepo_EnsureSellerResponseIdentity_LanesAreIndependent(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)

	platform, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)
	external, err := r.EnsureSellerResponseIdentityForProspect(ctx, testPublicationID, testBuyerID, testRequestID, testProspectID)
	require.NoError(t, err)

	require.NotEqual(t, platform.ID, external.ID)
	require.Equal(t, SellerResponseLaneExternal, external.Lane)
	require.Nil(t, external.StoreID)
	require.NotNil(t, external.ProspectID)

	other, err := r.EnsureSellerResponseIdentityForProspect(ctx, testPublicationID, testBuyerID, testRequestID, testProspectID+1)
	require.NoError(t, err)
	require.NotEqual(t, external.ID, other.ID)
}

func TestRepo_FindSellerResponseByIdentity_WrongOwnerLooksLikeNotFound(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)

	created, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	found, err := r.FindSellerResponseByStoreIdentity(ctx, testPublicationID, testStoreID)
	require.NoError(t, err)
	require.Equal(t, created.ID, found.ID)

	_, err = r.FindSellerResponseByStoreIdentity(ctx, testPublicationID, testStoreID+1)
	require.ErrorIs(t, err, ErrSellerResponseNotFound)
}

func TestRepo_UpsertSellerResponseDraft_CASAndStale(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	first, err := r.UpsertSellerResponseDraft(ctx, id.ID, json.RawMessage(`{"goodsAmount":"30000"}`), 0)
	require.NoError(t, err)
	require.EqualValues(t, 1, first.Version)

	second, err := r.UpsertSellerResponseDraft(ctx, id.ID, json.RawMessage(`{"goodsAmount":"31000"}`), 1)
	require.NoError(t, err)
	require.EqualValues(t, 2, second.Version)
	require.JSONEq(t, `{"goodsAmount":"31000"}`, string(second.Payload))

	_, err = r.UpsertSellerResponseDraft(ctx, id.ID, json.RawMessage(`{"goodsAmount":"32000"}`), 1)
	require.ErrorIs(t, err, ErrSellerResponseDraftStale, "帶著過期的 draft version 必須被擋下")
	require.False(t, errors.Is(err, ErrSellerResponseStale),
		"草稿的 CAS 衝突不得與正式回價的 stale 共用同一個錯誤——兩者給使用者的話不一樣")

	current, err := r.FindSellerResponseDraft(ctx, id.ID)
	require.NoError(t, err)
	require.JSONEq(t, `{"goodsAmount":"31000"}`, string(current.Payload))
}

func TestRepo_SellerResponseDraft_JSONBRoundTrip(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	payload := json.RawMessage(`{"offeredQuality":{"grade":"A 級"},"unitBasis":{"quantity":"1","unit":"BOX"}}`)
	_, err = r.UpsertSellerResponseDraft(ctx, id.ID, payload, 0)
	require.NoError(t, err)

	got, err := r.FindSellerResponseDraft(ctx, id.ID)
	require.NoError(t, err)
	require.JSONEq(t, string(payload), string(got.Payload))
}

func TestRepo_InsertSellerResponseVersion_ThenBump(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	in := validVersionInput(id.ID, 1, "submit-1", "hash-v1")
	in.UnitBasisQuantity = "1.500"
	in.OfferedQuantity = "30.250"
	v, err := r.InsertSellerResponseVersion(ctx, in)
	require.NoError(t, err)
	require.EqualValues(t, 1, v.Version)
	require.Equal(t, "1.5", v.UnitBasisQuantity,
		"投影層去尾零：NUMERIC(14,3) 的 scale 是儲存細節，不得洩到 wire 上（cbe0e4401）")
	require.Equal(t, "30.25", v.OfferedQuantity)
	require.EqualValues(t, 30800, v.TotalAmount)

	bumped, err := r.BumpSellerResponseLatestVersion(ctx, id.ID, id.Version, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, bumped.LatestVersion)
	require.True(t, bumped.HasSubmission())
	require.Greater(t, bumped.Version, id.Version, "樂觀鎖必須往前走")
}

func TestRepo_InsertSellerResponseVersion_SameKeySamePayloadReplays(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	first, err := r.InsertSellerResponseVersion(ctx, validVersionInput(id.ID, 1, "submit-1", "hash-v1"))
	require.NoError(t, err)
	replay, err := r.InsertSellerResponseVersion(ctx, validVersionInput(id.ID, 1, "submit-1", "hash-v1"))
	require.NoError(t, err)
	require.Equal(t, first.ID, replay.ID, "同鍵同內容必須回放同一列")

	all, err := r.ListSellerResponseVersions(ctx, id.ID)
	require.NoError(t, err)
	require.Len(t, all, 1, "回放不得建立第二筆版本")
}

func TestRepo_InsertSellerResponseVersion_SameKeyDifferentPayloadConflicts(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	_, err = r.InsertSellerResponseVersion(ctx, validVersionInput(id.ID, 1, "submit-1", "hash-v1"))
	require.NoError(t, err)

	_, err = r.InsertSellerResponseVersion(ctx, validVersionInput(id.ID, 1, "submit-1", "hash-DIFFERENT"))
	require.ErrorIs(t, err, ErrSellerResponseIdempotencyConflict)

	all, err := r.ListSellerResponseVersions(ctx, id.ID)
	require.NoError(t, err)
	require.Len(t, all, 1, "衝突必須零 mutation")
	require.Equal(t, "hash-v1", all[0].PayloadHash)
}

func TestRepo_ReviseKeepsOldVersionIntact(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	v1, err := r.InsertSellerResponseVersion(ctx, validVersionInput(id.ID, 1, "submit-1", "hash-v1"))
	require.NoError(t, err)
	afterV1, err := r.BumpSellerResponseLatestVersion(ctx, id.ID, id.Version, 1)
	require.NoError(t, err)

	revised := validVersionInput(id.ID, 2, "submit-2", "hash-v2")
	revised.GoodsAmount = 29000
	revised.TotalAmount = 29800
	v2, err := r.InsertSellerResponseVersion(ctx, revised)
	require.NoError(t, err)
	_, err = r.BumpSellerResponseLatestVersion(ctx, id.ID, afterV1.Version, 2)
	require.NoError(t, err)

	reread, err := r.FindSellerResponseVersion(ctx, id.ID, 1)
	require.NoError(t, err)
	require.Equal(t, v1.PayloadHash, reread.PayloadHash)
	require.EqualValues(t, 30000, reread.GoodsAmount, "舊版金額不得被改版改寫")
	require.EqualValues(t, 29000, v2.GoodsAmount)

	all, err := r.ListSellerResponseVersions(ctx, id.ID)
	require.NoError(t, err)
	require.Len(t, all, 2)
	require.EqualValues(t, 1, all[0].Version, "ListSellerResponseVersions 必須依版本序")
	require.EqualValues(t, 2, all[1].Version)
}

func TestRepo_InsertSellerResponseVersion_DuplicateVersionRejected(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	_, err = r.InsertSellerResponseVersion(ctx, validVersionInput(id.ID, 1, "submit-1", "hash-v1"))
	require.NoError(t, err)

	_, err = r.InsertSellerResponseVersion(ctx, validVersionInput(id.ID, 1, "submit-OTHER", "hash-other"))
	require.Error(t, err, "同一個 (seller_response_id, version) 不得有第二列")
}

func TestRepo_WithdrawSellerResponse_DoesNotRewriteVersions(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	v1, err := r.InsertSellerResponseVersion(ctx, validVersionInput(id.ID, 1, "submit-1", "hash-v1"))
	require.NoError(t, err)
	afterBump, err := r.BumpSellerResponseLatestVersion(ctx, id.ID, id.Version, 1)
	require.NoError(t, err)

	withdrawn, err := r.WithdrawSellerResponse(ctx, id.ID, afterBump.Version, 1)
	require.NoError(t, err)
	require.NotNil(t, withdrawn.WithdrawnVersion)
	require.EqualValues(t, 1, *withdrawn.WithdrawnVersion)
	require.NotNil(t, withdrawn.WithdrawnAt)

	reread, err := r.FindSellerResponseVersion(ctx, id.ID, 1)
	require.NoError(t, err)
	require.Equal(t, v1.PayloadHash, reread.PayloadHash, "撤回不得改寫版本列")
	require.Equal(t, v1.SubmittedAt, reread.SubmittedAt)
}

func TestRepo_WithdrawSellerResponse_WrongLatestVersionIsStale(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	_, err = r.InsertSellerResponseVersion(ctx, validVersionInput(id.ID, 1, "submit-1", "hash-v1"))
	require.NoError(t, err)
	afterBump, err := r.BumpSellerResponseLatestVersion(ctx, id.ID, id.Version, 1)
	require.NoError(t, err)

	_, err = r.WithdrawSellerResponse(ctx, id.ID, afterBump.Version, 2)
	require.ErrorIs(t, err, ErrSellerResponseStale)

	unchanged, err := r.FindSellerResponseByID(ctx, id.ID)
	require.NoError(t, err)
	require.Nil(t, unchanged.WithdrawnVersion, "被擋下就不得留下任何撤回痕跡")
}

func TestRepo_ResubmitAfterWithdrawal(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	_, err = r.InsertSellerResponseVersion(ctx, validVersionInput(id.ID, 1, "submit-1", "hash-v1"))
	require.NoError(t, err)
	afterBump, err := r.BumpSellerResponseLatestVersion(ctx, id.ID, id.Version, 1)
	require.NoError(t, err)
	withdrawn, err := r.WithdrawSellerResponse(ctx, id.ID, afterBump.Version, 1)
	require.NoError(t, err)

	_, err = r.InsertSellerResponseVersion(ctx, validVersionInput(id.ID, 2, "submit-2", "hash-v2"))
	require.NoError(t, err)
	after, err := r.BumpSellerResponseLatestVersion(ctx, id.ID, withdrawn.Version, 2)
	require.NoError(t, err)

	require.EqualValues(t, 2, after.LatestVersion)
	require.NotNil(t, after.WithdrawnVersion)
	require.EqualValues(t, 1, *after.WithdrawnVersion, "舊的撤回事實留著，指的仍是第 1 版")
}

func TestRepo_CloseSellerResponseWrite_BlocksFurtherWrites(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	closed, err := r.CloseSellerResponseWrite(ctx, id.ID, id.Version, SellerResponseClosedDeclined)
	require.NoError(t, err)
	require.True(t, closed.WriteClosed())
	require.NotNil(t, closed.WriteClosedReason)
	require.Equal(t, SellerResponseClosedDeclined, *closed.WriteClosedReason)

	_, err = r.InsertSellerResponseVersion(ctx, validVersionInput(id.ID, 1, "submit-1", "hash-v1"))
	require.NoError(t, err, "版本列本身沒有 write-closed 述詞，守門在 bump")

	_, err = r.BumpSellerResponseLatestVersion(ctx, id.ID, closed.Version, 1)
	require.ErrorIs(t, err, ErrSellerResponseWriteClosed, "已 decline 的賣家不得推進 latest_version")

	_, err = r.CloseSellerResponseWrite(ctx, id.ID, closed.Version, SellerResponseClosedDeclined)
	require.ErrorIs(t, err, ErrSellerResponseWriteClosed)
}

func TestRepo_BumpSellerResponseLatestVersion_StaleVsClosed(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	_, err = r.InsertSellerResponseVersion(ctx, validVersionInput(id.ID, 1, "submit-1", "hash-v1"))
	require.NoError(t, err)

	_, err = r.BumpSellerResponseLatestVersion(ctx, id.ID, id.Version+99, 1)
	require.ErrorIs(t, err, ErrSellerResponseStale, "版本不符是 stale，不是 closed")
	require.False(t, errors.Is(err, ErrSellerResponseWriteClosed))
}

func TestRepo_FindSellerResponseVersion_NotFound(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	_, err = r.FindSellerResponseVersion(ctx, id.ID, 7)
	require.ErrorIs(t, err, ErrSellerResponseNotFound)

	all, err := r.ListSellerResponseVersions(ctx, id.ID)
	require.NoError(t, err)
	require.Empty(t, all, "沒有任何版本時回空切片而不是錯誤")
}

func TestRepo_InsertSellerResponseVersion_TotalMustBeSum(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	bad := validVersionInput(id.ID, 1, "submit-1", "hash-v1")
	bad.TotalAmount = 99999
	_, err = r.InsertSellerResponseVersion(ctx, bad)
	require.Error(t, err, "total ≠ goods + shipping + tax 必須被 CHECK 擋下")
}

func TestRepo_InsertSellerResponseVersion_RejectsUnsafeNumeric(t *testing.T) {
	r, ctx := newSellerResponseRepo(t)
	id, err := r.EnsureSellerResponseIdentityForStore(ctx, testPublicationID, testBuyerID, testRequestID, testStoreID)
	require.NoError(t, err)

	for _, bad := range []string{"3e2", "NaN", "Infinity", ""} {
		in := validVersionInput(id.ID, 1, "submit-"+bad, "hash-"+bad)
		in.OfferedQuantity = bad
		_, err := r.InsertSellerResponseVersion(ctx, in)
		require.Error(t, err, "OfferedQuantity=%q 必須被擋在寫入之前", bad)
	}

	all, err := r.ListSellerResponseVersions(ctx, id.ID)
	require.NoError(t, err)
	require.Empty(t, all, "被擋下的輸入不得留下任何列")
}
