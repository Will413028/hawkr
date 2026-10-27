package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/cache"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkgredis "github.com/Will413028/hawkr/go-pkg/redis"
	"github.com/gin-gonic/gin"
)

const (
	wsTicketTTL = 30 * time.Second
	// wsTicketStoreTimeout bounds the Redis write so a flapping/dead Redis falls
	// through to the HMAC degrade path in ~this time instead of hanging on
	// connection-pool retries (mirrors the verify-side guard; IM design D5).
	wsTicketStoreTimeout = 800 * time.Millisecond
)

// MintChatWSTicket
// @Summary      取得聊天室 WebSocket 連線票券
// @Description  以已登入身分（cookie 或 Bearer）換取一張短效的 ticket（預設單次、存於 Redis）。
// @Description  Redis 不可用時自動降級為無狀態 HMAC 簽章票（同短效、不中斷連線）。
// @Description  前端用 wss://<gateway>/api/v1/app/chat/ws?ticket=<ticket> 建立連線。
// @Tags         聊天 (Chat)
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,message=string,data=object{ticket=string,userId=string,userType=int,expiresIn=int}}
// @Failure      401  {object}  object{code=int,message=string,traceId=string}
// @Failure      500  {object}  object{code=int,message=string,traceId=string}
// @Router /app/chat/ws-ticket [post]
func (h *Holder) MintChatWSTicket(c *gin.Context) {
	aid, exists := c.Get("accountId")
	if !exists {
		response.Unauthorized(c)
		return
	}
	accountID, ok := aid.(int64)
	if !ok || accountID == 0 {
		response.Unauthorized(c)
		return
	}

	accountType := 0
	if v, ok := c.Get("accountType"); ok {
		if t, ok := v.(int); ok {
			accountType = t
		}
	}
	// parseAccountType resolves the buyer/seller view (1|2|0) from accountType
	// then loginRole — identical to what ChatWebSocket sends IM.
	activeRole := parseAccountType(c)

	// Primary: single-use Redis ticket. If Redis is unavailable — nil client, or
	// a token/write failure — degrade to a stateless HMAC ticket so a Redis outage
	// does not fail-close web chat (IM design D5). The response shape, and the
	// client contract, are identical on both paths.
	var token string
	if cache.RedisClient != nil {
		if t, err := pkgredis.GenerateSessionToken(); err == nil {
			ticket := &pkgredis.WSTicket{
				AccountID:   accountID,
				AccountType: int32(accountType),
				ActiveRole:  activeRole,
			}
			ctx, cancel := context.WithTimeout(c.Request.Context(), wsTicketStoreTimeout)
			err := pkgredis.PutWSTicket(ctx, cache.RedisClient, t, ticket, wsTicketTTL)
			cancel()
			if err == nil {
				token = t
			}
		}
	}
	if token == "" {
		// Redis nil or unhealthy -> stateless HMAC degrade (D5).
		t, err := middleware.SignChatWSTicket(accountID, int32(accountType), activeRole, wsTicketTTL)
		if err != nil {
			response.Err(c, http.StatusInternalServerError, 500, "failed to generate ws ticket")
			return
		}
		token = t
	}

	wireID := accountID
	switch activeRole {
	case 1:
		if bid, rerr := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(accountID)); rerr == nil {
			wireID = int64(bid)
		}
	case 2:
		if sid, rerr := h.Store.CurrentStoreID(c.Request.Context(), identity.AccountID(accountID)); rerr == nil {
			wireID = int64(sid)
		}
	}

	response.OK(c, gin.H{
		"ticket":    token,
		"userId":    strconv.FormatInt(wireID, 10),
		"userType":  activeRole,
		"expiresIn": int(wsTicketTTL.Seconds()),
	})
}
