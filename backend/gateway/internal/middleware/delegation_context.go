package middleware

import (
	"strconv"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/gin-gonic/gin"
)

// ctxDelegationSession is the gin context key under which the delegated operation
// guard parks the session snapshot it already resolved. Handlers read it instead of
// calling GetCurrentSession again: a second call would be a second round trip AND a
// second answer, and the two could disagree (the first one may have slid
// last_active_at, the second one is judged against a different now()).
//
// It lives in middleware rather than in router because both router (writer) and
// handler (readers) import this package, while handler must not import router.
const ctxDelegationSession = "delegationSession"

// SetDelegationSession stores the resolved session snapshot on the request and puts
// the ACTOR's identity into the outbound gRPC context. Only the guard calls it — the
// value can never come from client input.
//
// 身分注入放在這裡、不放在各個 guard：session 與身分必須同生同滅。拆成兩個呼叫點就
// 多出「有 session、沒身分」的第三種狀態，而它的症狀完全靜默（下段）。
//
// 為什麼代行請求需要顯式注入：代行群組沒有 JWT、也沒有 ADM cookie（`__Host-` 是
// host-only，工作區在 www），applyIdentity 從未執行 ⇒ x-account-id 一路是空的。而
// 56 §6.1.13 定版三把「代行上傳的圖算誰的」定為「同一位 ADM 操作者」，靠的正是
// media 既有的 `token.account_id == caller 的 x-account-id`：upload session 由 admin
// 開，轉正卻是領域服務代打，那一跳身分若是空的，比對必然不成立。⇒ 圖片被 media
// 靜默丟棄，而請求回 code: 0、version 照樣 +1、稽核記 SUCCEEDED、賣家還收到「已代您
// 執行編輯商品」的通知；products 的 images 是整份替換，於是**既有圖片一併被清空**。
// 設計書該節寫「這道防線是既有的，不需要任何新程式碼」，前提是那句「x-account-id
// 全程是同一個 admin id，並由 client interceptor 逐跳傳播」——這一行就是讓它成立的
// 起點，缺了它整條鏈上沒有任何一跳有身分可傳。
//
// 注入的是 actor 自己（admin_accounts.id ＋ ADMIN），不是 target 賣家：代行不換發
// 賣家身分（§1.3 Non-Goal）。填賣家會讓下游一切寫入的歸屬指向賣家本人，「誰做的」
// 在資料層永久無解，而那正是本功能唯一的風險控制點（§4.2）。
func SetDelegationSession(c *gin.Context, s *accountpb.OperationSessionSummary) {
	c.Set(ctxDelegationSession, s)

	adminID, err := strconv.ParseInt(s.GetActor().GetAdminId(), 10, 64)
	if err != nil || adminID <= 0 || c.Request == nil {
		return
	}
	ctx := grpcclient.WithAuthContext(c.Request.Context(), adminID, strconv.Itoa(AccountTypeAdmin))
	c.Request = c.Request.WithContext(ctx)
}

// DelegationSession returns the session snapshot resolved by the delegated
// operation guard for this request. ok is false on any route that is not behind
// that guard, which is the honest answer — those routes have no session at all.
func DelegationSession(c *gin.Context) (*accountpb.OperationSessionSummary, bool) {
	v, exists := c.Get(ctxDelegationSession)
	if !exists {
		return nil, false
	}
	s, ok := v.(*accountpb.OperationSessionSummary)
	if !ok || s == nil {
		return nil, false
	}
	return s, true
}

// DelegationReservedHeaders are the Gateway-internal delegation headers that must be
// stripped at the /api/v1/admin group entry before anything reads them (56 §6.1.0).
//
// 沒有任何 handler 現在讀它們——這正是要現在刪的理由：全 gateway 只有一處 Header.Del，
// 而 applyIdentity「一律用 Set 覆蓋」的慣例對**新增**的 header 不適用，只要日後有人加了
// 一個讀取點，外部就能偽造一個代行身分，而那一行 diff 看起來完全無害。
var DelegationReservedHeaders = []string{
	"X-Delegation-Session-Id",
	"X-Delegation-Capability",
	"X-Delegation-Audit-Id",
	"X-Acting-Store-Id",
	"X-Admin-Actor-Id",
}

// StripDelegationReservedHeaders deletes every reserved delegation header from the
// inbound request. Mounted on both /api/v1/admin groups (the requireAdmin one and
// the credential-only A31 sibling).
func StripDelegationReservedHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, h := range DelegationReservedHeaders {
			c.Request.Header.Del(h)
		}
		c.Next()
	}
}
