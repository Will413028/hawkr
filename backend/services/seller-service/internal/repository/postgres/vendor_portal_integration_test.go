//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

func TestEnsureInvite_FirstCallCreatesIdentityAndInvite(t *testing.T) {
	ctx := context.Background()
	env := newVendorPortalTestEnv(t)
	defer env.Close()

	in := domain.EnsureInviteInput{
		ProspectID:     env.ProspectID,
		RFQRecipientID: 99001,
		ExpiresAt:      time.Now().Add(30 * 24 * time.Hour).UTC(),
	}

	inv, secret, err := env.Repo.EnsureInvite(ctx, in)
	require.NoError(t, err)
	require.NotZero(t, inv.ID)
	require.Equal(t, int64(99001), inv.RFQRecipientID)
	require.Len(t, secret, 64, "256-bit secret 以 hex 呈現是 64 個字元")
	require.Nil(t, inv.SecretDeliveredAt)
	require.Equal(t, "vendor@example.com", inv.Email,
		"回傳的明文 email 供 notification-service 組 mail.Message.To 用")

	var stored string
	require.NoError(t, env.Pool.QueryRow(ctx,
		`SELECT token_hash FROM vendor_portal_invites WHERE id=$1`, inv.ID).Scan(&stored))
	require.NotEqual(t, secret, stored)
	require.Len(t, stored, 64)

	var snapshot string
	require.NoError(t, env.Pool.QueryRow(ctx,
		`SELECT invited_email FROM vendor_portal_identities WHERE prospect_id=$1`,
		env.ProspectID).Scan(&snapshot))
	require.Equal(t, "vendor@example.com", snapshot)
}

func TestEnsureInvite_RotatesWhenNotYetDelivered(t *testing.T) {
	ctx := context.Background()
	env := newVendorPortalTestEnv(t)
	defer env.Close()

	in := domain.EnsureInviteInput{
		ProspectID: env.ProspectID, RFQRecipientID: 99002,
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	}

	first, s1, err := env.Repo.EnsureInvite(ctx, in)
	require.NoError(t, err)
	second, s2, err := env.Repo.EnsureInvite(ctx, in)
	require.NoError(t, err)

	require.Equal(t, first.ID, second.ID, "同一個 rfq_recipient 只能有一張邀請")
	require.NotEqual(t, s1, s2, "未交付前必須輪替")
	require.Equal(t, "vendor@example.com", first.Email)
	require.Equal(t, "vendor@example.com", second.Email, "輪替分支也要回傳 email")
}

func TestEnsureInvite_RefusesAfterDelivered(t *testing.T) {
	ctx := context.Background()
	env := newVendorPortalTestEnv(t)
	defer env.Close()

	in := domain.EnsureInviteInput{
		ProspectID: env.ProspectID, RFQRecipientID: 99003,
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	}
	inv, _, err := env.Repo.EnsureInvite(ctx, in)
	require.NoError(t, err)
	require.NoError(t, env.Repo.MarkSecretDelivered(ctx, inv.ID, time.Now().UTC()))

	_, secret, err := env.Repo.EnsureInvite(ctx, in)
	require.ErrorIs(t, err, domain.ErrInviteSecretAlreadyDelivered)
	require.Empty(t, secret)
}

func TestMarkSecretDelivered_SecondCallIsNotAnError(t *testing.T) {
	ctx := context.Background()
	env := newVendorPortalTestEnv(t)
	defer env.Close()

	inv, _, err := env.Repo.EnsureInvite(ctx, domain.EnsureInviteInput{
		ProspectID: env.ProspectID, RFQRecipientID: 99006,
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	})
	require.NoError(t, err)

	require.NoError(t, env.Repo.MarkSecretDelivered(ctx, inv.ID, time.Now().UTC()))
	require.NoError(t, env.Repo.MarkSecretDelivered(ctx, inv.ID, time.Now().UTC()),
		"重投第二次呼叫 MarkSecretDelivered 時 0 rows affected 不是錯誤——代表前一次已經標記過")
}

func TestEnsureInvite_ChangedProspectEmailUpdatesIdentitySnapshot(t *testing.T) {
	ctx := context.Background()
	env := newVendorPortalTestEnv(t)
	defer env.Close()

	_, _, err := env.Repo.EnsureInvite(ctx, domain.EnsureInviteInput{
		ProspectID: env.ProspectID, RFQRecipientID: 99010,
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	})
	require.NoError(t, err)

	var before string
	require.NoError(t, env.Pool.QueryRow(ctx,
		`SELECT invited_email FROM vendor_portal_identities WHERE prospect_id=$1`,
		env.ProspectID).Scan(&before))
	require.Equal(t, "vendor@example.com", before)

	_, err = env.Pool.Exec(ctx,
		`UPDATE seller_prospects SET public_email = $1 WHERE id = $2`,
		"new-vendor@example.com", env.ProspectID)
	require.NoError(t, err)

	inv2, _, err := env.Repo.EnsureInvite(ctx, domain.EnsureInviteInput{
		ProspectID: env.ProspectID, RFQRecipientID: 99011,
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	})
	require.NoError(t, err)
	require.Equal(t, "new-vendor@example.com", inv2.Email)

	var after string
	require.NoError(t, env.Pool.QueryRow(ctx,
		`SELECT invited_email FROM vendor_portal_identities WHERE prospect_id=$1`,
		env.ProspectID).Scan(&after))
	require.Equal(t, "new-vendor@example.com", after, "email 變更後快照必須更新")
}

// 同一個 RFQRecipientID 第二次呼叫走 rotate 分支（不是 create 分支）：
// DLQ 重投未送達的邀請時最常見的觸發點就是這裡——BD 才剛改完地址。
func TestEnsureInvite_RotateWithChangedProspectEmailUpdatesIdentitySnapshot(t *testing.T) {
	ctx := context.Background()
	env := newVendorPortalTestEnv(t)
	defer env.Close()

	_, secret1, err := env.Repo.EnsureInvite(ctx, domain.EnsureInviteInput{
		ProspectID: env.ProspectID, RFQRecipientID: 99012,
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	})
	require.NoError(t, err)

	var before string
	require.NoError(t, env.Pool.QueryRow(ctx,
		`SELECT invited_email FROM vendor_portal_identities WHERE prospect_id=$1`,
		env.ProspectID).Scan(&before))
	require.Equal(t, "vendor@example.com", before)

	_, err = env.Pool.Exec(ctx,
		`UPDATE seller_prospects SET public_email = $1 WHERE id = $2`,
		"rotated-vendor@example.com", env.ProspectID)
	require.NoError(t, err)

	inv2, secret2, err := env.Repo.EnsureInvite(ctx, domain.EnsureInviteInput{
		ProspectID: env.ProspectID, RFQRecipientID: 99012,
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	})
	require.NoError(t, err)
	require.NotEqual(t, secret1, secret2, "rotate 必須換發新 secret")
	require.Equal(t, "rotated-vendor@example.com", inv2.Email)

	var after string
	require.NoError(t, env.Pool.QueryRow(ctx,
		`SELECT invited_email FROM vendor_portal_identities WHERE prospect_id=$1`,
		env.ProspectID).Scan(&after))
	require.Equal(t, "rotated-vendor@example.com", after, "rotate 分支也必須更新快照")
}

func TestEnsureInvite_SharedEmailAcrossProspectsBothGetInvited(t *testing.T) {
	ctx := context.Background()
	env := newVendorPortalTestEnv(t)
	defer env.Close()

	other := env.InsertProspectWithEmail(t, "另一家業者", "vendor@example.com")

	first, s1, err := env.Repo.EnsureInvite(ctx, domain.EnsureInviteInput{
		ProspectID: env.ProspectID, RFQRecipientID: 99004,
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	})
	require.NoError(t, err)

	second, s2, err := env.Repo.EnsureInvite(ctx, domain.EnsureInviteInput{
		ProspectID: other, RFQRecipientID: 99005,
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	})
	require.NoError(t, err,
		"seller_prospects 刻意不對 email 設 UNIQUE：共用總機／代工廠／育成中心可能合法共用一個信箱，"+
			"第二家業者不得因此收不到邀請信")

	require.NotEqual(t, first.IdentityID, second.IdentityID, "兩家業者必須是各自獨立的 identity")
	require.NotEqual(t, s1, s2, "兩封邀請信必須各自帶不同的 secret")
	require.Equal(t, "vendor@example.com", first.Email)
	require.Equal(t, "vendor@example.com", second.Email)
}

func TestEnsureInvite_ProspectWithoutEmailFails(t *testing.T) {
	ctx := context.Background()
	env := newVendorPortalTestEnv(t)
	defer env.Close()

	noEmail := env.InsertProspectWithEmail(t, "尚未填信箱的業者", "")

	_, secret, err := env.Repo.EnsureInvite(ctx, domain.EnsureInviteInput{
		ProspectID: noEmail, RFQRecipientID: 99006,
		ExpiresAt: time.Now().Add(time.Hour).UTC(),
	})
	require.ErrorIs(t, err, domain.ErrProspectHasNoEmail)
	require.Empty(t, secret)

	var n int
	require.NoError(t, env.Pool.QueryRow(ctx,
		`SELECT count(*) FROM vendor_portal_invites WHERE rfq_recipient_id = $1`, 99006).Scan(&n))
	require.Zero(t, n, "失敗時不得留下半張邀請")
}

func TestEnsureInvite_PlaceholderEmailWritesNothing(t *testing.T) {
	ctx := context.Background()
	env := newVendorPortalTestEnv(t)
	defer env.Close()

	prospectID := env.InsertProspectWithEmail(t, "e2e-placeholder-inbox", "待確認")

	_, _, err := env.Repo.EnsureInvite(ctx, domain.EnsureInviteInput{
		ProspectID:     prospectID,
		RFQRecipientID: 99301,
		ExpiresAt:      time.Now().Add(30 * 24 * time.Hour).UTC(),
	})

	require.ErrorIs(t, err, domain.ErrProspectHasNoEmail,
		"dev 上 640 個業者有 540 個的 public_email 是「待確認」，收不到信就不該發邀請")

	var identities, invites int
	require.NoError(t, env.Pool.QueryRow(ctx,
		`SELECT count(*) FROM vendor_portal_identities WHERE prospect_id = $1`, prospectID).Scan(&identities))
	require.NoError(t, env.Pool.QueryRow(ctx,
		`SELECT count(*) FROM vendor_portal_invites WHERE rfq_recipient_id = $1`, int64(99301)).Scan(&invites))
	require.Zero(t, identities, "不得留下 identity")
	require.Zero(t, invites, "不得留下 invite（舊行為會建好還標成已寄出）")
}
