package router

import (
	"path"
	"slices"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/gin-gonic/gin"
)

// requireSeller 依帳號角色與入口提供的能力集合（X-Seller-Capabilities，seller 模組推導）檢查能力：只看 cap 是不是
// 集合的成員，不自己從停權或生命週期推導。拒絕時，現任店家停權中回 9240，其餘（例如等待刪除）回 403。沒有能力集合
// （沒有店家、ws-ticket）一律拒絕。
func requireSeller(cap authz.Capability) gin.HandlerFunc {
	return func(c *gin.Context) {
		if middleware.SkipRoleCheck {
			c.Next()
			return
		}

		accountTypeVal, _ := c.Get("accountType")
		code, _ := accountTypeVal.(int)
		if !middleware.IsSeller(code) {
			response.Forbidden(c, "seller access required")
			c.Abort()
			return
		}

		caps, _ := c.Get("sellerCapabilities")
		if granted, _ := caps.([]authz.Capability); slices.Contains(granted, cap) {
			c.Next()
			return
		}
		if reason := c.GetString("sellerSuspended"); reason != "" {
			denySuspended(c, reason)
		} else {
			response.Forbidden(c, "seller capability not granted")
		}
		c.Abort()
	}
}

// denySuspended emits the unified suspension deny (46 §7): business code 9240,
// HTTP 409 (collapsed; front-end keys on code, routes 賣家 to the停權公告頁). reason
// is the stored reason code (may be empty on a lifted/blip race) — surfaced for obs;
// the message is the fixed user-facing text (原因碼由停權公告頁自 GetMyStore 撈).
func denySuspended(c *gin.Context, reason string) {
	_ = reason
	response.BizDeny(c, bizcode.SellerSuspendedOperationBlocked, "賣家帳號停權中，無法執行此操作")
}

// sellerViewGuard guards a buyer/seller-shared `?view=` route (46 §3.3 rule 2). When
// the request is a seller-facing view (view=seller/selling) it runs the seller
// capability check for sellerCap; any other view is a buyer-facing read and passes
// through untouched. Only the AUTHZ判斷 moves out of the handler — the HTTP `?view=`
// contract is unchanged (front-end相依), and the handler still reads ?view to pick
// its buyer/seller RPC.
func sellerViewGuard(sellerCap authz.Capability) gin.HandlerFunc {
	guard := requireSeller(sellerCap)
	return func(c *gin.Context) {
		if authz.IsSellerView(c.Query("view")) {
			guard(c) // may abort with 403 (not seller) or 9240 (suspended)
			return
		}
		c.Next() // buyer-facing view
	}
}

// injectSellerSuspension 把已驗證的停權原因傳給下游 HTTP 與 gRPC。
func injectSellerSuspension() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Header.Del(headerSellerSuspended)
		if !middleware.SkipRoleCheck {
			if code, _ := c.Get("accountType"); middleware.IsSeller(intVal(code)) {
				if reason := c.GetString("sellerSuspended"); reason != "" {
					c.Request.Header.Set(headerSellerSuspended, reason)
					c.Request = c.Request.WithContext(grpcclient.WithSellerSuspension(c.Request.Context(), reason))
				}
			}
		}
		c.Next()
	}
}

// headerSellerSuspended carries the suspension reason code to downstream services
// for the IM/social 訂單關聯細分 (46 §4.4). Absent = not suspended (or not a seller).
const headerSellerSuspended = "X-Seller-Suspended"

func intVal(v any) int {
	n, _ := v.(int)
	return n
}

// sellerHandle mounts a seller-facing route on g and registers (method, fullPath,
// cap) in the authz registry — declaration=掛載合一 (46 §3.3). g is expected to
// already carry JWTAuth via .Use(); requireSeller(cap) is prepended to the chain.
func sellerHandle(g *gin.RouterGroup, method, relPath string, cap authz.Capability, h gin.HandlerFunc) {
	full := ginJoinPath(g.BasePath(), relPath)
	authz.Register(method, full, cap)
	g.Handle(method, relPath, requireSeller(cap), h)
}

// sellerHandleAuthed is sellerHandle for a route mounted directly on a group that
// does NOT carry JWTAuth via .Use() — it prepends JWTAuth then requireSeller(cap).
func sellerHandleAuthed(g *gin.RouterGroup, method, relPath string, cap authz.Capability, h gin.HandlerFunc) {
	full := ginJoinPath(g.BasePath(), relPath)
	authz.Register(method, full, cap)
	g.Handle(method, relPath, middleware.JWTAuth(), requireSeller(cap), h)
}

// ginJoinPath replicates gin's internal joinPaths so a registered full path matches
// engine.Routes()[i].Path exactly (used by the boot assertion cross-check).
func ginJoinPath(absolutePath, relativePath string) string {
	if relativePath == "" {
		return absolutePath
	}
	finalPath := path.Join(absolutePath, relativePath)
	if relativePath[len(relativePath)-1] == '/' && finalPath[len(finalPath)-1] != '/' {
		return finalPath + "/"
	}
	return finalPath
}
