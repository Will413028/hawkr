// procurement_rfq_publication_invite_test.go —— RFQ 發布直投線（#2450 Slice 2b、
// 64 號設計書 §4.2–§4.4、36 號 §3.6l.1）的模板契約測試。
//
// 本檔守的是**一條機械保證**，不是文案：ai-service 的 publicationoutbox worker 刻意
// 「先發 MQ、後把 invite 轉 SENT」，所以 MarkDone 失敗時那一列會重發；吸收重發的是
// notification-service 的兩道 DB 唯一索引
//
//	notifications_inbox_dedup_idx / push_notifications_dedup_idx
//	UNIQUE (user_id, user_type, event_type, reference_id)
//
// 而這兩道索引只有在 reference_id **逐 invite 唯一**時才吸收得掉。兩種綁錯法的後果不同：
//
//   - 綁 #{publicationId}（同一場發布的所有收件人共用一個值）⇒ 去重過頭：同一場發布
//     只有第一位賣家收得到，其餘全被吞掉。
//   - **留空 ⇒ 去重整個關掉**（不是「退化成上一種」）：handleTemplate 算出空字串，
//     repository 的 nullableText（internal/repository/postgres/nullable.go:5-10）把 ""
//     映成 SQL NULL，而兩道索引都是 partial index（WHERE reference_id IS NOT NULL），
//     該列根本不在索引裡 ⇒ 每一次重發都寫成一則全新的通知。
//
// 兩種失效都沒有任何訊號（不進 DLQ、不重試、沒有 metric）。
//
// 形狀比照 procurement_quote_invite_test.go，但**兩條線的 payload 不同**：本線恰好三欄
// 且全為 camelCase 字串化 TSID（inviteId / publicationId / storeId），quote-invite 那條
// 是 snake_case store_id。抄錯欄名的後果是解析不到收件人、通知靜默 0 投遞。
package worker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

const (
	rfqPublicationInviteEventType  = "procurement.rfq_publication_invite.created.seller"
	rfqPublicationInviteRoutingKey = "procurement.rfq_publication_invite.created"

	// 三個值刻意互不相同：綁錯欄位（publicationId／storeId）時斷言才會紅。
	// 首任帳號 grandfather 恆等（store_id == account_id）那類「值剛好相等所以假綠」的
	// 坑，在這裡的對應形式就是「三個 id 用同一個字面值」。
	rfqInviteID      = "7501000000000000001"
	rfqPublicationID = "7502000000000000002"
	rfqStoreIDStr    = "7496483081637855232"
	rfqStoreID       = int64(7496483081637855232)
)

// rfqPublicationInviteTemplate 複製
// db/migrations/20260910120000_procurement_rfq_publication_invite_template.sql 寫進
// notification_templates 的那一列。migration 與本結構的綁定關係由
// TestProcurementRfqPublicationInviteMigrationBindsReferenceIDToInviteID 錨住。
func rfqPublicationInviteTemplate() *domain.NotificationTemplate {
	return &domain.NotificationTemplate{
		ID:              1,
		Exchange:        "procurement.events",
		RoutingKey:      rfqPublicationInviteRoutingKey,
		EventType:       rfqPublicationInviteEventType,
		TargetUserType:  "seller",
		TargetUserField: "storeId",
		Category:        "order_update",
		TitleTemplate:   "買家指名向你採購",
		BodyTemplate:    "有買家把採購需求直接發給你，請開啟網頁查看",
		DataType:        "PROCUREMENT_RFQ_PUBLICATION_INVITE",
		DataFields: map[string]domain.FieldSpec{
			"inviteId":      {Field: strPtr("inviteId")},
			"publicationId": {Field: strPtr("publicationId")},
		},
		ReferenceIDTemplate: strPtr("#{inviteId}"),
		Enabled:             true,
	}
}

// rfqPublicationInvitePayload 是 producer 實際寫進 outbox 的形狀：恰好三欄、全字串。
// 來源 services/ai-service/internal/procurement/usecase/publication_execute.go 的
// inviteCreatedPayload；worker 原樣送出，不解碼再重編碼。
func rfqPublicationInvitePayload(t *testing.T) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"inviteId":      rfqInviteID,
		"publicationId": rfqPublicationID,
		"storeId":       rfqStoreIDStr,
	})
	require.NoError(t, err)
	return body
}

func newRfqPublicationInviteHandler(t *testing.T) (handler func(context.Context, string, []byte) error, push *capturingPushRepo, inbox *capturingInboxRepo) {
	t.Helper()
	cache := buildTestCache([]*domain.NotificationTemplate{rfqPublicationInviteTemplate()})
	sendPush, pushRepo := newTestSendPushWithRepo()
	inboxUC, inboxRepo := newTestInboxUC()
	h := newTemplateHandler(
		cache, sendPush, inboxUC, "procurement.events",
		"https://test-media.example.com", zap.NewNop(),
	)
	return h, pushRepo, inboxRepo
}

// reference_id 必須解析成 inviteId——不是 publicationId、不是 storeId、不是空字串。
func TestTemplateHandler_ProcurementRfqPublicationInviteReferenceIsInviteID(t *testing.T) {
	handler, pushRepo, inboxRepo := newRfqPublicationInviteHandler(t)

	require.NoError(t, handler(
		context.Background(),
		rfqPublicationInviteRoutingKey,
		rfqPublicationInvitePayload(t),
	))

	require.Len(t, pushRepo.records, 1)
	require.Equal(t, rfqInviteID, pushRepo.records[0].ReferenceID,
		"push dedup 鍵的 reference_id 必須是 inviteId；綁 publicationId 會讓同一場發布只有第一位賣家收得到")
	require.NotEqual(t, rfqPublicationID, pushRepo.records[0].ReferenceID)

	require.NotNil(t, inboxRepo.createdItem)
	require.Equal(t, rfqInviteID, inboxRepo.createdItem.ReferenceID,
		"inbox dedup 鍵的 reference_id 必須是 inviteId")
	require.Equal(t, rfqPublicationInviteEventType, inboxRepo.createdItem.EventType)
}

// 收件人取自 camelCase 的 storeId，且落在 store_id 身分空間（13 號 §12.1）。
func TestTemplateHandler_ProcurementRfqPublicationInviteTargetsStoreID(t *testing.T) {
	handler, pushRepo, inboxRepo := newRfqPublicationInviteHandler(t)

	require.NoError(t, handler(
		context.Background(),
		rfqPublicationInviteRoutingKey,
		rfqPublicationInvitePayload(t),
	))

	require.Len(t, pushRepo.records, 1)
	require.Equal(t, rfqStoreID, pushRepo.records[0].UserID)
	require.Equal(t, "seller", pushRepo.records[0].UserType)
	require.NotNil(t, inboxRepo.createdItem)
	require.Equal(t, rfqStoreID, inboxRepo.createdItem.UserID)
	require.Equal(t, "seller", inboxRepo.createdItem.UserType)
}

// 重發（MarkDone 失敗後 worker 重跑那一列）必須算出**同一個** reference_id：
// 這正是兩道 DB 唯一索引吸收得掉它的前提。索引本身由 DB 保證，本測試守的是「餵給索引
// 的第四個欄位穩定且逐 invite 唯一」這一半。
func TestTemplateHandler_ProcurementRfqPublicationInviteRedeliveryKeepsSameReference(t *testing.T) {
	handler, pushRepo, _ := newRfqPublicationInviteHandler(t)
	body := rfqPublicationInvitePayload(t)

	require.NoError(t, handler(context.Background(), rfqPublicationInviteRoutingKey, body))
	require.NoError(t, handler(context.Background(), rfqPublicationInviteRoutingKey, body))

	require.Len(t, pushRepo.records, 2)
	require.Equal(t, pushRepo.records[0].ReferenceID, pushRepo.records[1].ReferenceID)
	require.Equal(t, rfqInviteID, pushRepo.records[1].ReferenceID)
	require.Equal(t, pushRepo.records[0].UserID, pushRepo.records[1].UserID)
	require.Equal(t, pushRepo.records[0].EventType, pushRepo.records[1].EventType)
}

// 本線的 payload 沒有 snake_case store_id。誤把 target_user_field 抄成 quote-invite 那條的
// `store_id` 時，這支測試是唯一會紅的東西——handler 對取不到 user_id 只印 Warn 就 return，
// 事件照樣 ACK，賣家永遠收不到而且沒有任何訊號。
func TestTemplateHandler_ProcurementRfqPublicationInviteSnakeCaseStoreIDFailsClosed(t *testing.T) {
	tmpl := rfqPublicationInviteTemplate()
	tmpl.TargetUserField = "store_id" // 隔壁 legacy quote-invite 那條線的欄名
	cache := buildTestCache([]*domain.NotificationTemplate{tmpl})
	sendPush, pushRepo := newTestSendPushWithRepo()
	inboxUC, inboxRepo := newTestInboxUC()
	handler := newTemplateHandler(
		cache, sendPush, inboxUC, "procurement.events",
		"https://test-media.example.com", zap.NewNop(),
	)

	require.NoError(t, handler(
		context.Background(),
		rfqPublicationInviteRoutingKey,
		rfqPublicationInvitePayload(t),
	))

	require.Empty(t, pushRepo.records, "本線 payload 是 camelCase storeId，snake_case store_id 不得意外命中")
	require.Nil(t, inboxRepo.createdItem)
}

// 錨點：上面四支測試用的是本檔自己造的 template，證明的是 **handler 會照
// reference_id_template 解析**；真正寫進 DB 的那一列在 migration 裡。這一支把兩者綁起來，
// 否則「migration 綁 publicationId、測試綁 inviteId」可以同時全綠。
//
// 整包 migration 重放的契約測試（TestTemplateContract，build tag integration）驗的是
// 「reference_id_template 的 placeholder 有沒有被 publisher 實發」——#{publicationId} 也
// 是實發欄位，所以那道 gate 對本失效模式是綠的，不能取代這一支。
func TestProcurementRfqPublicationInviteMigrationBindsReferenceIDToInviteID(t *testing.T) {
	path := filepath.Join(
		"..", "..", "db", "migrations",
		"20260910120000_procurement_rfq_publication_invite_template.sql",
	)
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "migration 被改名或搬走時本錨點必須紅，不是靜默跳過")
	sql := string(raw)

	require.Contains(t, sql, "'"+rfqPublicationInviteEventType+"'",
		"migration 必須寫入本測試斷言的那個 event_type")
	require.Contains(t, sql, "'#{inviteId}'",
		"reference_id_template 必須是 '#{inviteId}'——它是兩道 dedup 唯一索引的第四個欄位")
	require.NotContains(t, sql, "'#{publicationId}'",
		"#{publicationId} 逐場發布共用一個值，當 reference_id 會讓同一場發布只有第一位賣家收得到")
	require.Contains(t, sql, "'storeId'",
		"target_user_field 必須是 camelCase storeId（本線 payload 的鍵名）")

	// enabled 必須為 true：停用等於讓已經發得出去的事件繼續被 logger.Debug + ACK 丟棄。
	require.True(t, strings.Contains(sql, "true, 'visible', true"),
		"本列必須 enabled = true、push_mode = visible、identity_migrated = true")
}
