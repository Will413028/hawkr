package router

import (
	"path"
	"slices"

	"github.com/Will413028/hawkr/gateway/internal/authz"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/gin-gonic/gin"
)

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

func denySuspended(c *gin.Context, reason string) {
	_ = reason
	response.BizDeny(c, bizcode.SellerSuspendedOperationBlocked, "賣家帳號停權中，無法執行此操作")
}

func sellerViewGuard(sellerCap authz.Capability) gin.HandlerFunc {
	guard := requireSeller(sellerCap)
	return func(c *gin.Context) {
		if authz.IsSellerView(c.Query("view")) {
			guard(c)
			return
		}
		c.Next()
	}
}

func sellerHandle(g *gin.RouterGroup, method, relPath string, cap authz.Capability, h gin.HandlerFunc) {
	full := ginJoinPath(g.BasePath(), relPath)
	authz.Register(method, full, cap)
	g.Handle(method, relPath, requireSeller(cap), h)
}

func sellerHandleAuthed(g *gin.RouterGroup, method, relPath string, cap authz.Capability, h gin.HandlerFunc) {
	full := ginJoinPath(g.BasePath(), relPath)
	authz.Register(method, full, cap)
	g.Handle(method, relPath, middleware.JWTAuth(), requireSeller(cap), h)
}

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
