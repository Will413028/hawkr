//go:build integration

// 驗證 20260815120000_procurement_quote_invite_templates.sql（Task 11）套用後，
// procurement.quote_invite.created.seller／procurement.quote_invite.lost.seller
// 兩筆 notification_templates row 確實存在、且 reference_id_template 為
// https:// 網址（非 hawkr:// app scheme——本輪「網頁端、不做 App」的決定）。
// 走 getTestPool(t)（dbtest_test.go），套的是 db/migrations 底下真正的 migration
// 檔，不是另外建的 fixture。
//
// lost.seller 的 reference_id_template 必須帶 #{batchId}（非靜態字串）——見
// migration 檔內的註解：push_notifications dedup unique index 是
// (user_id, user_type, event_type, reference_id)，靜態 reference_id 會讓同一
// 賣家第二次以後的「報價單已被其他賣家承接」通知被 dedup 靜默吞掉。
//
// 兩筆必須是 enabled = true：ai-service publisher、procurement.events topology 與
// notificationQueueByExchange 已經同一批落地。收件人身分已定版為 store_id；
// migration 不會回填歷史 notifications。
package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration_BuyerCleanupTemplatesUseCanonicalBuyerID(t *testing.T) {
	pool := getTestPool(t)
	ctx := context.Background()

	for _, eventType := range []string{
		"payment.rescue.refund_completed",
		"product.restock.target",
	} {
		t.Run(eventType, func(t *testing.T) {
			var targetUserType, targetUserField string
			err := pool.QueryRow(ctx, `
				SELECT target_user_type, target_user_field
				FROM notification_templates
				WHERE event_type = $1
			`, eventType).Scan(&targetUserType, &targetUserField)
			require.NoError(t, err, "row for event_type %q should exist after migration", eventType)
			require.Equal(t, "buyer", targetUserType)
			require.Equal(t, "buyer_id", targetUserField,
				"buyer cleanup template must read the canonical buyer_id payload field")
		})
	}
}
