package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/blocklist"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	bizcode "github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/moderation"
	"github.com/Will413028/hawkr/go-pkg/pagination"
	"github.com/Will413028/hawkr/go-pkg/ratelimit"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/Will413028/hawkr/go-pkg/tsid"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"github.com/Will413028/hawkr/im-service/internal/domain"
	"github.com/Will413028/hawkr/im-service/internal/repository/postgres/gen"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	DefaultHistoryLimit          int32 = 20
	MaxHistoryLimit              int32 = 100
	DefaultConversationsPageSize int32 = 20
	MaxConversationsPageSize     int32 = 100
	DefaultSyncLimit             int32 = 100
	MaxSyncLimit                 int32 = 200
	PreviewMaxLength                   = 100

	// MaxMessageContentLength bounds stored message content (character count),
	// matching the messages.content CHECK constraint and the WS maxMsgSize (4096).
	// Over-length content is rejected with a 400, never silently truncated.
	MaxMessageContentLength = 4096

	// Read-path placeholders for moderated / recalled messages (B-135). hidden_at /
	// deleted_at are set out-of-band (HideMessage RPC / future recall) while the
	// original content row is left intact, so every read path renders these instead.
	hiddenMessagePlaceholder  = "此訊息已被移除"
	deletedMessagePlaceholder = "訊息已收回"
)

// TombstoneContent returns the content to expose for a message on read paths.
// A platform-hidden (moderation) or recalled message must never leak its original
// text: hidden_at / deleted_at are set out-of-band and the stored content is left
// intact, so callers substitute a neutral placeholder here. The row is always kept
// (seq_id continuity); only Content changes. Returns the original content when the
// message is visible.
func TombstoneContent(m gen.Message) string {
	switch {
	case m.HiddenAt != nil:
		return hiddenMessagePlaceholder
	case m.DeletedAt != nil:
		return deletedMessagePlaceholder
	default:
		return m.Content
	}
}

// computePreview returns the authoritative one-line summary for a message —
// persisted as messages.preview_text / conversations.last_message_preview,
// and reused verbatim as ChatMessageSentEvent.ContentPreview (the FCM push
// notification body). im-service stays agnostic to card semantics: a
// non-empty hint (client-supplied, for card messages) always wins; content
// that merely looks like a structured payload but arrives without a hint —
// a missing/old client, or a future card type nobody wired a hint for yet —
// falls back to a generic label. Raw JSON must never reach a push body.
func computePreview(msgType int32, content, hint string) string {
	if msgType == 2 {
		return "傳送了一張圖片"
	}
	if h := strings.TrimSpace(hint); h != "" {
		return truncatePreview(h)
	}
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return "傳送了一張卡片"
	}
	return truncatePreview(trimmed)
}

func truncatePreview(s string) string {
	if utf8.RuneCountInString(s) > PreviewMaxLength {
		return string([]rune(s)[:PreviewMaxLength]) + "..."
	}
	return s
}

// 49 §6.6 im/S4：圖片訊息 content 為 objectKey，讀取/WS 路徑一律直送裸 key，
// 顯示 URL 由組裝點統一組（REST=Gateway、WS=接收端前端 buildImageUrl）；服務側不組 URL。

// ChatMessageSentEvent is the chat.message.sent MQ payload published on every
// persisted chat message. The JSON field names are the wire contract consumed by
// notification-service notification_templates (title/body/data_fields/payload_filter).
// IDs are strings because TSIDs exceed JSON's safe-integer range (the consumer
// decodes into map[string]interface{}, which would lose precision as float64).
type ChatMessageSentEvent struct {
	MessageID      string `json:"message_id"`
	ConversationID string `json:"conversation_id"`
	SenderID       string `json:"sender_id"`
	SenderName     string `json:"sender_name"`
	SenderAvatar   string `json:"sender_avatar"`
	ReceiverID     string `json:"receiver_id"`
	ReceiverType   string `json:"receiver_type"` // "buyer" | "seller"
	StoreID        string `json:"store_id"`      // raw, unresolved store_id of this room's seller side (issue #2707) — NOT gated to ReceiverType=="seller"; always the room's seller surrogate, present on every message regardless of who received it
	ContentPreview string `json:"content_preview"`
	SentAt         string `json:"sent_at"` // RFC3339, lets the client measure end-to-end latency
}

// ChatEventPublisher publishes chat domain events for async fan-out (offline push).
type ChatEventPublisher interface {
	PublishChatMessageSent(ctx context.Context, e ChatMessageSentEvent)
}

// MessageUseCase handles chat message business logic.
type MessageUseCase struct {
	msgRepo       domain.MessageRepository
	convRepo      domain.ConversationRepository
	hub           domain.Hub
	buyerClient   BuyerClient
	sellerClient  SellerClient
	orderClient   OrderClient
	chatPublisher ChatEventPublisher
	mediaClient   MediaClient
	rdb           *redis.Client
	syncLimiter   *ratelimit.Limiter
	genID         func() int64
}

// PrincipalResolution is the outcome of asking a role service to map one of
// its own surrogate ids (buyer_id / store_id) back to the login principal.
//
// It exists so a WRITE or GATE path can tell "the id is not mine" (Found=false)
// apart from "I could not answer" (a returned error) apart from a real
// resolution — a three-way distinction ResolvePrincipal deliberately flattens
// into a single int64 for read paths, because seller-service / buyer-service
// answer "unknown id" by echoing the input straight back.
//
// Mirrors social-service's identically named type (usecase/account_client.go);
// the two services resolve against the same two RPCs.
type PrincipalResolution struct {
	AccountID int64
	Found     bool
}

type BuyerClient interface {
	// GetProfileBatch 回傳查得的 profile 與**查詢是否失敗**。兩者獨立：
	// map 可能已含快取命中的項目，同時 err != nil 代表未命中的那批沒查成。
	// 呼叫端必須自己決定降級行為（顯示空字串／擋下操作），不得忽略 err
	// —— 「查無此人」與「查不到」是兩回事，混為一談會把基礎設施故障
	// 靜默轉成業務結論（#2830 群組 5）。
	GetProfileBatch(ctx context.Context, buyerIDs []int64) (map[int64]BuyerProfileInfo, error)
	// ResolvePrincipal is the READ-path helper: it flattens "not a live
	// buyer_id" into an echo-back of the input, so a caller cannot tell a real
	// resolution from a failed one. Never use it to compare two identities —
	// see PrincipalResolution.
	ResolvePrincipal(ctx context.Context, buyerID int64) int64
	// ResolvePrincipalStrict is the WRITE/GATE-path helper (#2789).
	ResolvePrincipalStrict(ctx context.Context, buyerID int64) (PrincipalResolution, error)
	ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error)
}

type SellerClient interface {
	// GetProfileBatch 語義同 BuyerClient.GetProfileBatch：map 與 err 獨立，
	// 呼叫端必須明示降級決定。
	GetProfileBatch(ctx context.Context, storeIDs []int64) (map[int64]SellerProfileInfo, error)
	// ResolvePrincipal is the READ-path helper — same caveat as
	// BuyerClient.ResolvePrincipal.
	ResolvePrincipal(ctx context.Context, storeID int64) int64
	// ResolvePrincipalStrict is the WRITE/GATE-path helper (#2789).
	ResolvePrincipalStrict(ctx context.Context, storeID int64) (PrincipalResolution, error)
	ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error)
}

// BuyerProfileInfo holds buyer display info.
type BuyerProfileInfo struct {
	Nickname string
	Avatar   string
}

// SellerProfileInfo holds seller display info.
type SellerProfileInfo struct {
	Nickname  string
	Avatar    string
	StoreName string
	LogoKey   string
	// StoreCode：賣家 canonical 路徑短碼（sellers.store_code）。與 StoreName 同源。
	StoreCode string
	// StoreClosed：該 stint 已終止（sellers.status == closed；② read-only tombstone）。
	StoreClosed bool
}

// OrderParties holds the buyer/store identity of an order, used to validate
// the order-context conversation-binding intent (14 號 §6.5.1 路徑①).
type OrderParties struct {
	BuyerID int64
	StoreID int64
}

// OrderClient defines the interface for the seller-suspension IM order-binding
// lookups (14 號 §6.5.1, 46 號 §4.4). Both RPCs are internal-only on the
// order-service side.
type OrderClient interface {
	// GetOrderParties fetches an order's buyer/store identity by order_no, used
	// to validate the "訂單情境發起" three-way match. Any error means "cannot
	// verify" — callers must not bind.
	GetOrderParties(ctx context.Context, orderNo string) (OrderParties, error)
	// HasOrderBetween reverse-looks-up whether a "related" order (status NOT IN
	// (0,80), §6.5.2 — the range judgment is delegated entirely to order-service,
	// no Go-side status-code duplication) exists between the pair. Any error
	// means "cannot verify."
	HasOrderBetween(ctx context.Context, buyerID, storeID int64) (has bool, latestOrderNo string, err error)
}

// NewMessageUseCase creates a new MessageUseCase.
func NewMessageUseCase(msgRepo domain.MessageRepository, convRepo domain.ConversationRepository, hub domain.Hub, buyerClient BuyerClient, sellerClient SellerClient, orderClient OrderClient, chatPublisher ChatEventPublisher, mediaClient MediaClient, rdb *redis.Client, cfg *config.Config) *MessageUseCase {
	return &MessageUseCase{
		msgRepo: msgRepo, convRepo: convRepo, hub: hub, buyerClient: buyerClient,
		sellerClient: sellerClient, orderClient: orderClient, chatPublisher: chatPublisher,
		mediaClient: mediaClient, rdb: rdb,
		syncLimiter: ratelimit.New(rdb, 0, zap.L()),
		genID:       tsid.NewGenerator(),
	}
}

// BuildRoomID creates a room ID from the two chat wire identities (buyer_id /
// store_id, both CURRENT stint surrogates — never raw login account_id).
func BuildRoomID(buyerID, sellerID int64) string {
	return fmt.Sprintf("b%d_s%d", buyerID, sellerID)
}

// ParseRoomID parses "b{buyerID}_s{sellerID}" back into those two wire identities.
func ParseRoomID(roomID string) (buyerID, sellerID int64, err error) {
	parts := strings.SplitN(roomID, "_s", 2)
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "b") {
		return 0, 0, fmt.Errorf("invalid roomId format: %s", roomID)
	}
	buyerID, err = strconv.ParseInt(parts[0][1:], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid buyerID in roomId: %s", roomID)
	}
	sellerID, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid sellerID in roomId: %s", roomID)
	}
	return buyerID, sellerID, nil
}

// 帳號類型代碼
const (
	accountTypeBuyer  = 1 // BUYER
	accountTypeSeller = 2 // SELLER
)

// authorizeRoomAccess enforces that (userID, loginRole) is the buyer or
// seller side of roomID. Role is matched explicitly to side — not just ID
// equality — so a buyer_id/store_id numeric collision across the two
// surrogate namespaces can never grant cross-role access (#2499). This is a
// read-path-only check for the sync/gap-fill flow; SendMessage's own
// participant check above is untouched — redoing it is an explicit non-goal
// (#2499), its check is only a consistency baseline.
func authorizeRoomAccess(roomID string, userID int64, loginRole int32) error {
	buyerID, sellerID, err := ParseRoomID(roomID)
	authorized := false
	if err == nil {
		switch loginRole {
		case accountTypeBuyer:
			authorized = userID == buyerID
		case accountTypeSeller:
			authorized = userID == sellerID
		}
	}
	if !authorized {
		return fmt.Errorf("room not authorized for this caller")
	}
	return nil
}

// syncRateLimit/syncRateWindow bound per-actor sync frames (#2499); draft
// values, tune after observing real dev traffic.
const (
	syncRateLimit  = 5
	syncRateWindow = time.Second
)

// allowSyncFrame reports whether userID may process one more sync frame right
// now. Degrade-open: authorizeRoomAccess is the real security control here,
// this is only an amplification guard — an infra blip must not stop sync
// (consistent with the IM D5 degrade-open philosophy used for ws-ticket
// minting). A nil syncLimiter (MessageUseCase built via struct literal, as
// most existing tests do) never rate-limits.
func (uc *MessageUseCase) allowSyncFrame(ctx context.Context, userID int64) bool {
	if uc.syncLimiter == nil {
		return true
	}
	return uc.syncLimiter.Allow(ctx, fmt.Sprintf("im:sync:%d", userID), syncRateLimit, syncRateWindow)
}

// chatRoleFromAccountType maps account_type code to chat role.
// Returns "buyer", "seller", or "" (unknown, for SELLER_AND_BUYER without explicit role).
func chatRoleFromAccountType(accountType int32) string {
	switch accountType {
	case accountTypeBuyer:
		return "buyer"
	case accountTypeSeller:
		return "seller"
	default:
		return ""
	}
}

// SendMessageOption configures optional per-send behavior (order-context binding
// intent, seller-suspension gating — 14 號 §6.5). Implemented as functional
// options so the ~30 existing positional call sites (mostly tests) are
// unaffected; only the production gRPC entry point (grpc/server.go) passes any.
type SendMessageOption func(*sendMessageOptions)

type sendMessageOptions struct {
	orderNoHint     string
	sellerSuspended bool
	previewHint     string
}

// WithOrderNoHint carries the client-supplied order_no "intent parameter"
// (14 號 §6.5.1 路徑①：訂單情境發起). This is never trusted directly — it only
// seeds a server-side three-way validation against order-service; the binding
// itself is always server-derived.
func WithOrderNoHint(orderNo string) SendMessageOption {
	return func(o *sendMessageOptions) { o.orderNoHint = orderNo }
}

// WithSellerSuspended flags that the caller is a currently-suspended seller
// (Gateway `X-Seller-Suspended` header, forwarded to im-service via gRPC
// metadata and read at the transport boundary in grpc/server.go). Only takes
// effect when the message sender is the seller side of the room (§6.5.3).
func WithSellerSuspended(suspended bool) SendMessageOption {
	return func(o *sendMessageOptions) { o.sellerSuspended = suspended }
}

// WithPreviewHint carries the client-computed, human-readable one-line
// summary for this message (e.g. "傳送了一張商品卡片"). Card messages
// (product/order) are opaque JSON to im-service — it never parses their
// semantics — so the client, which already renders the card, supplies this
// string via SendMessageRequest.extra["preview"]. Empty hint falls back to
// computePreview's generic JSON-sniff safety net.
func WithPreviewHint(preview string) SendMessageOption {
	return func(o *sendMessageOptions) { o.previewHint = preview }
}

// SendMessageByUsers persists a message given sender/receiver IDs and sender's account_type.
// Uses account_type to determine the correct buyer/seller position in room_id.
func (uc *MessageUseCase) SendMessageByUsers(ctx context.Context, senderID, receiverID int64, accountType, msgType int32, content string, opts ...SendMessageOption) (*gen.Message, error) {
	if senderID == receiverID {
		return nil, pkgerrors.NewBusiness(bizcode.SelfMessageNotAllowed, "不能傳訊息給自己")
	}
	// Try both roomID combinations to find existing conversation
	room1 := BuildRoomID(senderID, receiverID)
	room2 := BuildRoomID(receiverID, senderID)

	roomID := ""
	if _, err := uc.convRepo.GetByRoomID(ctx, room1); err == nil {
		roomID = room1
	} else if _, err := uc.convRepo.GetByRoomID(ctx, room2); err == nil {
		roomID = room2
	}

	// New conversation: determine room_id from sender's role
	if roomID == "" {
		role := chatRoleFromAccountType(accountType)
		switch role {
		case "seller":
			roomID = BuildRoomID(receiverID, senderID) // receiver=buyer, sender=seller
		default:
			roomID = BuildRoomID(senderID, receiverID) // sender=buyer (default)
		}
	}

	return uc.SendMessage(ctx, roomID, senderID, msgType, content, opts...)
}

// SendMessage persists a chat message and attempts real-time delivery to the receiver.
func (uc *MessageUseCase) SendMessage(ctx context.Context, roomID string, senderID int64, msgType int32, content string, opts ...SendMessageOption) (*gen.Message, error) {
	var o sendMessageOptions
	for _, opt := range opts {
		opt(&o)
	}
	// Content length guard (defense in depth behind the gateway binding, and the
	// only guard on the gRPC SendMessage path). Bound stored content to
	// MaxMessageContentLength characters so it can never trip the messages.content
	// CHECK; rune count matches Postgres char_length semantics. Returns a 400, not a panic.
	if utf8.RuneCountInString(content) > MaxMessageContentLength {
		return nil, pkgerrors.NewInvalidInput("訊息內容過長")
	}
	buyerID, sellerID, err := ParseRoomID(roomID)
	if err != nil {
		return nil, err
	}
	// 「room 的兩端是不是同一個人」必須拿反解析後的登入 principal 比，而反解析
	// 本身可能失敗。#2789：seller-service / buyer-service 對「查無此 id」的回答
	// 是把輸入原樣 echo 回來，所以用 ResolvePrincipal（回單一 int64）寫這道守門，
	// 解析失敗時比的是兩個未驗證的原始 id —— 同一個人的兩個 id 比出來不相等，
	// 守門靜默放行。改用 Strict 版本，把「不是我的 id」「我答不出來」「真的解出來了」
	// 三態分開，未解析即 fail-closed。
	//
	// client 為 nil 代表該軸沒接線（單元測試／未注入），不是執行期解析失敗：
	// 沿用今日語意用原始 id，不當成未解析。
	buyerPrincipal, buyerVerified := buyerID, uc.buyerClient == nil
	if uc.buyerClient != nil {
		res, rerr := uc.buyerClient.ResolvePrincipalStrict(ctx, buyerID)
		if rerr != nil {
			pkglogger.Warn(ctx, "cannot verify room identity: buyer principal lookup failed, refusing to send",
				zap.String("roomID", roomID), zap.Int64("buyerID", buyerID), zap.Error(rerr))
			return nil, pkgerrors.NewUnavailable("暫時無法驗證對話身分，請稍後再試")
		}
		if res.Found {
			buyerPrincipal, buyerVerified = res.AccountID, true
		}
	}
	sellerPrincipal, sellerVerified := sellerID, uc.sellerClient == nil
	if uc.sellerClient != nil {
		res, rerr := uc.sellerClient.ResolvePrincipalStrict(ctx, sellerID)
		if rerr != nil {
			pkglogger.Warn(ctx, "cannot verify room identity: seller principal lookup failed, refusing to send",
				zap.String("roomID", roomID), zap.Int64("sellerID", sellerID), zap.Error(rerr))
			return nil, pkgerrors.NewUnavailable("暫時無法驗證對話身分，請稍後再試")
		}
		if res.Found {
			sellerPrincipal, sellerVerified = res.AccountID, true
		}
	}
	if !buyerVerified || !sellerVerified {
		// 至少一端不是現存的 buyer_id / store_id（軟刪除的 stint，或根本不屬於
		// 任一身分空間）。此時 principal 比較不可信，不得退回原始 id 比對。
		pkglogger.Warn(ctx, "room identity unresolved; refusing to send (fail-closed)",
			zap.String("roomID", roomID),
			zap.Int64("buyerID", buyerID), zap.Bool("buyerVerified", buyerVerified),
			zap.Int64("sellerID", sellerID), zap.Bool("sellerVerified", sellerVerified))
		return nil, pkgerrors.NewInvalidInput("無效的 room_id：買家或賣家已不存在")
	}
	if buyerPrincipal == sellerPrincipal {
		return nil, pkgerrors.NewInvalidInput("無效的 room_id：buyer 和 seller 不能是同一人")
	}

	var receiverID int64
	switch senderID {
	case buyerID:
		receiverID = sellerID
	case sellerID:
		receiverID = buyerID
	default:
		return nil, fmt.Errorf("sender %d is not a participant of room %s", senderID, roomID)
	}

	// 賣家停權訂單關聯 gating + 綁定解析（14 號 §6.5）。resolveOrderBinding 同時服務
	// 兩種呼叫者：非停權時單純「有 hint 就嘗試綁定」（best-effort，從不擋信）；
	// 停權賣家送訊時「必須已綁定或本次可綁定，否則擋」（§6.5.3）。roomID 的 buyer/seller
	// 即對話雙端，一經綁定成立即隱含「order.seller==sender AND order.buyer==對話另一端」
	// （綁定寫入本身就是這個三方一致驗證的結果），故此處不需重複比對。
	var orderNoToBind string
	if o.orderNoHint != "" || (o.sellerSuspended && senderID == sellerID) {
		var bound bool
		bound, orderNoToBind = uc.resolveOrderBinding(ctx, roomID, buyerID, sellerID, o.orderNoHint)
		if o.sellerSuspended && senderID == sellerID && !bound && orderNoToBind == "" {
			return nil, pkgerrors.NewBusiness(bizcode.SellerSuspendedOperationBlocked, "賣家帳號停權中，無法執行此操作")
		}
	}

	// ② read-only tombstone：closed 店的對話雙向禁新訊息（買家側看得到歷史與
	// tombstone 店名 — GetSellerBatch reshape 後為 PK lookup — 但不能再送）。
	// ProfileCache 5-min TTL 界定關店後的傳播窗口；查不到 profile 時 fail-open。
	if uc.sellerClient != nil {
		profiles, err := uc.sellerClient.GetProfileBatch(ctx, []int64{sellerID})
		// 先看資料再看 err：介面契約明訂「map 可能已含快取命中的項目，同時
		// err != nil」（未命中的那批沒查成）。用 else-if 會讓 err 覆蓋掉手上
		// 已經有的答案 —— 今天單 id 查詢下 err 必然伴隨 cache miss 所以無害，
		// 但一旦有人把這道守門批次化就會靜默漏判關店的店家。
		if p, ok := profiles[sellerID]; ok && p.StoreClosed {
			return nil, pkgerrors.NewBusiness(bizcode.StoreClosedMessageNotAllowed, "此商店已停業，無法傳送訊息")
		} else if err != nil {
			// 明示的 fail-open：查不到店家狀態時放行送出。
			// 取捨理由——擋下來的代價是 seller-service 一抖動全站 IM 送不出，
			// 放行的代價是少數訊息落進唯讀墓碑對話（對方本來就回不了）。
			// 這是刻意選的，不是把錯誤吞掉的副作用；改動前先重讀這段。
			pkglogger.Warn(ctx, "store-closed guard degraded: seller profile lookup failed, allowing send",
				zap.Int64("sellerID", sellerID), zap.Error(err))
		}
	}

	// B-135: bidirectional block check. Either party blocking the other
	// halts new messages with a 4xx (D10: not silent shadow ban).
	if blocklist.IsBlocked(ctx, uc.rdb, senderID, receiverID) {
		return nil, pkgerrors.NewBusiness(bizcode.BlockedMessageNotAllowed, "無法送出訊息")
	}

	// Best-effort UX guard: refuse new messages to a deleted account. receiverID is a
	// buyer_id/store_id surrogate, and RedactDeletedAccount writes one marker per
	// historical identity precisely so this lookup hits. The marker is
	// set by the account.deleted consumer (RedactDeletedAccount); checking it — not
	// profile presence — decouples the send path from profile-service health (a
	// missing profile alone can't distinguish "deleted" from "fetch error"). This is
	// NOT a compliance invariant: the marker lives in Redis (LRU-evictable, no HA)
	// and Redis errors fail-open, so a stray message to a deleted account is possible
	// — harmless, since that account's own PII is already purged. Never block a live
	// send on a flaky guard lookup.
	if uc.rdb != nil {
		if n, _ := uc.rdb.Exists(ctx, deletedAccountKey(receiverID)).Result(); n > 0 {
			return nil, pkgerrors.NewBusiness(bizcode.ReceiverAccountDeleted, "對方帳號已不存在，無法傳送訊息")
		}
	}

	// B-135: profanity dictionary on TEXT messages only. IMAGE skips.
	if msgType == 1 {
		if hit, _ := moderation.Check(content); hit {
			return nil, pkgerrors.NewBusiness(bizcode.ModerationRejected, "內容不符社群守則")
		}
	}

	// IMAGE: finalize staged token → asset URL（pre-generate ID 供 FinalizeAndLink 建立引用）
	msgID := uc.genID()
	if msgType == 2 {
		info, fErr := uc.mediaClient.FinalizeAndLink(ctx, content, media.RefTypeChatMessage, fmt.Sprintf("%d", msgID))
		if fErr != nil {
			return nil, fmt.Errorf("圖片處理失敗: %w", fErr)
		}
		if info.ObjectKey == "" {
			return nil, fmt.Errorf("圖片處理失敗: no asset returned")
		}
		// 存 objectKey（49 §5.3 #2；messages.content 語意中性名不 rename）；
		// 顯示 URL 於各讀取路徑（WS/gRPC）現組，wire 契約仍為完整 URL。
		content = info.ObjectKey
	}

	// 卡片訊息（product/order）的 preview 由 client 端 extra["preview"] 提供
	// （o.previewHint）；其餘情況（純文字、圖片、缺 hint 的呼叫）由
	// computePreview 提供唯一權威 fallback，寫入當下算好、存起來，後續讀取
	// （推播 body、conversations 列）一律沿用這個值，不再各自對 content 猜測。
	previewText := computePreview(msgType, content, o.previewHint)

	now := time.Now()
	isRead := false
	msg, err := uc.msgRepo.CreateWithLock(ctx, gen.CreateMessageParams{
		ID:          msgID,
		RoomID:      roomID,
		SenderID:    senderID,
		ReceiverID:  receiverID,
		Type:        msgType,
		Content:     content,
		PreviewText: &previewText,
		CreatedAt:   &now,
		IsRead:      &isRead,
		SeqRoom:     roomID,
	})
	if err != nil {
		return nil, err
	}

	if err := uc.upsertConversation(ctx, roomID, buyerID, sellerID, msg.ID, msgType, content, previewText, now); err != nil {
		pkglogger.Error(ctx, "upsertConversation failed", zap.String("roomID", roomID), zap.Int64("buyerID", buyerID), zap.Int64("sellerID", sellerID), zap.Error(err))
	}

	// 訂單關聯綁定寫入（14 號 §6.5.1）：必須在 upsertConversation 之後，此時 conversations
	// row 保證已存在（UPDATE-only，見 BindOrder）。Best-effort：綁定失敗不影響已送達的訊息。
	if orderNoToBind != "" {
		if err := uc.convRepo.BindOrder(ctx, roomID, orderNoToBind, now); err != nil {
			pkglogger.Warn(ctx, "bind conversation order context failed (best-effort)",
				zap.String("roomID", roomID), zap.String("orderNo", orderNoToBind), zap.Error(err))
		}
	}

	// 遞增 receiver 的 unread count
	if receiverID == buyerID {
		_ = uc.convRepo.IncrementBuyerUnread(ctx, roomID)
	} else {
		_ = uc.convRepo.IncrementSellerUnread(ctx, roomID)
	}

	// Determine receiver's role in this room for targeted WS push
	var receiverRole int32
	if receiverID == buyerID {
		receiverRole = 1 // buyer
	} else {
		receiverRole = 2 // seller
	}

	// Fetch sender display name/avatar for WS and FCM payload.
	// ProfileCache (5-min TTL in-memory) makes this O(1) on cache hit.
	// Reading from a nil map returns zero value — no extra nil check needed.
	var senderName, senderAvatar string
	if senderID == buyerID {
		if uc.buyerClient != nil {
			profiles, err := uc.buyerClient.GetProfileBatch(ctx, []int64{senderID})
			if err != nil {
				pkglogger.Warn(ctx, "sender display degraded: buyer profile lookup failed",
					zap.Int64("senderID", senderID), zap.Error(err))
			}
			p := profiles[senderID]
			senderName = p.Nickname
			senderAvatar = p.Avatar
		}
	} else if uc.sellerClient != nil {
		profiles, err := uc.sellerClient.GetProfileBatch(ctx, []int64{senderID})
		if err != nil {
			pkglogger.Warn(ctx, "sender display degraded: seller profile lookup failed",
				zap.Int64("senderID", senderID), zap.Error(err))
		}
		p := profiles[senderID]
		senderName = p.StoreName
		senderAvatar = p.LogoKey
		if senderAvatar == "" {
			senderAvatar = p.Avatar
		}
	}

	// Dual delivery: always attempt both WS and FCM, client deduplicates via seq_id.
	// This guarantees zero message loss regardless of online detection accuracy.
	// 49 §6.6 im/S4：WS payload 只帶 objectKey（content 為裸 key），組裝點＝接收端前端
	// buildImageUrl；發送服務不組 URL，與 REST（Gateway 組）共用 objectKey 單一 SSOT。
	wsContent := content
	uc.hub.SendToUserRole(receiverID, receiverRole, &domain.Message{
		Type:         "chat",
		MsgType:      msgType,
		RoomID:       roomID,
		From:         senderID,
		To:           receiverID,
		Content:      wsContent,
		MessageID:    msg.ID,
		SeqID:        msg.SeqID,
		Timestamp:    now.UnixMilli(),
		SenderName:   senderName,
		SenderAvatar: senderAvatar,
	})

	// Push authoritative unread count to receiver so the client can drop its poll.
	uc.pushUnreadChanged(ctx, receiverID, receiverRole)

	if uc.chatPublisher != nil {
		var receiverType string
		if receiverID == buyerID {
			receiverType = "buyer"
		} else {
			receiverType = "seller"
		}

		// receiver_id 這一個 payload 欄位同時被 chat.message.sent 的買家 / 賣家
		// 兩張 notification template 讀，靠 payload_filter receiver_type 分流
		// （單一事件只會命中其中一張），所以要各自填對應身分空間的值 ——
		// notifications.user_id 的讀取端是不對稱的（gateway resolveInboxUserID）：
		// 賣家收件匣以 account_id 查、買家收件匣以現任 buyer_id 查。
		//   seller → sellerPrincipal（store_id 解析後的 account_id）
		//   buyer  → buyerID 原樣（roomID 內的 buyer_id 就是讀取端要的鍵；
		//            填 buyerPrincipal 等於寫一列買家永遠查不到的 inbox row）
		receiverPublishID := receiverID
		if receiverID == sellerID {
			receiverPublishID = sellerPrincipal
		}

		event := ChatMessageSentEvent{
			MessageID:      strconv.FormatInt(msg.ID, 10),
			ConversationID: roomID,
			SenderID:       strconv.FormatInt(senderID, 10),
			SenderName:     senderName,
			SenderAvatar:   senderAvatar,
			ReceiverID:     strconv.FormatInt(receiverPublishID, 10),
			ReceiverType:   receiverType,
			StoreID:        strconv.FormatInt(sellerID, 10),
			ContentPreview: previewText,
			SentAt:         timeutil.FormatISO8601(now),
		}
		// Async best-effort: real-time delivery already happened via WebSocket above.
		// The published event drives the offline FCM push (visible notification) in
		// notification-service, reusing its template + token + retry pipeline.
		go uc.chatPublisher.PublishChatMessageSent(context.Background(), event)
	}

	return &msg, nil
}

func (uc *MessageUseCase) upsertConversation(ctx context.Context, roomID string, buyerID, sellerID, msgID int64, msgType int32, content, previewText string, now time.Time) error {
	_, err := uc.convRepo.Upsert(ctx, gen.UpsertConversationParams{
		ID:                 uc.genID(),
		RoomID:             roomID,
		BuyerID:            buyerID,
		StoreID:            sellerID,
		LastMessageID:      &msgID,
		LastMessageContent: &content,
		LastMessageType:    msgType,
		LastMessagePreview: &previewText,
		UpdatedAt:          &now,
	})
	return err
}

// resolveOrderBinding determines the seller-suspension order-binding state for a
// room (14 號 §6.5.1). Returns bound=true when the conversation is already
// bound (nothing new to write). Otherwise returns orderNoToBind (non-empty)
// when a legitimate new binding was resolved this call — the caller persists it
// via BindOrder *after* the conversation row is guaranteed to exist (upsert has
// already run). Both order-service calls are best-effort lookups: any error is
// "cannot verify," which the §6.5.3 gating caller must treat as deny, not
// fail-open — this function surfaces that by returning (false, "").
func (uc *MessageUseCase) resolveOrderBinding(ctx context.Context, roomID string, buyerID, sellerID int64, orderNoHint string) (bound bool, orderNoToBind string) {
	if conv, err := uc.convRepo.GetByRoomID(ctx, roomID); err == nil && conv.OrderBindingVerifiedAt != nil {
		return true, ""
	}
	if uc.orderClient == nil {
		return false, ""
	}

	// 路徑①：訂單情境發起 — order_no 是呼叫端帶入的「意圖參數」，從不直接信任；
	// 必須三方一致（order.buyer==roomID buyer AND order.seller==roomID seller）才可能綁定，
	// 這正是防「掛他人單號」的關鍵：攻擊者即使帶入自己名下、其他買家的真實單號，
	// 該單的 buyer 不等於「本對話」的 buyer，三方比對即失敗。
	if orderNoHint != "" {
		if parties, err := uc.orderClient.GetOrderParties(ctx, orderNoHint); err == nil &&
			parties.BuyerID == buyerID && parties.StoreID == sellerID {
			// 訂單狀態範圍判準（§6.5.2：NOT IN (0,80)）委由 order-service HasOrderBetween
			// 認定，避免 Go 端重複硬編碼狀態碼；同時再次確認這對買賣雙方仍有「關聯」訂單
			// （防止 hint 指向的單已是唯一一筆但已被後續狀態變化排除在關聯範圍外的邊界情況）。
			if has, _, herr := uc.orderClient.HasOrderBetween(ctx, buyerID, sellerID); herr == nil && has {
				return false, orderNoHint
			}
		}
	}

	// 路徑②：既有對話回填 — 反查買賣雙方之間是否存在關聯訂單。
	if has, latestOrderNo, err := uc.orderClient.HasOrderBetween(ctx, buyerID, sellerID); err == nil && has {
		return false, latestOrderNo
	}

	return false, ""
}

// GetHistory returns message history for a room.
// If beforeID > 0, returns messages before that ID (cursor-based pagination).
func (uc *MessageUseCase) GetHistory(ctx context.Context, roomID string, beforeID int64, limit int32) ([]gen.Message, error) {
	if limit <= 0 || limit > MaxHistoryLimit {
		limit = DefaultHistoryLimit
	}
	if beforeID > 0 {
		return uc.msgRepo.ListBefore(ctx, gen.ListMessagesBeforeParams{
			RoomID: roomID,
			ID:     beforeID,
			Limit:  limit,
		})
	}
	return uc.msgRepo.List(ctx, gen.ListMessagesParams{
		RoomID: roomID,
		Limit:  limit,
		Offset: 0,
	})
}

// ConversationWithProfile enriches a conversation with the target user's profile.
type ConversationWithProfile struct {
	gen.Conversation
	TargetNickname string `json:"target_nickname"`
	TargetAvatar   string `json:"target_avatar"`
	StoreName      string `json:"store_name"`
	LogoKey        string `json:"logoKey"`
	StoreCode      string `json:"store_code"`
	TargetUserID   int64  `json:"target_user_id"`
	UnreadCount    int64  `json:"unread_count"`
}

func (uc *MessageUseCase) GetConversations(ctx context.Context, wireID int64, accountType, page, pageSize int32) ([]ConversationWithProfile, error) {
	_, pageSize, offset := pagination.NormalizePage(page, pageSize, DefaultConversationsPageSize, MaxConversationsPageSize)

	role := chatRoleFromAccountType(accountType)
	var convs []gen.Conversation
	var err error
	switch role {
	case "buyer":
		convs, err = uc.convRepo.ListByBuyer(ctx, gen.ListConversationsByBuyerParams{
			BuyerID: wireID,
			Limit:   pageSize,
			Offset:  offset,
		})
	case "seller":
		convs, err = uc.convRepo.ListBySeller(ctx, gen.ListConversationsBySellerParams{
			StoreID: wireID,
			Limit:   pageSize,
			Offset:  offset,
		})
	default:
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// B-135: drop conversations where the viewer is blocked from / has blocked
	// the other participant (D10: list-level hide entire conversation).
	filtered := convs[:0]
	for _, conv := range convs {
		buyerID, sellerID, parseErr := ParseRoomID(conv.RoomID)
		if parseErr != nil {
			continue
		}
		other := buyerID
		if wireID == buyerID {
			other = sellerID
		}
		if blocklist.IsBlocked(ctx, uc.rdb, wireID, other) {
			continue
		}
		filtered = append(filtered, conv)
	}
	convs = filtered

	// Collect target IDs from room IDs
	var buyerIDs []int64
	var sellerIDs []int64
	type target struct {
		id      int64
		isBuyer bool
	}
	targets := make([]target, len(convs))
	for i, conv := range convs {
		buyerID, sellerID, parseErr := ParseRoomID(conv.RoomID)
		if parseErr != nil {
			continue
		}
		// The target is the other participant (not the current user)
		if wireID == buyerID {
			targets[i] = target{id: sellerID, isBuyer: false}
			sellerIDs = append(sellerIDs, sellerID)
		} else {
			targets[i] = target{id: buyerID, isBuyer: true}
			buyerIDs = append(buyerIDs, buyerID)
		}
	}

	// Batch fetch buyer profiles
	var buyerProfiles map[int64]BuyerProfileInfo
	if len(buyerIDs) > 0 && uc.buyerClient != nil {
		var err error
		buyerProfiles, err = uc.buyerClient.GetProfileBatch(ctx, buyerIDs)
		if err != nil {
			pkglogger.Warn(ctx, "conversation list degraded: buyer profiles partially unavailable",
				zap.Int("requested", len(buyerIDs)), zap.Int("resolved", len(buyerProfiles)), zap.Error(err))
		}
	}

	// Batch fetch seller profiles
	var sellerProfiles map[int64]SellerProfileInfo
	if len(sellerIDs) > 0 && uc.sellerClient != nil {
		var err error
		sellerProfiles, err = uc.sellerClient.GetProfileBatch(ctx, sellerIDs)
		if err != nil {
			pkglogger.Warn(ctx, "conversation list degraded: seller profiles partially unavailable",
				zap.Int("requested", len(sellerIDs)), zap.Int("resolved", len(sellerProfiles)), zap.Error(err))
		}
	}

	// Build enriched result
	result := make([]ConversationWithProfile, len(convs))
	for i, conv := range convs {
		result[i] = ConversationWithProfile{
			Conversation: conv,
			TargetUserID: targets[i].id,
		}
		if targets[i].isBuyer && buyerProfiles != nil {
			if p, ok := buyerProfiles[targets[i].id]; ok {
				result[i].TargetNickname = p.Nickname
				result[i].TargetAvatar = p.Avatar
			}
		}
		if !targets[i].isBuyer && sellerProfiles != nil {
			if p, ok := sellerProfiles[targets[i].id]; ok {
				result[i].TargetNickname = p.Nickname
				result[i].TargetAvatar = p.Avatar
				result[i].StoreName = p.StoreName
				result[i].LogoKey = p.LogoKey
				result[i].StoreCode = p.StoreCode
			}
		}
		// 從 conversation 欄位讀取 unread count（欄位化，不再 COUNT 查詢）
		if role == "buyer" {
			result[i].UnreadCount = int64(conv.BuyerUnreadCount)
		} else {
			result[i].UnreadCount = int64(conv.SellerUnreadCount)
		}
	}
	return result, nil
}

// ParticipantInfo holds a chat participant's profile for the history response.
type ParticipantInfo struct {
	Role        string
	DisplayName string
	AvatarKey   string
	// UserID 是該參與者的 chat wire 身份（買家側 buyer_id、賣家側 store_id，皆為
	// CURRENT stint 代理鍵），由 room_id 兩側拆出，不是登入 account_id。
	UserID int64
}

// GetParticipants returns profile info for both participants in a room.
func (uc *MessageUseCase) GetParticipants(ctx context.Context, roomID string) []ParticipantInfo {
	buyerID, sellerID, err := ParseRoomID(roomID)
	if err != nil {
		return nil
	}

	participants := make([]ParticipantInfo, 0, 2)

	// Fetch buyer profile
	if uc.buyerClient != nil {
		profiles, err := uc.buyerClient.GetProfileBatch(ctx, []int64{buyerID})
		if err != nil {
			pkglogger.Warn(ctx, "participant display degraded: buyer profile lookup failed",
				zap.Int64("buyerID", buyerID), zap.Error(err))
		}
		p := ParticipantInfo{UserID: buyerID, Role: "buyer"}
		if bp, ok := profiles[buyerID]; ok {
			p.DisplayName = bp.Nickname
			p.AvatarKey = bp.Avatar
		}
		participants = append(participants, p)
	}

	// Fetch seller profile
	if uc.sellerClient != nil {
		profiles, err := uc.sellerClient.GetProfileBatch(ctx, []int64{sellerID})
		if err != nil {
			pkglogger.Warn(ctx, "participant display degraded: seller profile lookup failed",
				zap.Int64("sellerID", sellerID), zap.Error(err))
		}
		p := ParticipantInfo{UserID: sellerID, Role: "seller"}
		if sp, ok := profiles[sellerID]; ok {
			p.DisplayName = sp.StoreName
			p.AvatarKey = sp.LogoKey
			if p.AvatarKey == "" {
				p.AvatarKey = sp.Avatar
			}
		}
		participants = append(participants, p)
	}

	return participants
}

// MarkMessagesByIDs marks specific messages as read, verifying the user is the receiver,
// and recalculates unread counts for the affected conversations.
func (uc *MessageUseCase) MarkMessagesByIDs(ctx context.Context, userID int64, messageIDs []int64) error {
	if err := uc.msgRepo.MarkByIDsAsRead(ctx, userID, messageIDs); err != nil {
		return err
	}
	_ = uc.msgRepo.RecalcUnreadForMessages(ctx, messageIDs)
	return nil
}

// MarkAllRead marks all unread messages as read for a user, filtered by role,
// and resets all corresponding unread counts.
func (uc *MessageUseCase) MarkAllRead(ctx context.Context, userID int64, accountType int32) error {
	switch chatRoleFromAccountType(accountType) {
	case "buyer":
		if err := uc.msgRepo.MarkAllAsReadByBuyer(ctx, userID); err != nil {
			return err
		}
		_ = uc.convRepo.ResetAllBuyerUnread(ctx, userID)
		uc.pushUnreadChanged(ctx, userID, accountTypeBuyer)
		return nil
	case "seller":
		if err := uc.msgRepo.MarkAllAsReadBySeller(ctx, userID); err != nil {
			return err
		}
		_ = uc.convRepo.ResetAllSellerUnread(ctx, userID)
		uc.pushUnreadChanged(ctx, userID, accountTypeSeller)
		return nil
	default:
		return nil
	}
}

// GetUnreadCount returns total unread message count for a user, filtered by role.
// Uses SUM of conversation unread_count columns (O(1) per conversation, no message scan).
func (uc *MessageUseCase) GetUnreadCount(ctx context.Context, userID int64, accountType int32) (int64, error) {
	switch chatRoleFromAccountType(accountType) {
	case "buyer":
		return uc.convRepo.SumBuyerUnreadCount(ctx, userID)
	case "seller":
		return uc.convRepo.SumSellerUnreadCount(ctx, userID)
	default:
		return 0, nil
	}
}

// SyncMessages returns messages with seq_id > lastSeq for a given room (gap-fill).
func (uc *MessageUseCase) SyncMessages(ctx context.Context, roomID string, lastSeq int64, limit int32) ([]gen.Message, error) {
	if limit <= 0 || limit > MaxSyncLimit {
		limit = DefaultSyncLimit
	}
	return uc.msgRepo.ListAfterSeq(ctx, gen.ListMessagesAfterSeqParams{
		RoomID: roomID,
		SeqID:  lastSeq,
		Limit:  limit,
	})
}

// MarkConversationRead marks all unread messages in a room as read for the reader,
// and resets the corresponding unread count on the conversation.
func (uc *MessageUseCase) MarkConversationRead(ctx context.Context, roomID string, readerID int64) error {
	if err := uc.msgRepo.MarkAsRead(ctx, gen.MarkMessagesAsReadParams{
		RoomID:     roomID,
		ReceiverID: readerID,
	}); err != nil {
		return err
	}
	// 根據 reader 是 buyer 還是 seller 歸零對應的 unread count
	buyerID, _, parseErr := ParseRoomID(roomID)
	if parseErr != nil {
		return nil
	}
	if readerID == buyerID {
		_ = uc.convRepo.ResetBuyerUnread(ctx, roomID)
		uc.pushUnreadChanged(ctx, readerID, accountTypeBuyer)
	} else {
		_ = uc.convRepo.ResetSellerUnread(ctx, roomID)
		uc.pushUnreadChanged(ctx, readerID, accountTypeSeller)
	}
	return nil
}

// HideMessage marks a message as hidden by platform moderation. Used by
// social-service AdminResolveReport for HIDE_TARGET on CHAT_MESSAGE (B-135).
func (uc *MessageUseCase) HideMessage(ctx context.Context, messageID int64, reason string) error {
	return uc.msgRepo.HideMessage(ctx, messageID, reason)
}

// deletedAccountKey is the Redis marker set when an account is deleted, read by
// SendMessage to refuse new messages to that account. One marker is written per
// historical identity (account_id plus every buyer_id/store_id ever bound to it)
// because the read path only ever holds a surrogate — see RedactDeletedAccount.
func deletedAccountKey(accountID int64) string {
	return fmt.Sprintf("im:account:deleted:%d", accountID)
}

// RedactDeletedAccount purges a deleted account's chat PII, driven by the
// account.deleted/buyer.deleted MQ events (Apple 5.1.1(v) + 個資法 right-to-erasure). Unlike the
// pre-D8 version, this enumerates EVERY historical buyer_id and store_id ever
// bound to accountID — not just the single id carried by the event — because
// a sender_id in messages may be a closed stint's surrogate that no longer
// equals accountID (buyer store-stint re-key W2/W3, D8). It (1) blanks
// the body of every message any of those identities sent so the read-path
// tombstone renders, (2) blanks the denormalized last_message preview on
// conversations whose last message was theirs, (3') releases the media-service
// references for their chat images so GC erases the OSS blobs, and (3) marks
// every one of those identities deleted so SendMessage refuses new messages to
// them (one marker per identity — the send path resolves receivers to surrogates,
// so an accountID-only marker never matches once the two diverge). Steps 1–2–3'
// are the compliance-critical purge and abort on error (the consumer requeues
// -> DLQ, including if historical-ID enumeration itself fails — see R4/R5
// tests); the marker is best-effort. Idempotent: safe to replay.
func (uc *MessageUseCase) RedactDeletedAccount(ctx context.Context, accountID int64) error {
	buyerIDs, err := uc.buyerClient.ListIDsByAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("enumerate historical buyer_ids for account %d: %w", accountID, err)
	}
	storeIDs, err := uc.sellerClient.ListIDsByAccount(ctx, accountID)
	if err != nil {
		return fmt.Errorf("enumerate historical store_ids for account %d: %w", accountID, err)
	}
	senderIDs := mergeUniqueIDs(accountID, buyerIDs, storeIDs)

	n, err := uc.msgRepo.RedactBySender(ctx, senderIDs)
	if err != nil {
		return fmt.Errorf("redact messages by senders %v: %w", senderIDs, err)
	}
	if err := uc.convRepo.RedactLastMessageBySender(ctx, senderIDs, deletedMessagePlaceholder); err != nil {
		return fmt.Errorf("redact conversation previews for senders %v: %w", senderIDs, err)
	}
	imageMsgIDs, err := uc.msgRepo.ListImageMessageIDsBySender(ctx, senderIDs)
	if err != nil {
		return fmt.Errorf("list image messages for senders %v: %w", senderIDs, err)
	}
	if len(imageMsgIDs) > 0 {
		refIDs := make([]string, len(imageMsgIDs))
		for i, id := range imageMsgIDs {
			refIDs[i] = strconv.FormatInt(id, 10)
		}
		// Fail-closed: releasing the media references (so GC erases the OSS blobs)
		// is the compliance lever; on error the consumer retries → DLQ. Idempotent.
		if err := uc.mediaClient.ReleaseChatReferences(ctx, refIDs); err != nil {
			return fmt.Errorf("release chat image references for senders %v: %w", senderIDs, err)
		}
	}
	if uc.rdb != nil {
		// Mark EVERY historical identity, not just accountID: SendMessage looks the
		// marker up by receiverID, which is a buyer_id/store_id surrogate decoded from
		// the roomID — never the login account_id. Keying on accountID alone made the
		// guard a permanent no-op for any account whose surrogates have diverged from
		// it (identity separation, 2026-09-02), while still passing on grandfathered
		// accounts where the two happen to be equal.
		for _, id := range senderIDs {
			if err := uc.rdb.Set(ctx, deletedAccountKey(id), 1, 0).Err(); err != nil {
				pkglogger.Warn(ctx, "set deleted-account marker failed (send guard weakened, purge intact)",
					zap.Int64("markerID", id), zap.Int64("accountID", accountID), zap.Error(err))
			}
		}
	}
	pkglogger.Info(ctx, "redacted chat PII for deleted account",
		zap.Int64("accountID", accountID), zap.Int64s("senderIDs", senderIDs), zap.Int64("messagesRedacted", n))
	return nil
}

func mergeUniqueIDs(accountID int64, buyerIDs, storeIDs []int64) []int64 {
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

// pushUnreadChanged queries the authoritative full unread count for the given
// user+role and pushes an unread_changed WebSocket event to all of that user's
// connections. Single IM pod => the in-memory hub reaches every device.
// Failures are logged and swallowed: this must never break the SendMessage /
// MarkRead path. The client can always resync on the next event or reconnect.
func (uc *MessageUseCase) pushUnreadChanged(ctx context.Context, userID int64, role int32) {
	var (
		total int64
		err   error
	)
	switch role {
	case accountTypeBuyer:
		total, err = uc.convRepo.SumBuyerUnreadCount(ctx, userID)
	case accountTypeSeller:
		total, err = uc.convRepo.SumSellerUnreadCount(ctx, userID)
	default:
		return
	}
	if err != nil {
		pkglogger.Error(ctx, "pushUnreadChanged: sum unread failed",
			zap.Int64("userID", userID), zap.Int32("role", role), zap.Error(err))
		return
	}
	uc.hub.SendToUserRole(userID, role, &domain.Message{
		Type:        "unread_changed",
		Role:        role,
		UnreadCount: &total,
		Timestamp:   time.Now().UnixMilli(),
	})
}
