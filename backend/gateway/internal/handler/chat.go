package handler

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/httputil"
	"github.com/Will413028/hawkr/go-pkg/mediaurl"
	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	"github.com/Will413028/hawkr/go-pkg/timeutil"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  wsBufferSize,
	WriteBufferSize: wsBufferSize,
	CheckOrigin:     wsCheckOrigin,
}

// wsCheckOrigin defends the chat WebSocket upgrade against CSWSH. Browsers send
// an Origin header — it must be allowlisted (CORS_ALLOWED_ORIGINS). Native apps
// (Flutter) send no Origin, so they are allowed: mobile behavior is unchanged.
//
// 單體轉 WebSocket 時保留 Origin，而單體的 CSRF（http.CrossOriginProtection）不檢查 GET 的 upgrade，所以這道
// 檢查留在 gateway。CORS_ALLOWED_ORIGINS 是 web 與後台的完整 origin（逗號分隔），與單體允許的 origin 相同；
// 每次都讀環境變數，WebSocket 的握手頻率低，不需要快取。
func wsCheckOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, allowed := range strings.Split(os.Getenv("CORS_ALLOWED_ORIGINS"), ",") {
		if strings.TrimSpace(allowed) == origin {
			return true
		}
	}
	return false
}

// ChatWebSocket
// @Summary      建立 WebSocket 連線
// @Description  建立即時聊天的 WebSocket 連線，用於接收與發送即時訊息。
// @Description
// @Description  **WS 推送訊息格式（伺服器 → 客戶端）**
// @Description
// @Description  ```json
// @Description  {
// @Description    "type": "chat",
// @Description    "msgType": 1,
// @Description    "roomId": "b123_s456",
// @Description    "from": 123,
// @Description    "to": 456,
// @Description    "content": "你好",
// @Description    "messageId": 7459000000000000000,
// @Description    "seqId": 1,
// @Description    "timestamp": 1778399213260,
// @Description    "senderName": "測試買家",
// @Description    "avatarKey": "public/defaults/buyer_avatar.png"
// @Description  }
// @Description  ```
// @Description
// @Description  - `senderName`：發送者顯示名稱（買家為 nickname，賣家為 storeName），`omitempty`（未設定 profile 時不出現）
// @Description  - `avatarKey`：發送者頭像 objectKey（49 §6.7 S4：只帶 key，前端 buildImageUrl 組顯示 URL），`omitempty`
// @Tags         聊天 (Chat)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      101  {string}  string  "WebSocket 連線建立成功"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      429  {object}  object{code=int,message=string}  "同帳號並發聊天連線數超過上限"
// @Failure      502  {object}  object{code=int,message=string}
// @Router /app/chat/ws [get]
func (h *Holder) ChatWebSocket(c *gin.Context) {
	accountID, exists := c.Get("accountId")
	if !exists {
		response.Unauthorized(c)
		return
	}

	// Per-account concurrent connection cap (§11.1 WS DoS surface). The gateway
	// terminates and proxies every browser WS, so this single choke point bounds
	// both gateway and im resource use. Reject-new: a rejected client backs off
	// (Full Jitter) and the 30s read-deadline reclaims dead conns, so a legit
	// user under the generous cap is never locked out.
	id, idOK := accountID.(int64)
	if !idOK {
		response.Unauthorized(c)
		return
	}
	release, slotOK := acquireWSSlot(id)
	if !slotOK {
		response.TooManyRequests(c, "too many concurrent chat connections")
		return
	}
	defer release()

	wireID, wok := h.chatWireIdentity(c)
	if !wok {
		return
	}

	// Upgrade client connection
	clientConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer func() { _ = clientConn.Close() }()
	h.serveChatWebSocketGRPC(c.Request.Context(), clientConn, wireID, parseAccountType(c))
}

// wsProxyConn serializes writes to the public WebSocket (one-writer rule) and
// stamps a write deadline so a slow/stuck client cannot block the Gateway→IM
// stream bridge.
type wsProxyConn struct {
	Conn *websocket.Conn
	mu   sync.Mutex
}

func (c *wsProxyConn) write(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.Conn.SetWriteDeadline(time.Now().Add(wsProxyWriteWait))
	return c.Conn.WriteMessage(messageType, data)
}

// resolveChatTargetID normalizes a client-supplied chat counterpart id before
// it's used to build/query a conversation room. When the current user is a
// buyer, the target is a seller — and per resolveStorefrontSellerID
// (2026-07-23 storefront URL migration), a seller-facing id from the client
// may be either the real store_id or the public store_code storefront pages
// use; both must resolve to the same real store_id here too. Without this,
// a conversation gets keyed on a value that never matches any seller row —
// the room silently carries a dead surrogate, and every downstream read
// (profile enrichment, history lookup) comes back empty with no error ever
// surfaced. Sellers messaging a buyer are passed through unchanged — buyer_id
// has no equivalent public short-code form in this codebase.
func (h *Holder) resolveChatTargetID(c *gin.Context, targetID string) (string, bool) {
	if parseAccountType(c) == 2 {
		return targetID, true
	}
	storeID, ok := h.resolveStorefrontSellerID(c, targetID)
	if !ok {
		return "", false
	}
	return strconv.FormatInt(storeID, 10), true
}

// SendChatMessage
// @Summary      發送聊天訊息
// @Description  透過 IM 服務發送一則聊天訊息給指定使用者。type 支援 "text"（預設）和 "image"。
// @Description  type=image 時，content 須為 Media Service 的 staged token，後端會自動 FinalizeImages 轉正為 CDN URL。
// @Tags         聊天 (Chat)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{toUserId=string,type=string,content=string,extra=object}  true  "發送訊息請求（type: text|image，image 時 content 為 staged token）。買家傳訊給賣家時 toUserId 可為 store_id 或 store_code，後端統一解析成 store_id 再建立對話。"
// @Success      200  {object}  object{code=int,message=string,data=object{delivered=bool,messageId=int64,seqId=int64,timestamp=string,content=string}}
// @Failure      400  {object}  object{code=int,message=string}  "圖片處理失敗（staged token 無效）或 toUserId 格式無效"
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}  "toUserId 對應的賣家不存在"
// @Failure      502  {object}  object{code=int,message=string}
// @Router /app/chat/messages [post]
func (h *Holder) SendChatMessage(c *gin.Context) {
	wireID, ok := h.chatWireIdentity(c)
	if !ok {
		return
	}

	var req struct {
		Extra map[string]string `json:"extra"`
		Type  string            `json:"type"`
		// max=4096 aligns with the WS maxMsgSize and the messages.content CHECK
		// constraint; over-length content is rejected at the edge with a 400.
		Content  string `json:"content" binding:"required,max=4096"`
		ToUserId string `json:"toUserId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	toUserID, ok := h.resolveChatTargetID(c, req.ToUserId)
	if !ok {
		return
	}

	msgType := pb.MessageType_MESSAGE_TYPE_TEXT
	if req.Type == "image" {
		msgType = pb.MessageType_MESSAGE_TYPE_IMAGE
	}

	resp, err := h.IM.SendMessage(c.Request.Context(), &pb.SendMessageRequest{
		FromUserId:  strconv.FormatInt(wireID, 10),
		ToUserId:    toUserID,
		Type:        msgType,
		Content:     req.Content,
		Extra:       req.Extra,
		AccountType: formatAccountType(c),
	})
	if err != nil {
		response.GrpcDomainErr(c, "im", err)
		return
	}

	response.OK(c, gin.H{
		"delivered": resp.Delivered,
		"messageId": resp.MessageId,
		"seqId":     resp.SeqId,
		"timestamp": milliStrToRFC3339(resp.Timestamp),
		// 圖片訊息 content 為 objectKey，Gateway 組顯示 URL（49 §6.6 im/S4）。
		"content": chatContentDisplay(h.MediaBaseURL, resp.Content, msgType),
	})
}

// GetChatHistory
// @Summary      取得聊天歷史紀錄
// @Description  取得與指定使用者的聊天歷史訊息，支援分頁
// @Tags         聊天 (Chat)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        targetId  path   string  true   "對方使用者 ID"
// @Param        beforeId  query  string  false  "游標：取得此 ID 之前的訊息（不傳則從最新開始）"
// @Param        pageSize  query  int    false  "每頁筆數（預設 20）"
// @Success      200  {object}  object{code=int,data=object{messages=[]object{id=int64,fromUserId=int64,toUserId=int64,type=string,content=string,seqId=int64,timestamp=int64,isRead=bool},participants=[]object{userId=int64,role=string,displayName=string,avatarUrl=string},total=int64,hasMore=bool}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router /app/chat/history/{targetId} [get]
func (h *Holder) GetChatHistory(c *gin.Context) {
	wireID, ok := h.chatWireIdentity(c)
	if !ok {
		return
	}

	targetId := c.Param("targetId")
	if targetId == "" {
		response.BadRequest(c, "invalid targetId")
		return
	}

	beforeIdStr := c.Query("beforeId")
	pageSize := httputil.ParsePageParam(c.Query("pageSize"), 10)
	resp, err := h.IM.GetChatHistory(c.Request.Context(), &pb.GetChatHistoryRequest{
		UserId:      strconv.FormatInt(wireID, 10),
		TargetId:    targetId,
		BeforeId:    beforeIdStr,
		PageSize:    int32(pageSize),
		AccountType: formatAccountType(c),
	})
	if err != nil {
		response.GrpcErr(c, "im", err)
		return
	}

	// Convert proto messages to JSON-friendly format
	messages := make([]gin.H, 0, len(resp.Messages))
	for _, msg := range resp.Messages {
		messages = append(messages, gin.H{
			"id":         msg.Id,
			"fromUserId": msg.FromUserId,
			"toUserId":   msg.ToUserId,
			"type":       messageTypeToSimpleString(msg.Type),
			"content":    chatContentDisplay(h.MediaBaseURL, msg.Content, msg.Type),
			"seqId":      msg.SeqId,
			"timestamp":  milliStrToRFC3339(msg.Timestamp),
			"isRead":     msg.IsRead,
		})
	}

	participants := make([]gin.H, 0, len(resp.Participants))
	for _, p := range resp.Participants {
		participants = append(participants, gin.H{
			// userId = 該參與者的 chat wire 身份（買家 buyer_id／賣家 store_id，皆為
			// CURRENT stint 代理鍵），與同回應 messages[].fromUserId/toUserId 同一個 id
			// 空間；client 判斷「這是不是我的訊息」要拿 ws-ticket 的 userId 比對，不是
			// 登入 account_id（seller-flow-verify flow 51）。
			"userId":      p.UserId,
			"role":        p.Role,
			"displayName": p.DisplayName,
			// avatar_key = objectKey/外部 URL（proto 值）；avatarUrl = Gateway 組顯示 URL（49 §6.6 im）。
			"avatarKey": p.AvatarKey,
			"avatarUrl": mediaurl.BuildURL(h.MediaBaseURL, p.AvatarKey),
		})
	}

	response.OK(c, gin.H{
		"messages":     messages,
		"total":        resp.Total,
		"hasMore":      resp.HasMore,
		"participants": participants,
	})
}

// GetConversations
// @Summary      取得對話列表
// @Description  取得當前登入使用者的所有對話列表，支援分頁
// @Tags         聊天 (Chat)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page      query  int  false  "頁碼（預設 1）"
// @Param        pageSize  query  int  false  "每頁筆數（預設 20）"
// @Success      200  {object}  object{code=int,data=object{conversations=[]object{targetUserId=int64,targetNickname=string,avatarKey=string,avatarUrl=string,storeName=string,storeCode=string,logoKey=string,logoUrl=string,lastMessage=string,lastMessageType=string,lastMessageTime=int64,unreadCount=int32},total=int64}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router /app/chat/conversations [get]
func (h *Holder) GetConversations(c *gin.Context) {
	wireID, ok := h.chatWireIdentity(c)
	if !ok {
		return
	}

	page := httputil.ParsePageParam(c.Query("page"), 1)
	pageSize := httputil.ParsePageParam(c.Query("pageSize"), 10)

	resp, err := h.IM.GetConversations(c.Request.Context(), &pb.GetConversationsRequest{
		UserId:      strconv.FormatInt(wireID, 10),
		Page:        int32(page),
		PageSize:    int32(pageSize),
		AccountType: formatAccountType(c),
	})
	if err != nil {
		response.GrpcErr(c, "im", err)
		return
	}

	// Convert proto conversations to JSON-friendly format
	conversations := make([]gin.H, 0, len(resp.Conversations))
	for _, conv := range resp.Conversations {
		conversations = append(conversations, gin.H{
			"targetUserId":   conv.TargetUserId,
			"targetNickname": conv.TargetNickname,
			// *_key = objectKey/外部 URL（proto 值）；顯示欄 = Gateway 組 URL（49 §6.6 im）。
			"avatarKey": conv.TargetAvatarKey,
			"avatarUrl": mediaurl.BuildURL(h.MediaBaseURL, conv.TargetAvatarKey),
			"storeName": conv.StoreName,
			// 賣家 canonical 短碼；targetUserId 在賣家側是 store_id（內部代理鍵），
			// 賣家頁 canonical 網址走 storeCode。與 storeName 同源自 GetSellerBatch。
			"storeCode": conv.StoreCode,
			"logoKey":   conv.LogoKey,                                    // 商家 logo objectKey（SSOT；49 §6.1）
			"logoUrl":   mediaurl.BuildURL(h.MediaBaseURL, conv.LogoKey), // 顯示 URL（Gateway 組）
			// 圖片型最後訊息預覽 content 為 objectKey，Gateway 組顯示 URL（S4）。
			"lastMessage":     chatContentDisplay(h.MediaBaseURL, conv.LastMessage, conv.LastMessageType),
			"lastMessageType": messageTypeToSimpleString(conv.LastMessageType),
			"lastMessageTime": milliStrToRFC3339(conv.LastMessageTime),
			"unreadCount":     conv.UnreadCount,
		})
	}

	response.OK(c, gin.H{
		"conversations": conversations,
		"total":         resp.Total,
	})
}

// GetChatUnreadCount
// @Summary      取得未讀訊息數量
// @Description  取得當前登入使用者的未讀聊天訊息總數
// @Tags         聊天 (Chat)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=object{unread=int64}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router /app/chat/unread [get]
func (h *Holder) GetChatUnreadCount(c *gin.Context) {
	wireID, ok := h.chatWireIdentity(c)
	if !ok {
		return
	}

	resp, err := h.IM.GetUnreadCount(c.Request.Context(), &pb.GetUnreadCountRequest{
		UserId:      strconv.FormatInt(wireID, 10),
		AccountType: formatAccountType(c),
	})
	if err != nil {
		response.GrpcErr(c, "im", err)
		return
	}

	response.OK(c, gin.H{
		"unread": resp.UnreadCount,
	})
}

// MarkMessagesRead
// @Summary      標記訊息為已讀
// @Description  將指定的訊息 ID 列表標記為已讀狀態
// @Tags         聊天 (Chat)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{messageIds=[]string}  true  "標記已讀請求"
// @Success      200  {object}  object{code=int,message=string,data=object{updatedCount=int32}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router /app/chat/messages [patch]
func (h *Holder) MarkMessagesRead(c *gin.Context) {
	wireID, ok := h.chatWireIdentity(c)
	if !ok {
		return
	}

	var req struct {
		MessageIds []string `json:"messageIds" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	resp, err := h.IM.MarkMessagesRead(c.Request.Context(), &pb.MarkMessagesReadRequest{
		UserId:     strconv.FormatInt(wireID, 10),
		MessageIds: req.MessageIds,
	})
	if err != nil {
		response.GrpcErr(c, "im", err)
		return
	}

	response.OK(c, gin.H{
		"updatedCount": resp.UpdatedCount,
	})
}

// MarkConversationRead
// @Summary      標記對話為已讀
// @Description  將與指定使用者的對話中所有訊息標記為已讀
// @Tags         聊天 (Chat)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        targetId  path  string  true  "對方使用者 ID"
// @Success      200  {object}  object{code=int,message=string,data=object{updatedCount=int32}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router /app/chat/conversations/{targetId} [patch]
func (h *Holder) MarkConversationRead(c *gin.Context) {
	wireID, ok := h.chatWireIdentity(c)
	if !ok {
		return
	}

	targetId := c.Param("targetId")
	if targetId == "" {
		response.BadRequest(c, "invalid targetId")
		return
	}

	_, err := h.IM.MarkConversationRead(c.Request.Context(), &pb.MarkConversationReadRequest{
		UserId:      strconv.FormatInt(wireID, 10),
		TargetId:    targetId,
		AccountType: formatAccountType(c),
	})
	if err != nil {
		response.GrpcErr(c, "im", err)
		return
	}

	response.OK(c, nil)
}

// MarkAllRead
// @Summary      標記所有訊息為已讀
// @Description  將當前登入使用者的所有未讀訊息標記為已讀
// @Tags         聊天 (Chat)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,message=string,data=object{updatedCount=int32}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router /app/chat/conversations [patch]
func (h *Holder) MarkAllRead(c *gin.Context) {
	wireID, ok := h.chatWireIdentity(c)
	if !ok {
		return
	}

	_, err := h.IM.MarkAllRead(c.Request.Context(), &pb.MarkAllReadRequest{
		UserId:      strconv.FormatInt(wireID, 10),
		AccountType: formatAccountType(c),
	})
	if err != nil {
		response.GrpcErr(c, "im", err)
		return
	}

	response.OK(c, nil)
}

// parseAccountType returns the effective chat role (1=buyer, 2=seller).
// For type=1/2, uses accountType directly. For type=3, uses loginRole from JWT.
func parseAccountType(c *gin.Context) int32 {
	if v, exists := c.Get("accountType"); exists {
		if t, ok := v.(int); ok && (t == 1 || t == 2) {
			return int32(t)
		}
	}
	if v, exists := c.Get("loginRole"); exists {
		if role, ok := v.(int); ok && (role == 1 || role == 2) {
			return int32(role)
		}
	}
	return 0
}

// Helper to parse int64 from interface
func parseAccountID(v interface{}) int64 {
	switch id := v.(type) {
	case int64:
		return id
	case float64:
		return int64(id)
	case string:
		n, _ := strconv.ParseInt(id, 10, 64)
		return n
	default:
		return 0
	}
}

// milliStrToRFC3339 converts Unix milliseconds as string to ISO 8601 UTC string.
func milliStrToRFC3339(msStr string) string {
	ms, err := strconv.ParseInt(msStr, 10, 64)
	if err != nil || ms == 0 {
		return ""
	}
	return timeutil.FormatISO8601(time.UnixMilli(ms))
}

// formatAccountType returns the effective chat role as string for proto fields.
func formatAccountType(c *gin.Context) string {
	at := parseAccountType(c)
	if at == 0 {
		return ""
	}
	return strconv.FormatInt(int64(at), 10)
}

// messageTypeToSimpleString converts proto MessageType enum to client-friendly string.
func messageTypeToSimpleString(t pb.MessageType) string {
	switch t {
	case pb.MessageType_MESSAGE_TYPE_IMAGE:
		return "image"
	default:
		return "text"
	}
}

// chatContentDisplay 組出 REST 對外的 content 顯示值（49 §6.6/S4 im：組裝點＝Gateway）。
// 圖片訊息（type=2）content 為 objectKey，Gateway 以 MEDIA_BASE_URL 組顯示 URL；
// 文字訊息原樣。WS 通道則直送 objectKey（前端 buildImageUrl 組），兩通道 objectKey 為單一 SSOT。
func chatContentDisplay(base, content string, t pb.MessageType) string {
	if t == pb.MessageType_MESSAGE_TYPE_IMAGE {
		return mediaurl.BuildURL(base, content)
	}
	return content
}
