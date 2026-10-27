package handler

import (
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	notificationpb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RegisterDeviceToken
// @Summary      註冊裝置 token
// @Description  首次登入後上報 FCM/APNs device token，用於接收推播
// @Tags         通知 (Notification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{token=string,platform=string}  true  "裝置 token 與平台（ios）"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/notifications/device-token [post]
func (h *Holder) RegisterDeviceToken(c *gin.Context) {
	h.upsertDeviceToken(c)
}

// RefreshDeviceToken
// @Summary      更新裝置 token
// @Description  既有 token 失效或 app 偵測到 token rotate 時呼叫，覆寫該裝置綁定
// @Tags         通知 (Notification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{token=string,platform=string}  true  "新的裝置 token 與平台（ios）"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/notifications/device-token [put]
func (h *Holder) RefreshDeviceToken(c *gin.Context) {
	h.upsertDeviceToken(c)
}

// upsertDeviceToken 為 POST/PUT device-token 共用實作。
// notification-service 的 RegisterDeviceToken 已對同一 (user, token) 做 idempotent upsert，
// 所以註冊與刷新走同一條 gRPC，差別只在 REST 語意。
func (h *Holder) upsertDeviceToken(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	var req struct {
		Token    string `json:"token" binding:"required"`
		Platform string `json:"platform" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "token and platform are required")
		return
	}

	if req.Platform != "ios" {
		response.BadRequest(c, "platform must be ios")
		return
	}

	userType, ok := resolveInboxUserType(c)
	if !ok {
		response.BadRequest(c, "invalid login role")
		return
	}
	// 身分空間解析走 resolveNotificationUserID（notification_inbox.go）這個唯一實作，
	// 不在這裡各留一份 switch —— auth.go 的 OAuth 註冊路徑就是因為自己組了一份
	// 而漏掉買家解析，寫出永遠查不到的列。
	userID, err := h.resolveNotificationUserID(c.Request.Context(), accountID, userType)
	if err != nil {
		if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
			response.Forbidden(c, "buyer access required")
			return
		}
		if errors.Is(err, storeid.ErrNoCurrentStore) {
			response.Forbidden(c, "seller access required")
			return
		}
		response.GrpcErr(c, "buyer", err)
		return
	}

	_, err = h.Notification.RegisterDeviceToken(c.Request.Context(), &notificationpb.RegisterDeviceTokenRequest{
		UserId:   strconv.FormatInt(userID, 10),
		UserType: userType,
		Token:    req.Token,
		Platform: req.Platform,
	})
	if err != nil {
		pkglogger.Warn(c.Request.Context(), "[upsertDeviceToken] gRPC error", zap.Int64("accountId", accountID), zap.Error(err))
		response.GrpcErr(c, "notification", err)
		return
	}

	response.OK(c, nil)
}

// RemoveDeviceToken
// @Summary      移除裝置 token
// @Description  使用者登出或關閉推播時呼叫，將該裝置 token 從綁定列表移除
// @Tags         通知 (Notification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{token=string}  true  "要解除綁定的裝置 token"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /app/notifications/device-token [delete]
func (h *Holder) RemoveDeviceToken(c *gin.Context) {
	accountID, ok := getAccountID(c)
	if !ok {
		response.Unauthorized(c)
		return
	}
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "token is required")
		return
	}

	userType, ok := resolveInboxUserType(c)
	if !ok {
		response.BadRequest(c, "invalid login role")
		return
	}
	// 身分空間解析走 resolveNotificationUserID（notification_inbox.go）這個唯一實作，
	// 不在這裡各留一份 switch —— auth.go 的 OAuth 註冊路徑就是因為自己組了一份
	// 而漏掉買家解析，寫出永遠查不到的列。
	userID, err := h.resolveNotificationUserID(c.Request.Context(), accountID, userType)
	if err != nil {
		if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
			response.Forbidden(c, "buyer access required")
			return
		}
		if errors.Is(err, storeid.ErrNoCurrentStore) {
			response.Forbidden(c, "seller access required")
			return
		}
		response.GrpcErr(c, "buyer", err)
		return
	}

	_, err = h.Notification.RemoveDeviceToken(c.Request.Context(), &notificationpb.RemoveDeviceTokenRequest{
		UserId:   strconv.FormatInt(userID, 10),
		UserType: userType,
		Token:    req.Token,
	})
	if err != nil {
		pkglogger.Warn(c.Request.Context(), "[RemoveDeviceToken] gRPC error", zap.Int64("accountId", accountID), zap.Error(err))
		response.GrpcErr(c, "notification", err)
		return
	}

	response.OK(c, nil)
}
