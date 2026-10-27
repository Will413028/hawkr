package middleware

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

// PrivateNoStore 把一組路由的回應標記為 private, no-store（#2447 BE-01A）。
//
// **涵蓋範圍到哪為止**：從掛載的 group 往內全部涵蓋，包含 JWTAuth／capability
// gate 早退的 401／403。**不**涵蓋 engine 全域鏈在它之前就 abort 的回應——
// infrastructure/di.go 的 gin.Recovery()（panic 500）、CORS()（preflight 204）
// 與分級限流（429）都排在 group 之前。這三者的 body 都不帶使用者內容（通用錯誤
// envelope／空 preflight），所以刻意不往上搬：把這支掛到全域會連帶覆蓋 pricebook
// 與 seller_city_summary 刻意可快取的回應。
//
// 掛法有兩條硬性要求，順序寫錯不會編譯失敗、只會靜默漏掉一部分回應：
//
//  1. **必須排在 JWTAuth／capability gate 之前。** 那些中介層以
//     AbortWithStatusJSON 早退，排在它們後面的中介層根本不會執行——一個被共享
//     快取留下的 401／403 正是這張票要防的東西。
//  2. **header 必須在 c.Next() 之前寫。** unary 走 c.JSON、SSE 走第一次 Flush、
//     上游失敗走 writeUpstreamFailureEnvelope 的 WriteHeader，三條路徑都會把
//     header 送出，之後再改 header map 不會上線。
//
// gin 的 c.Header 是 Header().Set 語意（value 為空字串才是 Del），所以 handler
// 內若再寫一次 Cache-Control 會覆蓋這裡的值。AI／採購這條路徑上自己寫 SSE header
// 的兩個地方（proxy.serveAIAssistantStreamWith 與 handler.ProxyBuyerReturnPolicySSE）
// 因此引用同一個 response.CacheControlPrivateNoStore 常數。
//
// internal/sse 的留言／通知 SSE hub 是另一個子系統、有自己的 header 寫入點，仍是
// no-cache，不在 #2447 範圍內。
func PrivateNoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", response.CacheControlPrivateNoStore)
		c.Next()
	}
}
