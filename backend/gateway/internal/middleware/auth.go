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

const (
	headerAccountID   = "X-Account-Id"
	headerAccountType = "X-Account-Type"
	headerLoginRole   = "X-Login-Role"

	headerStoreID            = "X-Store-Id"
	headerSellerSuspended    = "X-Seller-Suspended"
	headerSellerCapabilities = "X-Seller-Capabilities"
)

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

type identity struct {
	AccountID   int64
	AccountType int32
	LoginRole   int32
	Business    *calleridentity.Identity
	SellerCapabilities []authz.Capability
}

func newIdentity(accountID int64, accountType, loginRole int32) *identity {
	return &identity{
		AccountID:   accountID,
		AccountType: accountType,
		LoginRole:   sanitizeActiveRole(loginRole, accountType),
	}
}

var errNoIdentity = errors.New("no identity headers")

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
			for _, k := range []string{headerAccountID, headerAccountType, headerLoginRole, "X-Buyer-Id", headerStoreID, headerSellerSuspended, headerSellerCapabilities} {
				c.Request.Header.Del(k)
			}
		}
		c.Next()
	}
}

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

func logInvalidIdentity(c *gin.Context, err error) {
	pkglogger.Warn(c.Request.Context(), "[JWTAuth] 身分 header 不合契約，當成沒有身分",
		zap.String("path", c.Request.URL.Path), zap.Error(err))
}

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

func sanitizeActiveRole(role, accountType int32) int32 {
	if RoleInSet(role, accountType) {
		return role
	}
	return 0
}

func pickActiveRole(accountType, resolved int32) int32 {
	switch accountType {
	case AccountTypeBuyer:
		return 1
	case AccountTypeSeller:
		return 2
	}
	return sanitizeActiveRole(resolved, accountType)
}
