package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/response"
	pkgredis "github.com/Will413028/hawkr/go-pkg/redis"
	"github.com/gin-gonic/gin"
)

// wsTicketLookupTimeout 限制一次 ticket 的 Redis 查詢：Redis 掛掉時約這麼久就降級到 HMAC ticket，不會卡在
// connection pool 的重試上，讓斷線重連的風暴堆積起來。
const wsTicketLookupTimeout = 800 * time.Millisecond

// ChatWSAuth authenticates the chat WebSocket upgrade. Browsers cannot set
// headers, so web clients present a short-lived ticket as ?ticket=. The primary
// ticket is single-use (consumed via Redis GETDEL); when Redis — the ticket
// store — is unavailable, the request degrades to verifying a stateless HMAC
// ticket so a Redis outage does not fail-close the whole web chat (IM design D5).
//
// 沒有 ticket 時與 JWTAuth 相同，信任單體帶的身分 header（App 以 Bearer 連線，單體認證後設上）。
//
// ticket 是 gateway 的 ws-ticket 端點在 JWTAuth 之後簽出的，簽發時呼叫者已經由單體認證；帳號的停權與刪除由單體
// 刪掉 session 處理，gateway 不再另查封鎖名單。
func ChatWSAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if ticket := c.Query("ticket"); ticket != "" {
			if id, ok := redeemChatWSTicket(c.Request.Context(), ticket); ok {
				applyIdentity(c, id)
				c.Next()
				return
			}
			response.Err(c, http.StatusUnauthorized, 401, "invalid or expired ws ticket")
			c.Abort()
			return
		}

		if !authenticate(c) {
			return
		}
		c.Next()
	}
}

// redeemChatWSTicket 兌換 ticket：先以 Redis GETDEL 消耗一次性的 ticket；Redis 沒有注入、連不上，或健康但沒有
// 這張 ticket 時，改驗無狀態的 HMAC ticket（Redis 掛掉期間簽出的 ticket 不會存進 Redis）。
func redeemChatWSTicket(ctx context.Context, ticket string) (*identity, bool) {
	if rdb := rdbHolder.Load(); rdb != nil {
		lctx, cancel := context.WithTimeout(ctx, wsTicketLookupTimeout)
		t, err := pkgredis.ConsumeWSTicket(lctx, rdb, ticket)
		cancel()
		if err == nil && t != nil {
			// 存下的 active role 是簽發當下的狀態，不是現在的事實：與 HMAC ticket 一樣限制在帳號的角色內。
			return newIdentity(t.AccountID, t.AccountType, t.ActiveRole), true
		}
		// err == nil && t == nil：Redis 健康但沒有這張 ticket（不存在、過期或已用過）。它仍可能是之前 Redis 掛掉
		// 時簽出的 HMAC ticket，所以往下驗；不透明的 Redis ticket 過不了簽章檢查。
		// err != nil：Redis 連不上，降級。
	}
	return verifyChatWSTicket(ticket)
}
