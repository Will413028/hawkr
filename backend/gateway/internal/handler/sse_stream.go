// SSE stream handlers for the realtime push platform (41 號設計書 §12)。
// 兩個 endpoint：
//   - GET /api/v1/public/posts/{post_id}/comments/stream — 匿名 + per-subject cap
//   - GET /api/v1/app/notifications/stream                — JWT + per-user cap（身分為收件匣身分，非 JWT account_id）
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/realtime"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/gateway/internal/sse"
	"github.com/gin-gonic/gin"
)

// StreamPostComments 訂閱單一貼文留言 SSE 流。
// @Summary      訂閱貼文留言 SSE
// @Description  Server-Sent Events 推送指定貼文的留言新增 / 編輯 / 刪除 / 按讚變更事件。匿名可訂閱，同一限流身分連線上限 5。瀏覽器原生 EventSource 即可使用；reconnect 時自動帶 Last-Event-ID。本路徑回傳 text/event-stream，Connection 為長連線，不會回 JSON envelope。詳見 §41 設計書 §三 / §12.1。
// @Tags         留言 (Comment) - Realtime
// @Produce      text/event-stream
// @Param        post_id        path    string  true  "貼文 ID"
// @Param        Last-Event-ID  header  string  false "上次收到的事件 ID，用於斷線重連 replay"
// @Success      200  "text/event-stream（持續推送 event: ready / comment.* / comment.batch / heartbeat）"
// @Failure      400  {object}  object{code=int,message=string}  "post_id 無效"
// @Failure      404  {object}  object{code=int,message=string}  "貼文不存在或已下架"
// @Failure      429  {object}  object{code=int,message=string}  "同一限流身分連線上限（回應帶 Retry-After / X-RateLimit-*）"
// @Failure      503  {object}  object{code=int,message=string}  "pod 連線上限或服務不可用"
// @Router       /public/posts/{post_id}/comments/stream [get]
func (h *Holder) StreamPostComments(c *gin.Context) {
	postIDStr := c.Param("id")
	postID, err := strconv.ParseInt(postIDStr, 10, 64)
	if err != nil || postID <= 0 {
		response.BadRequest(c, "invalid post id")
		return
	}

	// 404 pre-check (§8.1)：先確認貼文存在；不存在不應消耗 SSE 連線。
	// 匿名訂閱也允許（OptionalJWTAuth 路徑 / 直接無 JWT），這時 accountID 與兩個業務 id 都是 0。
	// GetPostDetail proto 的 post_id 已改 string（§4 前端可控 id），故傳原始字串；
	// topic key 仍以 int64 生成，與 SSE publish 端（social-service）保持一致。
	// 貼文讚永遠是本人身分，所以預檢的 is_liked 用 buyer_id；連線上限以 account_id 計；
	// 事件裡的 isMine 依這條連線觀看者此刻的 buyer_id、store_id 在寫出前算。
	accountID := getOptionalAccountID(c)
	caller, ok := h.socialCaller(c, accountID)
	if !ok {
		return
	}
	detail, err := h.Social.GetPostDetail(c.Request.Context(), caller.BuyerID, postIDStr)
	if err != nil {
		response.GrpcErr(c, "social", err)
		return
	}
	if detail == nil || detail.Code != 0 || detail.Data == nil {
		response.NotFound(c, "post not found")
		return
	}

	h.serveSSE(c, h.CommentChannel, realtime.TopicCommentPost(postID), accountID, h.CommentChannel.Personalizer(), sse.Viewer{BuyerID: caller.BuyerID, StoreID: caller.StoreID})
}

// StreamNotifications 訂閱使用者通知 SSE 流（僅 web 端使用，mobile 維持 FCM）。
// @Summary      訂閱通知 SSE
// @Description  Server-Sent Events 推送登入者的新 inbox 項目與未讀數變化。需 JWT；topic 綁定收件匣身分（買家 = 當前 buyer_id、賣家 = 當前 store_id），與 GET /app/notifications 同一個身分空間。per-user 連線上限 3。本路徑回傳 text/event-stream，不會回 JSON envelope。詳見 §41 設計書 §四 / §12.2。
// @Tags         通知 (Notification) - Realtime
// @Produce      text/event-stream
// @Security     BearerAuth
// @Param        Last-Event-ID  header  string  false "上次收到的事件 ID，用於斷線重連 replay"
// @Success      200  "text/event-stream（持續推送 event: ready / notification.new / notification.unread_changed / heartbeat）"
// @Failure      400  {object}  object{code=int,message=string}  "loginRole 非 buyer/seller"
// @Failure      401  {object}  object{code=int,message=string}  "未登入"
// @Failure      403  {object}  object{code=int,message=string}  "買家身分解析不到現任 buyer_id"
// @Failure      429  {object}  object{code=int,message=string}  "per-accountId 連線上限（回應帶 Retry-After / X-RateLimit-*）"
// @Failure      503  {object}  object{code=int,message=string}  "pod 連線上限或服務不可用"
// @Router       /app/notifications/stream [get]
func (h *Holder) StreamNotifications(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}

	// topic 必須用**收件匣的身分空間**建，不是 JWT 的 account_id。
	//
	// 發布端（realtime/notification_channel.go ResolveTopic）是從事件 payload 的
	// userId 解 topic，而那個值就是 notifications.user_id ⇒ 買家是當前 buyer_id、
	// 賣家是當前 store_id。發布端與訂閱端都固定使用這個 canonical 空間；歷史 retired
	// stint 不透過 account_id fallback 重新曝光，交由 lifecycle cleanup 處理。
	//
	// 解析後的 id 同時當作 serveSSE 的授權主體與 per-user 連線上限鍵，與
	// StreamPostComments 的既有作法一致（同一條連線的身分只有一個）。
	//
	// ⚠️ 授權在下面那兩行就決定完了：topic 與授權主體都出自同一個 streamUserID，
	// 所以 NotificationChannel.AuthorizeSubscribe 的 topic 比對在這條路徑上恆等。
	// **擋住越權訂閱的是「streamUserID 由 JWT 推導」這件事，不是那道比對**；那道比對
	// 守的是下一個呼叫點（理由見該函式的 doc）。要動這裡就是在動授權本身。
	userType, ok := resolveInboxUserType(c)
	if !ok {
		response.BadRequest(c, "invalid login role")
		return
	}
	streamUserID, ok := h.resolveInboxUserIDInt(c, accountID, userType)
	if !ok {
		return
	}

	topic := realtime.TopicNotificationUser(streamUserID)
	h.serveSSE(c, h.NotifChannel, topic, streamUserID, nil, sse.Viewer{})
}

// serveSSE is the shared subscribe-and-serve flow for any channel。personalizer 與 viewer 為這條連線專屬的 payload 個人化（nil = 共用）。
//  1. AuthorizeSubscribe — Channel-defined。⚠️ topic 與 accountID 都由呼叫端決定，
//     兩者同源時（StreamNotifications）它只驗得出參數一致，驗不出身分正確——身分的
//     正確性是呼叫端推導 accountID 時就要保證的事。
//  2. Hub.Subscribe — cap enforcement
//  3. ServeSSE — write loop
//  4. Hub.Unsubscribe in defer
func (h *Holder) serveSSE(c *gin.Context, ch realtime.Channel, topic string, accountID int64, personalizer sse.Personalizer, viewer sse.Viewer) {
	if err := ch.AuthorizeSubscribe(c.Request.Context(), topic, accountID); err != nil {
		response.Forbidden(c, "forbidden")
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Err(c, http.StatusInternalServerError, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	conn := sse.NewConnection(topic, c.Writer, flusher, c.GetHeader("Last-Event-ID"))
	conn.AccountID = accountID
	conn.Personalizer = personalizer
	conn.Viewer = viewer
	if accountID == 0 {
		// Anonymous connections are capped per rate-limit subject, not per peer
		// address: the website's SSR worker and the CI runner each present one
		// address for an unbounded number of people, so an address key would let
		// five of them exhaust the cap for everybody (01 設計書 §3.2.8).
		conn.Subject, _ = middleware.RateLimitSubject(c)
	}

	replay, bufferMiss, err := h.SSEHub.Subscribe(conn)
	if err != nil {
		switch {
		case errors.Is(err, sse.ErrPodCapReached):
			response.ServiceUnavailable(c, "sse pod capacity reached")
		case errors.Is(err, sse.ErrSubjectCapReached), errors.Is(err, sse.ErrUserCapReached):
			// This 429 never passes through the rate-limit middleware — SSE is
			// tierExempt and the middleware early-returns — so the retry hints
			// have to be written here. It is also the only 429 with a known
			// front-end consumer (the web SSE client, which otherwise falls back
			// to a fixed 8s wait), so skipping it would make the whole
			// Retry-After change a no-op for real users.
			perUser, perSubject := h.SSEHub.ConnCaps()
			connLimit := perSubject
			if errors.Is(err, sse.ErrUserCapReached) {
				connLimit = perUser
			}
			response.SetRateLimitHeaders(c, connLimit, 0, response.ConnCapRetryAfter)
			response.Err(c, http.StatusTooManyRequests, http.StatusTooManyRequests, "sse connection cap reached")
		default:
			response.Err(c, http.StatusInternalServerError, http.StatusInternalServerError, err.Error())
		}
		return
	}
	defer h.SSEHub.Unsubscribe(conn)

	sse.ServeSSE(c.Request.Context(), h.SSEHub, conn, c.Writer, replay, bufferMiss, sse.ServeOptions{}, nil)
}
