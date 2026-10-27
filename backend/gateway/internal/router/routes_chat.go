package router

import (
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

// registerChatRoutes 配置聊天/即時通訊相關路由
// /api/v1/app/chat — WS upgrade uses ticket-first auth; everything else requires JWT.
func registerChatRoutes(app *gin.RouterGroup, h *handler.Holder) {
	chat := app.Group(ChatPath)

	// WS upgrade: ticket-first auth (web), with mobile credential fallback.
	chat.GET(ChatWS, middleware.ChatWSAuth(), h.ChatWebSocket)

	// Everything else (incl. ticket minting) requires a resolved credential.
	authed := chat.Group("")
	authed.Use(middleware.JWTAuth())
	{
		// IM 送訊/ws-ticket：買賣共用，停權需「訂單關聯」細分（Gateway 看不到
		// payload）→ 注入 X-Seller-Suspended 供 im-service usecase 判斷（46 §4.4），
		// 不 deny。讀取類（history/conversations/unread/已讀）CAP_CHAT_READ 白名單
		// 語意，不掛。
		authed.POST(ChatWSTicket, injectSellerSuspension(), h.MintChatWSTicket)
		authed.POST(ChatMessages, injectSellerSuspension(), h.SendChatMessage)
		authed.GET(ChatHistory, h.GetChatHistory)
		authed.GET(ChatConversations, h.GetConversations)
		authed.GET(ChatUnread, h.GetChatUnreadCount)
		authed.PATCH(ChatReadReceipts, h.MarkMessagesRead)
		authed.PATCH(ChatConversationReceipts, h.MarkConversationRead)
		authed.PATCH(ChatAllReceipts, h.MarkAllRead)
	}
}
