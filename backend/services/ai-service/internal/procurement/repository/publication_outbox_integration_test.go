//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

func seedOneInvite(ctx context.Context, t *testing.T, repo *Repo, storeID int64) PublicationInvite {
	t.Helper()
	invites, err := repo.InsertPublicationInvites(ctx, InsertPublicationInvitesInput{
		PublicationID: repo.newID(),
		BuyerID:       identity.BuyerID(9201),
		Recipients:    []domain.PublicationRecipient{domain.PlatformRecipient(storeID)},
		Now:           refTime(),
	})
	require.NoError(t, err)
	return invites[0]
}

func TestInsertPublicationOutbox_SecondRowForSameInviteAndEventIsRejected(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	inv := seedOneInvite(ctx, t, repo, 5101)
	payload := []byte(`{"inviteId":"1"}`)

	require.NoError(t, repo.InsertPublicationOutbox(ctx, inv.ID, PublicationOutboxEventInviteCreated, payload, refTime()))
	err := repo.InsertPublicationOutbox(ctx, inv.ID, PublicationOutboxEventInviteCreated, payload, refTime())
	require.Error(t, err,
		"uq_prpo_invite_event 是 deterministic dedupe 的機械保證：一個 invite 至多一個 command")
}

func TestInsertPublicationOutbox_UnknownEventTypeIsRejectedByCheck(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	inv := seedOneInvite(ctx, t, repo, 5102)
	err := repo.InsertPublicationOutbox(ctx, inv.ID, "NOT_A_REAL_EVENT", []byte(`{}`), refTime())
	require.Error(t, err,
		"chk_prpo_event_type 必須擋下不在值域內的 event_type——這條表的值域有 CHECK，"+
			"不是只寫在行尾註解")
}

func TestMarkPublicationInviteSent_IsIdempotentCAS(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	inv := seedOneInvite(ctx, t, repo, 5103)
	require.Equal(t, InviteDeliveryStateQueued, inv.DeliveryState)

	require.NoError(t, repo.MarkPublicationInviteSent(ctx, inv.ID, refTime()))
	// 第二次是重試路徑（MarkDone 失敗後 Worker 重跑 Process）。影響 0 列不是錯誤。
	require.NoError(t, repo.MarkPublicationInviteSent(ctx, inv.ID, refTime()))

	listed, err := repo.ListPublicationInvites(ctx, inv.PublicationID)
	require.NoError(t, err)
	require.Equal(t, InviteDeliveryStateSent, listed[0].DeliveryState)
}
