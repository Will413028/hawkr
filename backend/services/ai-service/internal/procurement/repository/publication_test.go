//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
)

func seedPublicationCommandInput(ctx context.Context, t *testing.T, repo *Repo) ClaimPublicationCommandInput {
	t.Helper()
	approval, err := repo.InsertPublicationApproval(ctx, seedApprovalInput(ctx, t, repo))
	require.NoError(t, err)
	return ClaimPublicationCommandInput{
		BuyerID: testBuyerID, RequestID: approval.RequestID,
		ApprovalID: approval.ID, ApprovalVersion: approval.Version,
		ClientRequestID: "exec-key-1", CanonicalRequestHash: "canon-hash-1",
	}
}

// 第一次 claim 是新的，第二次同 key 同內容是冪等命中（不建第二列）。
func TestClaimPublicationCommand_IsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	in := seedPublicationCommandInput(ctx, t, repo)
	first, isNew, err := repo.ClaimPublicationCommand(ctx, in)
	require.NoError(t, err)
	require.True(t, isNew)

	second, isNew, err := repo.ClaimPublicationCommand(ctx, in)
	require.NoError(t, err)
	require.False(t, isNew, "同 key 同內容必須是冪等命中，不是新 claim")
	require.Equal(t, first.ID, second.ID)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_rfq_publication_commands WHERE buyer_id = $1`,
		int64(testBuyerID)).Scan(&count))
	require.Equal(t, 1, count)
}

// 同 key 不同內容是衝突，零 mutation（Scope 7）。
func TestClaimPublicationCommand_SameKeyDifferentHashConflicts(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	in := seedPublicationCommandInput(ctx, t, repo)
	_, _, err := repo.ClaimPublicationCommand(ctx, in)
	require.NoError(t, err)

	drifted := in
	drifted.CanonicalRequestHash = "canon-hash-DIFFERENT"
	_, _, err = repo.ClaimPublicationCommand(ctx, drifted)
	require.ErrorIs(t, err, ErrPublicationCommandConflict)

	var count int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_rfq_publication_commands WHERE buyer_id = $1 AND client_request_id = $2`,
		int64(testBuyerID), in.ClientRequestID).Scan(&count))
	require.Equal(t, 1, count, "conflict 不得建立第二筆")
}

// 一個 command 只能有一張 publication——DB 唯一索引是最終防線。
func TestInsertPublication_OneCommandOnePublication(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	in := seedPublicationCommandInput(ctx, t, repo)
	cmd, _, err := repo.ClaimPublicationCommand(ctx, in)
	require.NoError(t, err)

	pubIn := InsertPublicationInput{
		BuyerID: testBuyerID, RequestID: cmd.RequestID, RequestRevisionID: 1,
		ApprovalID: cmd.ApprovalID, ApprovalVersion: cmd.ApprovalVersion,
		CommandID: cmd.ID, ConfirmedRfqSnapshotID: 1,
		SellerFacingPayloadHash: "sfp", AudienceMode: "WISH_POOL", AudienceHash: "aud",
		State: "PUBLISHED", SchemaVersion: "hawkr.agent.publication.v1",
		PublishedAt: refTime(),
	}
	got, err := repo.InsertPublication(ctx, pubIn)
	require.NoError(t, err)
	require.Equal(t, 1, got.Version)

	_, err = repo.InsertPublication(ctx, pubIn)
	require.Error(t, err, "同一個 command 不得建出第二張 publication")
}
