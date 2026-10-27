package middleware

import (
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
	pkgidentity "github.com/Will413028/hawkr/go-pkg/identity"
	"github.com/gin-gonic/gin"
)

// 反向代理路徑上的營運鍵 header（44 號的身分分離：account_id 是登入 principal，
// buyer_id／store_id 才是業務鍵）。
const (
	BuyerIDHeader = "x-buyer-id"
	StoreIDHeader = "x-store-id"
)

// InjectBuyerIDHeader／InjectStoreIDHeader 只在**反向代理**路徑上使用。
//
// 為什麼需要它們：`proxy.CreateProxy` 只注入 `x-account-id` 與 `x-account-type`；
// `x-buyer-id`／`x-store-id` 只存在於 gRPC 路徑的 metadata_interceptor，HTTP 反代
// 不經過它。不補注入的話下游只收得到 account_id，只剩兩條路——自己去 buyer／seller
// service 反解營運鍵（違反 44 號「Gateway 解析、服務端不猜」），或做不出
// `order_no × buyer_id × document_id` 這個述詞（Tier A 破功）。
//
// ⚠️ 走 gRPC 的端點**不要**掛：那條已由 metadata_interceptor 帶，重複注入會有兩個
// 真實來源。
//
// header 先刪再設：外部帶進來的同名 header 一律不可信。
func InjectBuyerIDHeader(r *buyerid.Resolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Header.Del(BuyerIDHeader)
		accountID, ok := accountIDFromContext(c)
		if !ok {
			response.AbortUnauthorized(c, "unauthorized")
			return
		}
		if r == nil {
			response.BadRequest(c, "buyer identity unavailable")
			c.Abort()
			return
		}
		buyerID, err := r.CurrentBuyerID(c.Request.Context(), pkgidentity.AccountID(accountID))
		if err != nil {
			response.BadRequest(c, "buyer identity unavailable")
			c.Abort()
			return
		}
		c.Request.Header.Set(BuyerIDHeader, strconv.FormatInt(int64(buyerID), 10))
		c.Next()
	}
}

// InjectStoreIDHeader 同上，賣家側。解析失敗回 403（不是 400）：賣家面的
// 「解不出當前 store」語意就是「你不是賣家」，與 resolveStoreID 的既有對映一致。
func InjectStoreIDHeader(r *storeid.Resolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Header.Del(StoreIDHeader)
		accountID, ok := accountIDFromContext(c)
		if !ok {
			response.AbortUnauthorized(c, "unauthorized")
			return
		}
		if r == nil {
			response.Forbidden(c, "seller access required")
			c.Abort()
			return
		}
		storeID, err := r.CurrentStoreID(c.Request.Context(), pkgidentity.AccountID(accountID))
		if err != nil {
			response.Forbidden(c, "seller access required")
			c.Abort()
			return
		}
		c.Request.Header.Set(StoreIDHeader, strconv.FormatInt(int64(storeID), 10))
		c.Next()
	}
}

// accountIDFromContext 讀 JWTAuth 放進 gin context 的 accountId（型別可能是 int64
// 或 float64，視 claim 解析路徑而定），統一收斂成 int64。
func accountIDFromContext(c *gin.Context) (int64, bool) {
	v, exists := c.Get("accountId")
	if !exists {
		return 0, false
	}
	switch id := v.(type) {
	case int64:
		return id, id != 0
	case int:
		return int64(id), id != 0
	case float64:
		return int64(id), id != 0
	case string:
		n, err := strconv.ParseInt(id, 10, 64)
		return n, err == nil && n != 0
	default:
		return 0, false
	}
}
