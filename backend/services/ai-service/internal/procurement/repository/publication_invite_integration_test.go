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

// 這三支守的是 Scope 6「每 recipient 至多一 invite」的**機械**保證。應用層的去重在上游
// （uq_procurement_candidates_set_store ＋ compileAudience），這裡驗的是防線本身有效。

func TestInsertPublicationInvites_WritesOneRowPerRecipientInPositionOrder(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	pubID := repo.newID()
	invites, err := repo.InsertPublicationInvites(ctx, InsertPublicationInvitesInput{
		PublicationID: pubID,
		BuyerID:       identity.BuyerID(9101),
		Recipients: []domain.PublicationRecipient{
			domain.PlatformRecipient(5001),
			domain.ExternalRecipient(6001),
			domain.PlatformRecipient(5002),
		},
		Now: refTime(),
	})
	require.NoError(t, err)
	require.Len(t, invites, 3)

	require.Equal(t, 0, invites[0].Position)
	require.Equal(t, domain.QuoteLanePlatform, invites[0].Lane)
	require.Equal(t, int64(5001), invites[0].StoreID)
	require.Equal(t, "QUEUED", invites[0].DeliveryState,
		"PLATFORM 有真實投遞管道，起始就進佇列")

	require.Equal(t, 1, invites[1].Position)
	require.Equal(t, domain.QuoteLaneExternal, invites[1].Lane)
	require.Equal(t, int64(6001), invites[1].ProspectID)
	require.Equal(t, "QUEUED", invites[1].DeliveryState,
		"Vendor Portal E1 起 EXTERNAL 也有真實投遞管道，起始就進佇列")

	listed, err := repo.ListPublicationInvites(ctx, pubID)
	require.NoError(t, err)
	require.Len(t, listed, 3)
	require.Equal(t, invites[0].ID, listed[0].ID, "順序是契約，依 position 排序")
}

func TestInsertPublicationInvites_SameStoreTwiceIsRejected(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	_, err := repo.InsertPublicationInvites(ctx, InsertPublicationInvitesInput{
		PublicationID: repo.newID(),
		BuyerID:       identity.BuyerID(9102),
		Recipients: []domain.PublicationRecipient{
			domain.PlatformRecipient(5001),
			domain.PlatformRecipient(5001),
		},
		Now: refTime(),
	})
	require.Error(t, err,
		"uq_prpi_publication_store 必須擋下同一 publication 內重複的 store")
}

func TestInsertPublicationInvites_SameProspectTwiceIsRejected(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	_, err := repo.InsertPublicationInvites(ctx, InsertPublicationInvitesInput{
		PublicationID: repo.newID(),
		BuyerID:       identity.BuyerID(9103),
		Recipients: []domain.PublicationRecipient{
			domain.ExternalRecipient(6001),
			domain.ExternalRecipient(6001),
		},
		Now: refTime(),
	})
	require.Error(t, err,
		"uq_prpi_publication_prospect 必須擋下同一 publication 內重複的站外業者")
}
