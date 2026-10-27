package middleware

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/calleridentity"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/response"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 單體→gateway 的身分 header（API 設計規範（Connect）§二「單體→gateway 的身分」）。
//
// 單體是對外的入口：它自己認證每個請求（web 的 session cookie、App 的 Bearer），先刪掉 client 帶來的這些
// header，只在呼叫者登入時設上。X-Active-Role 由單體解析成 X-Login-Role，不會傳到這裡。gateway 信任它們的前提
// 是只有單體連得到 gateway：gateway 不會部署（後端 CD 以遷移完成後的單體為對象），這個前提只在本機與 CI 成立。
const (
	headerAccountID   = "X-Account-Id"
	headerAccountType = "X-Account-Type"
	headerLoginRole   = "X-Login-Role"

	headerStoreID            = "X-Store-Id"
	headerSellerSuspended    = "X-Seller-Suspended"
	headerSellerCapabilities = "X-Seller-Capabilities"
)

// rdbHolder 是 Redis client：聊天 WebSocket 的一次性 ticket（ChatWSAuth）與限流共用。
// 在 Redis 初始化後由 SetRedisClient 注入；atomic.Pointer 讓讀取不需要鎖。
var rdbHolder atomic.Pointer[redis.Client]

// SetRedisClient 注入 Redis client。
func SetRedisClient(r *redis.Client) { rdbHolder.Store(r) }

// GuardRedisClient 回傳注入的 Redis client（或 nil），給限流使用，不另開連線或注入點。
func GuardRedisClient() *redis.Client { return rdbHolder.Load() }

// SkipRoleCheck 測試環境跳過角色檢查（由 SKIP_ROLE_CHECK 環境變數控制）
// 身分 header 仍然必須存在，僅跳過 requireSeller/requireAdmin 門檻
var SkipRoleCheck = os.Getenv("SKIP_ROLE_CHECK") == "true"

func init() {
	if SkipRoleCheck {
		log.Println("⚠️ Gateway 角色檢查已停用（測試模式）— 身分 header 仍然必須存在")
	}
}

// identity 是解析後的呼叫者：來自單體的身分 header，或聊天 WebSocket 的 ticket。
type identity struct {
	AccountID   int64
	AccountType int32
	LoginRole   int32
	Business    *calleridentity.Identity
	// SellerCapabilities 是單體（seller 模組）推導的現任店家能力；沒有店家（含 ws-ticket 與訪客）時為空，requireSeller 因此一律拒絕。
	SellerCapabilities []authz.Capability
}

// newIdentity 建立 identity，丟掉帳號沒有的 active role（見 sanitizeActiveRole）。每個身分進入 gateway 的地方
// （身分 header、ws-ticket）都經過這裡，新的來源不會漏掉這道檢查。
func newIdentity(accountID int64, accountType, loginRole int32) *identity {
	return &identity{
		AccountID:   accountID,
		AccountType: accountType,
		LoginRole:   sanitizeActiveRole(loginRole, accountType),
	}
}

// errNoIdentity 表示請求沒有帶身分 header：單體認定是訪客。
var errNoIdentity = errors.New("no identity headers")

// identityFromHeaders 讀單體設定的身分 header。全部沒有時回傳 errNoIdentity（訪客）；有但不合契約時回傳其他
// error，代表單體與 gateway 的契約不一致，呼叫端當成沒有身分。
//
//   - X-Account-Id：正的十進位 int64。
//   - X-Account-Type：舊契約的角色 bitmask 0–3，或管理員 4。
//   - X-Login-Role：可以沒有（沒有角色的帳號）；有時必須是整數，再由 newIdentity 限制在帳號的角色內。管理員的 4
//     是合法值，但管理員沒有買家或賣家的視角，往下游傳的 active role 與過去一樣是 0。
//   - X-Buyer-Id、X-Store-Id：正 int64，須符合帳號角色；管理員不帶業務身分。
//   - X-Seller-Suspended：現任店家的停權原因：管理員停權是 9601、9602、9603，治理停權是 SUSPENDED_GOVERNANCE；缺省表示未停權。
//   - X-Seller-Capabilities：現任店家目前的能力，以逗號分隔的 authz 能力 token（seller 模組是唯一推導的地方）。有 X-Store-Id
//     就必須有、沒有 X-Store-Id 就不能有；空值與不認得的 token 都是契約錯誤，不補預設值（fail-closed）。
func identityFromHeaders(h http.Header) (*identity, error) {
	rawID, rawType, rawRole := h.Get(headerAccountID), h.Get(headerAccountType), h.Get(headerLoginRole)
	if rawID == "" && rawType == "" && rawRole == "" && h.Get("X-Buyer-Id") == "" && h.Get(headerStoreID) == "" && h.Get(headerSellerSuspended) == "" && h.Get(headerSellerCapabilities) == "" {
		return nil, errNoIdentity
	}
	accountID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || accountID <= 0 {
		return nil, fmt.Errorf("%s %q 不是正整數", headerAccountID, rawID)
	}
	accountType, err := strconv.ParseInt(rawType, 10, 32)
	if err != nil || !validAccountType(int32(accountType)) {
		return nil, fmt.Errorf("%s %q 不是合法的角色 bitmask", headerAccountType, rawType)
	}
	var loginRole int64
	if rawRole != "" {
		if loginRole, err = strconv.ParseInt(rawRole, 10, 32); err != nil {
			return nil, fmt.Errorf("%s %q 不是整數", headerLoginRole, rawRole)
		}
	}
	id := newIdentity(accountID, int32(accountType), int32(loginRole))
	business := calleridentity.Identity{AccountID: accountID, IsAdmin: id.AccountType == AccountTypeAdmin}
	for header, target := range map[string]*int64{"X-Buyer-Id": &business.BuyerID, headerStoreID: &business.StoreID} {
		if raw := h.Get(header); raw != "" {
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || n <= 0 {
				return nil, fmt.Errorf("%s 不是正整數", header)
			}
			*target = n
		}
	}
	business.SellerSuspended = h.Get(headerSellerSuspended)
	switch business.SellerSuspended {
	case "", "9601", "9602", "9603", "SUSPENDED_GOVERNANCE":
	default:
		return nil, errors.New("X-Seller-Suspended 不是合法原因碼")
	}
	caps, err := parseSellerCapabilities(h.Get(headerSellerCapabilities))
	if err != nil {
		return nil, err
	}
	if (business.StoreID != 0) != (len(caps) > 0) {
		return nil, errors.New("X-Store-Id 與 X-Seller-Capabilities 必須同時存在")
	}
	id.SellerCapabilities = caps
	if (business.BuyerID != 0 && !RoleInSet(1, id.AccountType)) ||
		(business.StoreID != 0 && !RoleInSet(2, id.AccountType)) ||
		(business.SellerSuspended != "" && business.StoreID == 0) {
		return nil, errors.New("業務身分不符合帳號角色")
	}
	id.Business = &business
	return id, nil
}

// parseSellerCapabilities 解析 X-Seller-Capabilities：空值回空集合；有空的或不認得的 token 回錯誤。
func parseSellerCapabilities(raw string) ([]authz.Capability, error) {
	if raw == "" {
		return nil, nil
	}
	tokens := strings.Split(raw, ",")
	out := make([]authz.Capability, 0, len(tokens))
	for _, t := range tokens {
		c := authz.Capability(t)
		if !authz.IsKnown(c) {
			return nil, fmt.Errorf("%s 含有不認得的能力 %q", headerSellerCapabilities, t)
		}
		out = append(out, c)
	}
	return out, nil
}

// validAccountType 是 X-Account-Type 的值域：沒有角色 0、買家 1、賣家 2、兩者 3、管理員 4。
func validAccountType(t int32) bool {
	return (t >= 0 && t <= AccountTypeSellerAndBuyer) || t == AccountTypeAdmin
}

// JWTAuth 要求登入：信任單體帶的身分 header（見上方常數的說明），重建 gin ctx、往下游的 header 與 gRPC metadata。
// 沒有或不合法時回 401。名稱沿用自 gateway 自己驗 JWT 的時期，讓既有的路由註冊不必改。
//
// 單體無法解析 session 時自己回 503，不會轉到這裡，所以這裡沒有 503。
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !authenticate(c) {
			return
		}
		c.Next()
	}
}

// OptionalJWTAuth 有身分 header 時套用，沒有時當成訪客；一律不中斷請求。
func OptionalJWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := identityFromHeaders(c.Request.Header)
		switch {
		case err == nil:
			applyIdentity(c, id)
		case !errors.Is(err, errNoIdentity):
			logInvalidIdentity(c, err)
			// 不合契約的身分 header 不往下游傳。
			for _, k := range []string{headerAccountID, headerAccountType, headerLoginRole, "X-Buyer-Id", headerStoreID, headerSellerSuspended, headerSellerCapabilities} {
				c.Request.Header.Del(k)
			}
		}
		c.Next()
	}
}

// authenticate 是 JWTAuth 與 ChatWSAuth（沒有 ticket 時）共用的認證：成功時套用身分並回傳 true，否則回 401、
// 中斷請求並回傳 false。
func authenticate(c *gin.Context) bool {
	id, err := identityFromHeaders(c.Request.Header)
	if err != nil {
		if !errors.Is(err, errNoIdentity) {
			logInvalidIdentity(c, err)
		}
		response.Err(c, http.StatusUnauthorized, 401, "missing or invalid credentials")
		c.Abort()
		return false
	}
	applyIdentity(c, id)
	return true
}

// logInvalidIdentity 記錄不合契約的身分 header：只有單體會設它們，所以這是單體與 gateway 的契約不一致，不是
// client 的錯。
func logInvalidIdentity(c *gin.Context, err error) {
	pkglogger.Warn(c.Request.Context(), "[JWTAuth] 身分 header 不合契約，當成沒有身分",
		zap.String("path", c.Request.URL.Path), zap.Error(err))
}

// applyIdentity 把解析後的身分寫進請求 header（反向代理的下游服務讀它們）、gin ctx（gateway 的 handler）與 Go
// context（gRPC metadata）。header 一律以 Set 覆蓋成正規化後的值。
func applyIdentity(c *gin.Context, id *identity) {
	accountTypeStr := strconv.Itoa(int(id.AccountType))
	activeRole := pickActiveRole(id.AccountType, id.LoginRole)
	c.Request.Header.Set(headerAccountID, formatInt64(id.AccountID))
	c.Request.Header.Set(headerAccountType, accountTypeStr)
	c.Request.Header.Set(headerLoginRole, strconv.Itoa(int(activeRole)))
	c.Request.Header.Del(headerSellerCapabilities)

	c.Set("accountId", id.AccountID)
	c.Set("accountType", int(id.AccountType))
	c.Set("loginRole", int(activeRole))

	ctx := grpcclient.WithAuthContext(c.Request.Context(), id.AccountID, accountTypeStr)
	if id.Business != nil {
		c.Set("identityFromMonolith", true)
		c.Set("buyerId", id.Business.BuyerID)
		c.Set("storeId", id.Business.StoreID)
		c.Set("sellerSuspended", id.Business.SellerSuspended)
		c.Set("sellerCapabilities", id.SellerCapabilities)
		ctx = calleridentity.With(ctx, *id.Business)
	}
	c.Request = c.Request.WithContext(ctx)
}

func formatInt64(n int64) string {
	if n == 0 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}

// RoleInSet reports whether the canonical role (1=buyer, 2=seller) is present
// in the accountType bitmask. This is the validation that keeps an active role
// from ever granting a capability the account does not hold.
func RoleInSet(role, accountType int32) bool {
	switch role {
	case 1:
		return accountType&1 != 0
	case 2:
		return accountType&2 != 0
	}
	return false
}

// sanitizeActiveRole drops an active role the account does not hold, collapsing
// it to 0 — "no active role", the state an admin and an account without roles
// already carry, which every downstream consumer of X-Login-Role handles.
// Anything outside {1,2} (0, 3=the accountType bitmask for "both", 4=admin, 7,
// negatives) is not a buyer/seller view and lands there too.
//
// 單體送來的 X-Login-Role 已經限制在帳號的角色內；ws-ticket 的 active role 則是簽發當下的狀態，可能早於角色的
// 移除。兩者都經 newIdentity 走這一道，gateway 不會把帳號沒有的角色往下游傳。
func sanitizeActiveRole(role, accountType int32) int32 {
	if RoleInSet(role, accountType) {
		return role
	}
	return 0
}

// pickActiveRole resolves the per-request "active role" — the workspace view a
// user is currently in. It is a view/routing selector (consumed downstream as
// X-Login-Role by notification inbox, chat, IM), NOT an authorization input.
//
//  1. for a single-role account, the unambiguous role;
//  2. otherwise the resolved role (單體解析 X-Active-Role 後的 X-Login-Role，或 ws-ticket 的 active role), held
//     to the account's role set (sanitizeActiveRole). 0 means "no active role" — downstream consumers keep their
//     existing handling for that case.
func pickActiveRole(accountType, resolved int32) int32 {
	switch accountType {
	case AccountTypeBuyer:
		return 1
	case AccountTypeSeller:
		return 2
	}
	return sanitizeActiveRole(resolved, accountType)
}
