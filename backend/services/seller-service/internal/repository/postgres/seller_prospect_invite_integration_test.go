//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

// 撤銷後 FindUsableInviteByToken 必須查不到——「可用」的定義只有一份，
// 寫在 SQL 裡（未撤銷、未消費、未過期）。
func TestRevokeInviteMakesTokenUnusable(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	p, err := repo.Create(ctx, newTestSellerProspect("受邀業者", "水產", "高雄"))
	require.NoError(t, err)

	invite, err := repo.CreateInvite(ctx, p.ID, "tok-revoke-test", time.Now().Add(30*24*time.Hour))
	require.NoError(t, err)

	found, err := repo.FindUsableInviteByToken(ctx, "tok-revoke-test")
	require.NoError(t, err)
	require.Equal(t, invite.ID, found.ID)

	rows, err := repo.RevokeInvite(ctx, p.ID, invite.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, rows)

	_, err = repo.FindUsableInviteByToken(ctx, "tok-revoke-test")
	require.Error(t, err, "撤銷後不得再被視為可用")
}

// 撤銷必須同時比對 prospect_id：只憑 invite_id 會讓 admin 從 A 的詳情頁
// 撤銷掉 B 的邀請。
func TestRevokeInviteScopedToProspect(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	a, err := repo.Create(ctx, newTestSellerProspect("業者甲", "水產", "高雄"))
	require.NoError(t, err)
	b, err := repo.Create(ctx, newTestSellerProspect("業者乙", "水產", "高雄"))
	require.NoError(t, err)

	invite, err := repo.CreateInvite(ctx, b.ID, "tok-scope-test", time.Now().Add(24*time.Hour))
	require.NoError(t, err)

	rows, err := repo.RevokeInvite(ctx, a.ID, invite.ID)
	require.NoError(t, err)
	require.EqualValues(t, 0, rows, "不得跨 prospect 撤銷")
}
