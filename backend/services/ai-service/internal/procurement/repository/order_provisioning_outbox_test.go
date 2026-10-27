//go:build integration

package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRepo_InsertOrderProvisioningOutbox_IsIdempotentPerAcceptance 證的是
// uq_propo_acceptance_event 真的擋在 DB：acceptance 的 insert 會因為冪等回放而
// 再跑一次這一支，第二次不得產生第二筆建單命令——否則同一筆成交會建出兩張訂單。
func TestRepo_InsertOrderProvisioningOutbox_IsIdempotentPerAcceptance(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	acceptance, err := r.InsertBuyerQuoteAcceptance(ctx, acceptanceInput("idem-outbox-1", "hash-1"))
	require.NoError(t, err)

	payload := []byte(`{"schemaVersion":1,"acceptanceId":"1"}`)
	require.NoError(t, r.InsertOrderProvisioningOutbox(ctx, acceptance.ID, payload, refTime()))
	// 第二次是冪等回放的形狀：同一個 acceptance、同一個 event_type。
	require.NoError(t, r.InsertOrderProvisioningOutbox(ctx, acceptance.ID, payload, refTime()))

	row, err := r.FindOrderProvisioningOutboxByAcceptance(ctx, acceptance.ID)
	require.NoError(t, err)
	require.Equal(t, acceptance.ID, row.AcceptanceID)
	require.Equal(t, OrderProvisioningOutboxEventOrderShellRequested, row.EventType)
	require.Equal(t, "pending", row.Status)
	require.Nil(t, row.CompletedAt)
	// FindOrderProvisioningOutboxByAcceptance 是 :one——真的插了兩列的話它會回
	// 「多列」而不是靜靜地選一列，所以上面沒有錯誤本身就是 uq 生效的證據。
}

func TestRepo_FindOrderProvisioningOutbox_MissingIsTypedNotFound(t *testing.T) {
	r, ctx := newAcceptanceRepo(t)

	_, err := r.FindOrderProvisioningOutboxByAcceptance(ctx, 424242)
	require.ErrorIs(t, err, ErrOrderProvisioningOutboxNotFound)
}
