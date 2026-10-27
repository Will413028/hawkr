package usecase

import (
	"context"
	"fmt"
	"strconv"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/pagination"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"go.uber.org/zap"
)

const (
	DefaultInboxPageSize int32 = 20
	MaxInboxPageSize     int32 = 500
)

// EventPublisher abstracts MQ publish so usecase doesn't depend on pkgmq directly.
// 41 號設計書 §4.3：publish 給 gateway SSE 推 web（mobile 不訂閱本 channel）。
type EventPublisher interface {
	PublishNotificationInboxNew(ctx context.Context, userID int64, userType string, item map[string]interface{}, unreadCount int64) error
	PublishNotificationUnreadChanged(ctx context.Context, userID int64, userType string, unreadCount int64) error
}

// NotificationInboxUseCase handles inbox operations (list, unread, read, delete).
type NotificationInboxUseCase struct {
	repo         domain.NotificationInboxRepository
	publisher    EventPublisher               // optional — nil 時 publish 為 no-op（不影響主流程）
	buyerClient  BuyerClient                  // enumerates historical buyer_id surrogates for RedactChatNotificationsByAccount
	sellerClient SellerClient                 // enumerates historical store_id surrogates for RedactChatNotificationsByAccount
	tokenRepo    domain.DeviceTokenRepository // used by DeactivateRetiredSellerIncarnationDeviceTokens only (issue #2707 follow-up)
}

// NewNotificationInboxUseCase constructs the usecase. publisher 可為 nil（best-effort publish 設計）。
func NewNotificationInboxUseCase(repo domain.NotificationInboxRepository, publisher EventPublisher, buyerClient BuyerClient, sellerClient SellerClient, tokenRepo domain.DeviceTokenRepository) *NotificationInboxUseCase {
	return &NotificationInboxUseCase{repo: repo, publisher: publisher, buyerClient: buyerClient, sellerClient: sellerClient, tokenRepo: tokenRepo}
}

type ListResult struct {
	Records []*domain.InboxItem
	Total   int64
	HasMore bool
}

func (u *NotificationInboxUseCase) ListNotifications(ctx context.Context, userID int64, userType string, cursorID int64, pageSize int32, category string) (*ListResult, error) {
	pageSize = pagination.NormalizePageSize(pageSize, DefaultInboxPageSize, MaxInboxPageSize)

	// 多取一筆判斷 hasMore
	records, err := u.repo.ListByUser(ctx, userID, userType, cursorID, pageSize+1, category)
	if err != nil {
		return nil, err
	}

	hasMore := int32(len(records)) > pageSize
	if hasMore {
		records = records[:pageSize]
	}

	total, err := u.repo.CountByUser(ctx, userID, userType)
	if err != nil {
		return nil, err
	}
	return &ListResult{Records: records, Total: total, HasMore: hasMore}, nil
}

func (u *NotificationInboxUseCase) GetUnreadCount(ctx context.Context, userID int64, userType string) (int64, error) {
	return u.repo.CountUnread(ctx, userID, userType)
}

// MarkAsRead marks a single notification as read. Idempotent: already-read notifications return success.
// 41 號設計書 §4.3：DB update 成功後 best-effort publish notification.unread_changed。
func (u *NotificationInboxUseCase) MarkAsRead(ctx context.Context, id, userID int64, userType string) error {
	changed, err := u.repo.MarkAsRead(ctx, id, userID, userType)
	if err != nil {
		return err
	}
	if changed > 0 {
		u.publishUnreadChanged(ctx, userID, userType)
	}
	return nil
}

// MarkAllAsRead 41 號設計書 §4.3：成功後 publish unread_changed。
func (u *NotificationInboxUseCase) MarkAllAsRead(ctx context.Context, userID int64, userType string) (int64, error) {
	count, err := u.repo.MarkAllAsRead(ctx, userID, userType)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		u.publishUnreadChanged(ctx, userID, userType)
	}
	return count, nil
}

// DeleteNotifications 41 號設計書 §4.3：成功後 publish unread_changed
// （刪除可能含未讀項目，unreadCount 需重算）。
func (u *NotificationInboxUseCase) DeleteNotifications(ctx context.Context, ids []int64, userID int64, userType string) (int64, error) {
	if len(ids) == 0 {
		return 0, pkgerrors.NewInvalidInput("ids must not be empty")
	}
	count, err := u.repo.SoftDeleteByIDs(ctx, ids, userID, userType)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		u.publishUnreadChanged(ctx, userID, userType)
	}
	return count, nil
}

// CreateInboxItem writes a notification to the user's inbox.
// Idempotent: duplicate (user_id, user_type, event_type, reference_id) is silently ignored.
// 41 號設計書 §4.3：DB write 成功後 best-effort publish notification.inbox.new
// 供 gateway SSE 推 web（mobile 走 FCM 不訂閱本 channel）。
func (u *NotificationInboxUseCase) CreateInboxItem(ctx context.Context, item *domain.InboxItem) error {
	affected, err := u.repo.Create(ctx, item)
	if err != nil {
		return err
	}
	// affected==0 代表被 ON CONFLICT DO NOTHING 去重（同 reference_id 重複事件，
	// scheduler 每輪重發很常見）。此時不得再推 inbox.new，否則 web 端會每隔幾分鐘
	// 跳一次重複通知。對齊 MarkAsRead / Delete 的 `> 0` 守則。
	if affected > 0 {
		u.publishInboxNew(ctx, item)
	}
	return nil
}

// RedactChatNotificationsByAccount purges a deleted account's chat-notification
// inbox PII (個資法 right-to-erasure), driven by the account.deleted,
// buyer.deleted, and seller.deleted MQ handlers (see internal/worker/
// account_deleted.go and internal/worker/seller_deleted.go). Unlike the pre-fix
// version, this enumerates EVERY historical buyer_id and store_id ever bound to
// accountID — not just accountID itself — because notifications.data->>'senderId'
// is a resolved buyer_id/store_id wire surrogate, not the raw account id, and a
// sender_id may belong to a closed stint that has diverged from accountID
// (buyer store-stint re-key W2/W3). Modeled on im-service's
// RedactDeletedAccount enumerate-then-merge shape. Fail-closed: an enumeration
// error aborts before touching the repo, so the MQ consumer retries → DLQ
// instead of silently under-redacting.
func (u *NotificationInboxUseCase) RedactChatNotificationsByAccount(ctx context.Context, accountID int64) (int64, error) {
	buyerIDs, err := u.buyerClient.ListIDsByAccount(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("enumerate historical buyer_ids for account %d: %w", accountID, err)
	}
	storeIDs, err := u.sellerClient.ListIDsByAccount(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("enumerate historical store_ids for account %d: %w", accountID, err)
	}
	senderIDs := mergeUniqueSenderIDs(accountID, buyerIDs, storeIDs)
	return u.repo.RedactChatBySender(ctx, senderIDs)
}

// SoftDeleteRetiredBuyerStintInbox gives a closed buyer stint's inbox rows a
// terminal state. 44 號設計書 §10 D6（accepted 2026-07-15）已定版「買家軟刪除後重新
// 註冊 ＝ 完全全新開始」，§7.6 第 6 步把 notification 列為那 8 個下游之一 ⇒ 新 stint
// 讀不到舊 stint 的通知**是預期行為**，不是要靠遷移補起來的缺口（把
// notifications.user_id 改寫成新 buyer_id 會讓通知成為唯一跨 stint 的資料，與其餘 7
// 個下游不一致）。
//
// 但「讀不到」加上「永久保留」＝零價值＋純個資責任：使用者當初按的是刪除帳號，而
// 那些列的標題常含訂單號一類的個資，cleanup job 只回收 deleted_at 非空滿 90 天的列，
// 這批 deleted_at IS NULL 的列不在它的射程內。soft delete 把它們交給那支既有的
// job，不新增 consumer、不新增排程、不新增表（§22 責任邊界）。
//
// retiredBuyerID 是事件 payload 帶的那個「剛剛關閉的」stint，與列舉結果聯集後使用：
// 列舉走 buyer-service 的 ListBuyerIDsByAccount（current AND closed，故本呼叫只在
// 「該帳號當下沒有活躍買家 stint」的前提下成立，與同檔
// RedactChatNotificationsByAccount 共用這個前提），聯集則保證「這一次確定退休的那個
// id」即使列舉查不到也一定被涵蓋。
//
// Fail-closed：列舉失敗直接回錯，讓 MQ consumer 重試 → DLQ，而不是靜默少清一批。
func (u *NotificationInboxUseCase) SoftDeleteRetiredBuyerStintInbox(ctx context.Context, accountID, retiredBuyerID int64) (int64, error) {
	buyerIDs, err := u.buyerClient.ListIDsByAccount(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("enumerate historical buyer_ids for account %d: %w", accountID, err)
	}
	ids := mergeUniqueBuyerIDs(retiredBuyerID, buyerIDs)
	if len(ids) == 0 {
		return 0, nil
	}
	return u.repo.SoftDeleteByHistoricalBuyerIDs(ctx, ids)
}

// SoftDeleteRetiredSellerIncarnationInbox gives a closed seller incarnation's
// inbox rows a terminal state. Mirrors SoftDeleteRetiredBuyerStintInbox above
// for the seller side (issue #2707): a seller who closes and reopens their
// store gets a new store_id, and gateway now resolves seller notification
// identity to the *current* store_id (resolveNotificationUserID fix) instead
// of the raw account_id — so rows written under a closed incarnation's
// store_id become permanently unreadable, exactly like a closed buyer stint's
// rows under 44 號設計書 §10 D6.
//
// "Unreadable" + "kept forever" = zero value + pure PII liability: the seller
// closed their store, and those rows often carry order numbers and other PII
// in the title/body. The 90-day cleanup job only reclaims deleted_at IS NOT
// NULL rows, so this batch (deleted_at IS NULL) sits outside its reach until
// soft-deleted here — same existing job, no new consumer, no new schedule.
//
// retiredStoreID is the event payload's "just-closed" incarnation, unioned
// with the enumeration result: enumeration walks seller-service's
// ListIDsByAccount (current AND closed store_ids for the account), so this
// call only makes sense when the account currently has no active seller
// incarnation — same precondition RedactChatNotificationsByAccount relies on
// in this same file. The union guarantees the specific incarnation this event
// closed is covered even if enumeration doesn't (yet) surface it.
//
// Fail-closed: an enumeration error returns immediately, letting the MQ
// consumer retry → DLQ instead of silently under-purging.
func (u *NotificationInboxUseCase) SoftDeleteRetiredSellerIncarnationInbox(ctx context.Context, accountID, retiredStoreID int64) (int64, error) {
	storeIDs, err := u.sellerClient.ListIDsByAccount(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("enumerate historical store_ids for account %d: %w", accountID, err)
	}
	ids := mergeUniqueStoreIDs(retiredStoreID, storeIDs)
	if len(ids) == 0 {
		return 0, nil
	}
	return u.repo.SoftDeleteByHistoricalStoreIDs(ctx, ids)
}

// DeactivateRetiredSellerIncarnationDeviceTokens deactivates every device_tokens
// row still registered under a closed seller incarnation's own store_id (issue
// #2707 follow-up). Closes residual risk (b) documented in
// 20260830100003_notification_announcement_identity_migrated_marker.sql: when a
// seller closes and reopens their store on the SAME physical device without that
// device's FCM token rotating, UpsertDeviceToken's ON CONFLICT (user_id,
// user_type, token) inserts a NEW row keyed on the NEW store_id while the OLD
// store_id's row is left is_active = true — nothing previously deactivated it,
// so that device kept receiving duplicate announcement/push delivery across
// incarnation changes indefinitely. Unlike a pre-deploy stale account_id token
// (which self-heals on the device's next natural token rotation), this cause is
// permanent until deactivated here.
//
// Deliberately does NOT enumerate historical incarnations the way
// SoftDeleteRetiredSellerIncarnationInbox above does: only the ONE incarnation
// this event just closed can still hold is_active=true device_tokens rows — any
// incarnation retired before it was already deactivated by this same step when
// IT closed (or predates this fix and self-heals like the pre-deploy case
// above), so there is no historical set to union in.
//
// retiredStoreID == 0 is a no-op, not an error — mirrors mergeUniqueStoreIDs'
// zero-drop above: a backfilled seller.deleted event always carries StoreID=0
// (seller-service/cmd/backfill-seller-deleted/main.go), and WHERE user_id = 0
// AND user_type = 'seller' has no ownership meaning, so it is skipped rather
// than sent to the DB.
//
// Fail-closed: a repo error returns immediately, letting the MQ consumer retry
// → DLQ instead of silently leaving the retired incarnation's tokens active.
func (u *NotificationInboxUseCase) DeactivateRetiredSellerIncarnationDeviceTokens(ctx context.Context, retiredStoreID int64) (int64, error) {
	if retiredStoreID == 0 {
		return 0, nil
	}
	return u.tokenRepo.DeactivateByUser(ctx, retiredStoreID, domain.UserTypeSeller)
}

// mergeUniqueStoreIDs unions the just-retired store_id with the enumerated
// historical set, dropping zeros and duplicates. Mirrors mergeUniqueBuyerIDs
// below for the seller side — see its doc comment for why accountID is
// deliberately NOT seeded here (account_id and store_id are separate
// identity spaces; an unconditional accountID would soft-delete a *different*
// seller's inbox whenever some store_id happened to equal this account_id).
func mergeUniqueStoreIDs(retiredStoreID int64, storeIDs []int64) []int64 {
	seen := make(map[int64]bool, len(storeIDs)+1)
	result := make([]int64, 0, len(storeIDs)+1)
	for _, id := range append([]int64{retiredStoreID}, storeIDs...) {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

// mergeUniqueBuyerIDs unions the just-retired buyer_id with the enumerated
// historical set, dropping zeros and duplicates.
//
// Deliberately does NOT seed with accountID, unlike mergeUniqueSenderIDs below:
// there the id is matched against a chat payload's senderId (a value the account
// itself may have produced), here it is matched against notifications.user_id in
// the buyer identity space. account_id and buyer_id are separate spaces, so an
// unconditional accountID would soft-delete a *different* buyer's inbox whenever
// some buyer_id happened to equal this account_id.
func mergeUniqueBuyerIDs(retiredBuyerID int64, buyerIDs []int64) []int64 {
	seen := make(map[int64]bool, len(buyerIDs)+1)
	result := make([]int64, 0, len(buyerIDs)+1)
	for _, id := range append([]int64{retiredBuyerID}, buyerIDs...) {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

// mergeUniqueSenderIDs combines accountID with its historical buyer_id/store_id
// surrogates into a deduplicated purge set. accountID is included unconditionally
// (an account.deleted event carries no buyer/store surrogate at all, so the
// account-scoped redaction path relies on accountID itself being a candidate
// sender id). Duplicated locally rather than shared from im-service's
// mergeUniqueIDs (services/im-service/internal/usecase/message.go) — this is a
// small (~12 line), domain-agnostic dedup utility, and extracting a pkg/-level
// helper for two compliance-critical, independently-owned redaction paths adds
// cross-service coupling for marginal DRY benefit.
func mergeUniqueSenderIDs(accountID int64, buyerIDs, storeIDs []int64) []int64 {
	seen := map[int64]bool{accountID: true}
	result := []int64{accountID}
	for _, group := range [][]int64{buyerIDs, storeIDs} {
		for _, id := range group {
			if !seen[id] {
				seen[id] = true
				result = append(result, id)
			}
		}
	}
	return result
}

// publishInboxNew best-effort 發送 notification.inbox.new；失敗只 log 不影響主流程。
func (u *NotificationInboxUseCase) publishInboxNew(ctx context.Context, item *domain.InboxItem) {
	if u.publisher == nil || item == nil {
		return
	}
	// Count the same canonical identity that was just inserted so the realtime
	// unread_changed event is consistent with the inbox row and REST reads.
	unread, err := u.repo.CountUnread(ctx, item.UserID, item.UserType)
	if err != nil {
		zap.L().Warn("publishInboxNew: CountUnread failed, fallback to 0", zap.Error(err))
		unread = 0
	}
	payload := inboxItemToMap(item)
	_ = u.publisher.PublishNotificationInboxNew(ctx, item.UserID, item.UserType, payload, unread)
}

// publishUnreadChanged best-effort 發送 notification.unread_changed。
func (u *NotificationInboxUseCase) publishUnreadChanged(ctx context.Context, userID int64, userType string) {
	if u.publisher == nil {
		return
	}
	unread, err := u.repo.CountUnread(ctx, userID, userType)
	if err != nil {
		zap.L().Warn("publishUnreadChanged: CountUnread failed", zap.Error(err))
		return
	}
	_ = u.publisher.PublishNotificationUnreadChanged(ctx, userID, userType, unread)
}

// inboxItemToMap 將 domain.InboxItem 序列化成 SSE-friendly map（避免 gateway 端依賴 domain package）。
// gateway SSE handler 收到後可重用既有 deriveRefTypeAndID 邏輯從 data map 推導 refType/refId。
func inboxItemToMap(item *domain.InboxItem) map[string]interface{} {
	if item == nil {
		return nil
	}
	m := map[string]interface{}{
		// id / userId 都是 64-bit TSID，遠超 JS Number.MAX_SAFE_INTEGER (2^53)。
		// 必須序列化成字串（對齊 REST inbox 的 strconv.FormatInt），否則經
		// MQ→gateway→SSE 推到 web 前端會精度丟失；id 型別不一致更會導致同一則
		// 通知 toast 兩次。
		"id":          strconv.FormatInt(item.ID, 10),
		"userId":      strconv.FormatInt(item.UserID, 10),
		"userType":    item.UserType,
		"eventType":   item.EventType,
		"type":        item.EventType, // 對齊既有 REST response 欄位 `type`
		"referenceId": item.ReferenceID,
		"title":       item.Title,
		"body":        item.Body,
		"isRead":      item.IsRead,
		"createdAt":   item.CreatedAt,
		"data":        item.Data,
	}
	if item.ReadAt != nil {
		m["readAt"] = *item.ReadAt
	} else {
		m["readAt"] = nil
	}
	return m
}
