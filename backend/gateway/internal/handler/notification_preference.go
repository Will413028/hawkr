package handler

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/gin-gonic/gin"
)

// 賣家路徑（GetNotificationPreferences／UpdateNotificationPreference）自
// issue #2707 C2 修復（本 task）起改用 h.resolveStoreID 解析**當前** store_id，
// 不再直接送 account_id —— notification_preferences 表的既有列同批原子遷移到
// store_id 鍵。**沒有對應的 db migration 檔**：notification-service 與
// seller 模組是各自獨立的 database、無法跨 DB JOIN，既有資料的搬遷走
// 人工審核 runbook，不是 migration 檔（見
// docs/runbooks/seller-notification-identity-store-id-backfill.md 與 13 號
// 設計書 §12.1.4）。買家路徑（GetBuyerNotificationPreferences／
// UpdateBuyerNotificationPreference）已經正確使用 buyerid.Resolver，不受
// 影響、本次不動。

// GetNotificationPreferences
// @Summary      取得通知偏好設定
// @Description  取得賣家的 7 個通知類別開關狀態。回傳中的 `forced=true` 表示後端強制 `enabled=true`，前端應 disable 該 toggle（賣家的 `shipment_anomaly` 永遠 forced）。
// @Tags         通知 (Notification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=object{preferences=[]object{category=string,enabled=bool,title=string,description=string,forced=bool}}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router /app/sellers/me/notification-preferences [get]
func (h *Holder) GetNotificationPreferences(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	resp, err := h.Notification.GetNotificationPreferences(c.Request.Context(), &pb.GetNotificationPreferencesRequest{
		UserId:   strconv.FormatInt(int64(storeID), 10),
		UserType: "seller",
	})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}

	prefs := make([]json.RawMessage, 0, len(resp.Preferences))
	for _, p := range resp.Preferences {
		prefs = append(prefs, response.ProtoToRaw(p))
	}
	response.OK(c, gin.H{"preferences": prefs})
}

// UpdateNotificationPreference
// @Summary      更新通知偏好設定
// @Description  更新賣家的單一通知類別開關狀態
// @Tags         通知 (Notification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{category=string,enabled=bool}  true  "更新偏好請求"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router /app/sellers/me/notification-preferences [put]
func (h *Holder) UpdateNotificationPreference(c *gin.Context) {
	storeID, ok := h.resolveStoreID(c)
	if !ok {
		return
	}

	var req struct {
		Category string `json:"category" binding:"required"`
		Enabled  bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	validCategories := map[string]bool{
		"order_update": true, "refund_update": true, "system": true,
		"chat": true, "follower": true, "post": true,
		// shipment_anomaly: 賣家 forced=true。允許進入 backend 由 usecase 層拒絕（回 400）
		// — 不在 gateway 攔以保持 forced 邏輯的單一決策點。
		"shipment_anomaly": true,
	}
	if !validCategories[req.Category] {
		response.BadRequest(c, "invalid category: "+req.Category)
		return
	}

	_, err := h.Notification.UpdateNotificationPreference(c.Request.Context(), &pb.UpdateNotificationPreferenceRequest{
		UserId:   strconv.FormatInt(int64(storeID), 10),
		UserType: "seller",
		Category: req.Category,
		Enabled:  req.Enabled,
	})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}

	response.OK(c, nil)
}

// GetBuyerNotificationPreferences
// @Summary      取得買家通知偏好設定
// @Description  取得買家的 7 個通知類別開關狀態（不含 `shipment_anomaly` — 賣家專屬）。`forced` 欄位對買家恆為 false。
// @Tags         通知 (Notification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=object{preferences=[]object{category=string,enabled=bool,title=string,description=string,forced=bool}}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router /app/buyers/me/notification-preferences [get]
func (h *Holder) GetBuyerNotificationPreferences(c *gin.Context) {
	accountID, exists := c.Get("accountId")
	if !exists {
		response.Unauthorized(c)
		return
	}

	buyerID, err := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(parseAccountID(accountID)))
	if err != nil {
		if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
			response.Forbidden(c, "buyer access required")
			return
		}
		response.GrpcErr(c, "buyer", err)
		return
	}

	resp, err := h.Notification.GetNotificationPreferences(c.Request.Context(), &pb.GetNotificationPreferencesRequest{
		UserId:   strconv.FormatInt(int64(buyerID), 10),
		UserType: "buyer",
	})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}

	prefs := make([]json.RawMessage, 0, len(resp.Preferences))
	for _, p := range resp.Preferences {
		prefs = append(prefs, response.ProtoToRaw(p))
	}
	response.OK(c, gin.H{"preferences": prefs})
}

// UpdateBuyerNotificationPreference
// @Summary      更新買家通知偏好設定
// @Description  更新買家的單一通知類別開關狀態
// @Tags         通知 (Notification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{category=string,enabled=bool}  true  "更新偏好請求"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      502  {object}  object{code=int,message=string}
// @Router /app/buyers/me/notification-preferences [put]
func (h *Holder) UpdateBuyerNotificationPreference(c *gin.Context) {
	accountID, exists := c.Get("accountId")
	if !exists {
		response.Unauthorized(c)
		return
	}

	buyerID, err := h.BuyerID.CurrentBuyerID(c.Request.Context(), identity.AccountID(parseAccountID(accountID)))
	if err != nil {
		if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
			response.Forbidden(c, "buyer access required")
			return
		}
		response.GrpcErr(c, "buyer", err)
		return
	}

	var req struct {
		Category string `json:"category" binding:"required"`
		Enabled  bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	validCategories := map[string]bool{
		"order_update": true, "refund_update": true, "system": true,
		"chat": true, "coupon": true, "post": true, "message": true,
	}
	if !validCategories[req.Category] {
		response.BadRequest(c, "invalid category: "+req.Category)
		return
	}

	_, err = h.Notification.UpdateNotificationPreference(c.Request.Context(), &pb.UpdateNotificationPreferenceRequest{
		UserId:   strconv.FormatInt(int64(buyerID), 10),
		UserType: "buyer",
		Category: req.Category,
		Enabled:  req.Enabled,
	})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}

	response.OK(c, nil)
}
