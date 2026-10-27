//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

// chk_vps_assurance 的值域契約。
//
// 🔴 這是 expand-only 那一刀的驗收：本刀**沒有任何寫入者**會寫 GOOGLE_VERIFIED，
// 所以「放寬有沒有生效」在單元測試與服務行為上都看不出來——只有直接對 CHECK
// 下 INSERT 才驗得到。少了它，下一刀的寫入端會在部署當下才撞 23514。
//
// ⚠️ 同時守反向：放寬不等於放行任何字串。第三條斷言存在的理由是「把 CHECK 改成
// 只檢查非空」這種等價放寬會讓前兩條照樣綠。
func TestVendorPortalAssuranceValueDomain(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	identityID := seedVendorPortalIdentity(ctx, t, pool)
	inviteID := seedVendorPortalInvite(ctx, t, pool, identityID)

	t.Run("INVITE_ONLY 仍可寫入", func(t *testing.T) {
		require.NoError(t, insertSession(ctx, pool, 910000001, identityID, inviteID, "INVITE_ONLY"))
	})

	t.Run("GOOGLE_VERIFIED 可寫入（本刀放寬的那一個）", func(t *testing.T) {
		require.NoError(t, insertSession(ctx, pool, 910000002, identityID, inviteID, "GOOGLE_VERIFIED"))
	})

	t.Run("⛔ 未知值仍被擋", func(t *testing.T) {
		err := insertSession(ctx, pool, 910000003, identityID, inviteID, "EMAIL_VERIFIED")
		require.Error(t, err, "放寬值域不等於放行任何字串——CHECK 必須仍是正面列舉")
		require.Contains(t, err.Error(), "chk_vps_assurance")
	})
}

func seedVendorPortalIdentity(ctx context.Context, t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())
	prospect, err := repo.Create(ctx, newTestSellerProspect("站外業者-assurance 測試", "蔬菜", "台南"))
	require.NoError(t, err)

	const id = int64(900000001)
	_, err = pool.Exec(ctx,
		`INSERT INTO vendor_portal_identities (id, prospect_id, invited_email) VALUES ($1, $2, $3)`,
		id, prospect.ID, "vendor-assurance@example.com")
	require.NoError(t, err)
	return id
}

func seedVendorPortalInvite(ctx context.Context, t *testing.T, pool *pgxpool.Pool, identityID int64) int64 {
	t.Helper()
	const id = int64(905000001)
	_, err := pool.Exec(ctx,
		`INSERT INTO vendor_portal_invites (id, identity_id, rfq_recipient_id, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		id, identityID, int64(906000001), fmt.Sprintf("%064d", 1), time.Now().Add(72*time.Hour))
	require.NoError(t, err)
	return id
}

func insertSession(ctx context.Context, pool *pgxpool.Pool, id, identityID, inviteID int64, assurance string) error {
	now := time.Now()
	_, err := pool.Exec(ctx,
		`INSERT INTO vendor_portal_sessions
		   (id, session_hash, identity_id, invite_id, assurance, idle_expires_at, absolute_expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, fmt.Sprintf("%064d", id), identityID, inviteID, assurance,
		now.Add(24*time.Hour), now.Add(7*24*time.Hour))
	return err
}

// widenAssuranceMigration 是本刀那一支 migration 的檔名。
const widenAssuranceMigration = "20260921125356_widen_vendor_portal_assurance.sql"

// 排除本刀的 migration 之後，GOOGLE_VERIFIED 必須寫不進去。
//
// 🔴 沒有這一支，上面那三條在**舊 schema 上也可能全綠**——只要哪天有人把 CHECK
// 改成別的等價寬鬆形式，或 migration 根本沒被套用，都看不出來。它證明的是
// 「放寬確實來自這支 migration」，而不只是「現在的 schema 剛好接受」。
//
// ⚠️ 依 dbtest 的註解，每一組不同的排除集合各多一個容器，所以只留這一條。
func TestVendorPortalAssuranceRejectsGoogleVerifiedBeforeMigration(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t, widenAssuranceMigration)
	defer cleanup()

	identityID := seedVendorPortalIdentity(ctx, t, pool)
	inviteID := seedVendorPortalInvite(ctx, t, pool, identityID)

	require.NoError(t, insertSession(ctx, pool, 920000001, identityID, inviteID, "INVITE_ONLY"),
		"舊 schema 對既有值應照常放行")

	err := insertSession(ctx, pool, 920000002, identityID, inviteID, "GOOGLE_VERIFIED")
	require.Error(t, err, "排除 %s 之後 GOOGLE_VERIFIED 必須被擋——否則上面那三條證明不了任何事", widenAssuranceMigration)
	require.Contains(t, err.Error(), "chk_vps_assurance")
}
