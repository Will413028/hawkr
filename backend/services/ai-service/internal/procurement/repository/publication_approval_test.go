//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

func int64p(i int64) *int64 { return &i }

func seedApprovalInput(ctx context.Context, t *testing.T, repo *Repo) InsertPublicationApprovalInput {
	t.Helper()

	req, rev, err := repo.CreateRequest(ctx, testBuyerID)
	require.NoError(t, err)

	preview, err := repo.InsertPricePreview(ctx, InsertPricePreviewInput{
		BuyerID: testBuyerID, RequestID: req.ID, RequestRevisionID: rev.ID,
		PricingMode: "PRICEBOOK_GUIDED", QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
		ReferenceAvailability:   "AVAILABLE",
		BuyerRequestedRangeJSON: json.RawMessage(`{"min":"80","max":"120"}`),
		DealTemperatureJSON:     json.RawMessage(`{"status":"UNKNOWN"}`),
		ClientRequestID:         "seed-preview",
		PreviewInputHash:        "seed-input-hash",
		PayloadHash:             "seed-payload-hash",
		ExpiresAt:               refTime().Add(time.Hour),
	})
	require.NoError(t, err)

	canonical := json.RawMessage(`{"qualityKnowledgeState":"BUYER_QUALITY_UNKNOWN","pricingMode":"PRICEBOOK_GUIDED","buyerRequestedRange":{"min":"80","max":"120"},"item":"高麗菜","quantity":100,"unit":"公斤","deliveryCity":"台中市"}`)
	snap, err := repo.InsertConfirmedRfqSnapshot(ctx, InsertConfirmedRfqSnapshotInput{
		BuyerID: testBuyerID, RequestID: req.ID, RequestRevisionID: rev.ID,
		PricePreviewID: preview.ID, PricePreviewVersion: preview.Version,
		QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN", PricingMode: "PRICEBOOK_GUIDED",
		CanonicalPayloadJSON: canonical, PayloadHash: "seed-snapshot-hash",
		ClientRequestID: "seed-snapshot", ExpiresAt: refTime().Add(time.Hour),
	})
	require.NoError(t, err)

	return InsertPublicationApprovalInput{
		BuyerID: testBuyerID, RequestID: req.ID, RequestRevisionID: rev.ID,
		ConfirmedRfqSnapshotID: snap.ID, ConfirmedRfqPayloadHash: snap.PayloadHash,
		PricePreviewID: preview.ID, PricePreviewVersion: preview.Version,
		PricePreviewPayloadHash: preview.PayloadHash, PricePreviewInputHash: preview.PreviewInputHash,
		QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN", PricingMode: "PRICEBOOK_GUIDED",
		ReferenceAvailability: "AVAILABLE", BuyerRequestedRangeHash: "seed-range-hash",
		AudienceMode: "DIRECT_RECIPIENTS", AudiencePolicyID: "", AudiencePolicyVersion: 0,
		AudienceHash:   "seed-audience-hash",
		CandidateSetID: int64p(900), CandidateSetPolicyVersion: ptr(1),
		SellerFacingPayloadJSON: json.RawMessage(`{"item":"高麗菜"}`), SellerFacingPayloadHash: "seed-sfp-hash",
		DealTemperatureStatus: "UNKNOWN", State: "PENDING",
		SchemaVersion:          "hawkr.agent.publication-approval.v1",
		PreviewClientRequestID: "seed-approval",
		GeneratedAt:            refTime(), ExpiresAt: refTime().Add(time.Hour),
	}
}

func TestInsertPublicationApproval_IsIdempotentOnPreviewKey(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	in := seedApprovalInput(ctx, t, repo)
	first, err := repo.InsertPublicationApproval(ctx, in)
	require.NoError(t, err)
	require.NotZero(t, first.ID)
	require.Equal(t, 1, first.Version, "新建的 approval version 從 1 起")
	require.Equal(t, "PENDING", first.State)

	second, err := repo.InsertPublicationApproval(ctx, in)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID, "同 key 重送必須回同一列")

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_rfq_publication_approvals WHERE buyer_id = $1`,
		int64(testBuyerID)).Scan(&count))
	require.Equal(t, 1, count, "重送不得建立第二列")
}

func TestInsertPublicationApproval_SameKeyDifferentPayloadConflicts(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	in := seedApprovalInput(ctx, t, repo)
	first, err := repo.InsertPublicationApproval(ctx, in)
	require.NoError(t, err)

	drifted := in
	drifted.AudienceHash = "0000000000000000000000000000000000000000000000000000000000000000"
	_, err = repo.InsertPublicationApproval(ctx, drifted)
	require.ErrorIs(t, err, ErrPublicationApprovalIdempotencyConflict)

	var stored string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT audience_hash FROM procurement_rfq_publication_approvals WHERE id = $1`,
		first.ID).Scan(&stored))
	require.Equal(t, in.AudienceHash, stored)
}

func TestInsertPublicationApproval_SameKeyDifferentSellerPayloadConflicts(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	in := seedApprovalInput(ctx, t, repo)
	_, err := repo.InsertPublicationApproval(ctx, in)
	require.NoError(t, err)

	drifted := in
	drifted.SellerFacingPayloadHash = "1111111111111111111111111111111111111111111111111111111111111111"
	_, err = repo.InsertPublicationApproval(ctx, drifted)
	require.ErrorIs(t, err, ErrPublicationApprovalIdempotencyConflict)
}

func TestInsertPublicationApproval_WishPoolRejectsCandidateSet(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	in := seedApprovalInput(ctx, t, repo)
	in.AudienceMode = "WISH_POOL"
	in.AudiencePolicyID = "WISH_POOL_ALL_ACTIVE_SELLERS_V1"
	in.AudiencePolicyVersion = 1
	_, err := repo.InsertPublicationApproval(ctx, in)
	require.Error(t, err)
}

func TestInsertPublicationApproval_DirectRequiresCandidateSet(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	in := seedApprovalInput(ctx, t, repo)
	in.CandidateSetID = nil
	in.CandidateSetPolicyVersion = nil
	_, err := repo.InsertPublicationApproval(ctx, in)
	require.Error(t, err)
}

func TestInsertPublicationApproval_WishPoolRoundTrips(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	in := seedApprovalInput(ctx, t, repo)
	in.AudienceMode = "WISH_POOL"
	in.AudiencePolicyID = "WISH_POOL_ALL_ACTIVE_SELLERS_V1"
	in.AudiencePolicyVersion = 1
	in.CandidateSetID = nil
	in.CandidateSetPolicyVersion = nil
	in.ResponseDeadline = ptr("2026-09-15")

	got, err := repo.InsertPublicationApproval(ctx, in)
	require.NoError(t, err)
	require.Nil(t, got.CandidateSetID)
	require.Nil(t, got.CandidateSetPolicyVersion)
	require.Equal(t, "WISH_POOL_ALL_ACTIVE_SELLERS_V1", got.AudiencePolicyID)
	require.Equal(t, 1, got.AudiencePolicyVersion)
	require.NotNil(t, got.ResponseDeadline)
	require.Equal(t, "2026-09-15", *got.ResponseDeadline)

	found, err := repo.FindPublicationApprovalByIdempotencyKey(ctx, testBuyerID, in.PreviewClientRequestID)
	require.NoError(t, err)
	require.Equal(t, got.ID, found.ID)
	require.Equal(t, got.Version, found.Version)
	require.JSONEq(t, string(in.SellerFacingPayloadJSON), string(found.SellerFacingPayloadJSON))
}

func TestFindPublicationApprovalByIdempotencyKey_NotFound(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	_, err := repo.FindPublicationApprovalByIdempotencyKey(ctx, testBuyerID, "never-seen")
	require.ErrorIs(t, err, ErrPublicationApprovalNotFound)
}

func seedDirectRecipientsApprovalInput(ctx context.Context, t *testing.T, repo *Repo, recipients []domain.PublicationRecipient) InsertPublicationApprovalInput {
	t.Helper()

	in := seedApprovalInput(ctx, t, repo)
	require.NotNil(t, in.CandidateSetID)
	require.NotNil(t, in.CandidateSetPolicyVersion)

	audience := domain.DirectRecipientsAudience(*in.CandidateSetID, *in.CandidateSetPolicyVersion, recipients)
	hash, err := audience.Hash()
	require.NoError(t, err)

	in.Recipients = recipients
	in.AudienceHash = hash
	in.ExpiresAt = time.Now().UTC().Add(time.Hour)
	return in
}

func TestInsertPublicationApproval_PersistsApprovalRecipientsInOrder(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	recipients := []domain.PublicationRecipient{
		domain.PlatformRecipient(501),
		domain.ExternalRecipient(9001),
		domain.PlatformRecipient(502),
	}
	in := seedDirectRecipientsApprovalInput(ctx, t, repo, recipients)

	approval, err := repo.InsertPublicationApproval(ctx, in)
	require.NoError(t, err)

	got, err := repo.ListPublicationApprovalRecipients(ctx, approval.ID)
	require.NoError(t, err)
	require.Equal(t, recipients, got, "順序與 lane／身分必須逐項相同，不是集合比對")
}

func TestInsertPublicationApproval_WishPoolHasNoApprovalRecipients(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	in := seedApprovalInput(ctx, t, repo)
	in.AudienceMode = "WISH_POOL"
	in.AudiencePolicyID = "WISH_POOL_ALL_ACTIVE_SELLERS_V1"
	in.AudiencePolicyVersion = 1
	in.CandidateSetID = nil
	in.CandidateSetPolicyVersion = nil

	approval, err := repo.InsertPublicationApproval(ctx, in)
	require.NoError(t, err)

	got, err := repo.ListPublicationApprovalRecipients(ctx, approval.ID)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestClaimPublicationApprovalForExecute_AdvancesVersion(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	created, err := repo.InsertPublicationApproval(ctx, seedApprovalInput(ctx, t, repo))
	require.NoError(t, err)
	require.Equal(t, 1, created.Version)

	claimed, err := repo.ClaimPublicationApprovalForExecute(ctx, created.ID, 1, testBuyerID)
	require.NoError(t, err)
	require.Equal(t, 2, claimed.Version, "狀態轉移必須推進 version")
	require.Equal(t, "EXECUTING", claimed.State)
}

func TestClaimPublicationApprovalForExecute_StaleVersionIsRejected(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	created, err := repo.InsertPublicationApproval(ctx, seedApprovalInput(ctx, t, repo))
	require.NoError(t, err)

	_, err = repo.ClaimPublicationApprovalForExecute(ctx, created.ID, 1, testBuyerID)
	require.NoError(t, err)

	_, err = repo.ClaimPublicationApprovalForExecute(ctx, created.ID, 1, testBuyerID)
	require.ErrorIs(t, err, ErrPublicationApprovalCASFailed)
}

func TestClaimPublicationApprovalForExecute_VersionDivergesWhileStatePendingIsRejected(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	created, err := repo.InsertPublicationApproval(ctx, seedApprovalInput(ctx, t, repo))
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`UPDATE procurement_rfq_publication_approvals SET version = version + 1 WHERE id = $1`,
		created.ID)
	require.NoError(t, err)

	var state string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT state FROM procurement_rfq_publication_approvals WHERE id = $1`,
		created.ID).Scan(&state))
	require.Equal(t, "PENDING", state, "state 必須維持 PENDING——否則這條測試又會退化成只驗 state")

	_, err = repo.ClaimPublicationApprovalForExecute(ctx, created.ID, created.Version, testBuyerID)
	require.ErrorIs(t, err, ErrPublicationApprovalCASFailed)
}

func TestClaimPublicationApprovalForExecute_WrongBuyerIsRejected(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	created, err := repo.InsertPublicationApproval(ctx, seedApprovalInput(ctx, t, repo))
	require.NoError(t, err)

	_, err = repo.ClaimPublicationApprovalForExecute(ctx, created.ID, 1, identity.BuyerID(999999))
	require.ErrorIs(t, err, ErrPublicationApprovalCASFailed)
}

func TestMarkPublicationApprovalSucceeded_AdvancesVersion(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	created, err := repo.InsertPublicationApproval(ctx, seedApprovalInput(ctx, t, repo))
	require.NoError(t, err)
	claimed, err := repo.ClaimPublicationApprovalForExecute(ctx, created.ID, 1, testBuyerID)
	require.NoError(t, err)

	require.NoError(t, repo.MarkPublicationApprovalSucceeded(ctx, created.ID, claimed.Version))

	found, err := repo.FindPublicationApprovalByIdempotencyKey(ctx, testBuyerID, "seed-approval")
	require.NoError(t, err)
	require.Equal(t, "SUCCEEDED", found.State)
	require.Equal(t, 3, found.Version)
}
