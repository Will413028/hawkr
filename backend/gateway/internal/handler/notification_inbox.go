package handler

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/gin-gonic/gin"
)

// inboxItemDTO is the API response DTO for a notification item.
type inboxItemDTO struct {
	Data      map[string]string `json:"data,omitempty"`
	Type      string            `json:"type"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	RefType   *string           `json:"refType"`
	RefId     *string           `json:"refId"`
	ReadAt    *string           `json:"readAt"`
	CreatedAt string            `json:"createdAt"`
	ID        string            `json:"id"`
	IsRead    bool              `json:"isRead"`
}

// eventTypeToDisplayType converts dot-case event_type to UPPER_SNAKE_CASE.
func eventTypeToDisplayType(eventType string) string {
	return strings.ToUpper(strings.ReplaceAll(eventType, ".", "_"))
}

// deriveRefTypeAndID extracts refType and refId from the data map.
//
// Priority:
//  1. data.refType + data.refId (explicit contract written by notification-service
//     templates via FieldSpec — see notification design doc §3.4 / §6.4).
//  2. Fallback: legacy keys (order_id / conversation_id / certification_id / post_id)
//     for templates that have not yet been migrated to the explicit contract.
func deriveRefTypeAndID(data map[string]string) (*string, *string) {
	if rt, ok := data["refType"]; ok && rt != "" {
		if rid, ok := data["refId"]; ok && rid != "" {
			return &rt, &rid
		}
	}
	if v, ok := data["order_id"]; ok && v != "" {
		t := "ORDER"
		return &t, &v
	}
	if v, ok := data["conversation_id"]; ok && v != "" {
		t := "CHAT"
		return &t, &v
	}
	if v, ok := data["certification_id"]; ok && v != "" {
		t := "MERCHANT_VERIFICATION"
		return &t, &v
	}
	if v, ok := data["post_id"]; ok && v != "" {
		t := "POST"
		return &t, &v
	}
	return nil, nil
}

func protoToInboxDTO(item *pb.NotificationItem) inboxItemDTO {
	dto := inboxItemDTO{
		ID:        item.Id,
		Type:      eventTypeToDisplayType(item.EventType),
		Title:     item.Title,
		Body:      item.Body,
		IsRead:    item.IsRead,
		CreatedAt: item.CreatedAt,
		Data:      item.Data,
	}
	if item.ReadAt != "" {
		dto.ReadAt = &item.ReadAt
	}
	if item.Data != nil {
		dto.RefType, dto.RefId = deriveRefTypeAndID(item.Data)
	}
	return dto
}

// ListNotifications
// @Summary      取得通知列表
// @Description  取得登入使用者的通知列表，支援 cursor-based 分頁，按 ID 倒序排列
// @Tags         通知 (Notification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        cursorId  query  string  false  "上一頁最後一筆的 ID，不帶表示第一頁"
// @Param        pageSize  query  int     false  "每頁筆數（預設 20）"
// @Param        category  query  string  false  "類別過濾（對應通知偏好 category），不帶 = 全部"
// @Success      200  {object}  object{code=int,data=object{records=[]object{id=int64,type=string,title=string,body=string,refType=string,refId=string,isRead=bool,readAt=string,createdAt=string,data=object},total=string,pageSize=string,hasMore=bool}}
// @Failure      401  {object}  object{code=int,message=string}
// @Router /app/notifications [get]
func (h *Holder) ListNotifications(c *gin.Context) {
	rawAccountID := c.GetHeader("X-Account-Id")
	userType, ok := resolveInboxUserType(c)
	if !ok {
		response.BadRequest(c, "invalid login role")
		return
	}
	accountID, ok := h.resolveInboxUserID(c, rawAccountID, userType)
	if !ok {
		return
	}

	cursorID := c.DefaultQuery("cursorId", "0")
	pageSize, _ := strconv.ParseInt(c.DefaultQuery("pageSize", defaultPageSizeStr), 10, 32)
	category := c.Query("category")

	resp, err := h.Notification.ListNotifications(c.Request.Context(), &pb.ListNotificationsRequest{
		UserId:   accountID,
		UserType: userType,
		CursorId: cursorID,
		PageSize: int32(pageSize),
		Category: category,
	})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, "", nil)
		return
	}

	records := make([]inboxItemDTO, 0, len(resp.Records))
	for _, item := range resp.Records {
		records = append(records, protoToInboxDTO(item))
	}

	response.OK(c, gin.H{
		"records":  records,
		"total":    resp.Total,
		"pageSize": resp.PageSize,
		"hasMore":  resp.HasMore,
	})
}

// GetUnreadCount
// @Summary      取得未讀通知數量
// @Description  取得登入使用者的未讀通知數量，用於 AppBar badge 顯示
// @Tags         通知 (Notification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=object{count=int64}}
// @Failure      401  {object}  object{code=int,message=string}
// @Router /app/notifications/unread-count [get]
func (h *Holder) GetUnreadCount(c *gin.Context) {
	rawAccountID := c.GetHeader("X-Account-Id")
	userType, ok := resolveInboxUserType(c)
	if !ok {
		response.BadRequest(c, "invalid login role")
		return
	}
	accountID, ok := h.resolveInboxUserID(c, rawAccountID, userType)
	if !ok {
		return
	}

	resp, err := h.Notification.GetUnreadCount(c.Request.Context(), &pb.GetUnreadCountRequest{
		UserId:   accountID,
		UserType: userType,
	})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, "", nil)
		return
	}

	response.OK(c, gin.H{"count": resp.Count})
}

// MarkNotificationAsRead
// @Summary      標記單筆通知為已讀
// @Description  將指定通知標記為已讀（冪等操作，重複呼叫不會更新 readAt）
// @Tags         通知 (Notification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int64  true  "通知 ID"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router /app/notifications/{id}/read [patch]
func (h *Holder) MarkNotificationAsRead(c *gin.Context) {
	rawAccountID := c.GetHeader("X-Account-Id")
	userType, ok := resolveInboxUserType(c)
	if !ok {
		response.BadRequest(c, "invalid login role")
		return
	}
	accountID, ok := h.resolveInboxUserID(c, rawAccountID, userType)
	if !ok {
		return
	}
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid id")
		return
	}

	_, grpcErr := h.Notification.MarkAsRead(c.Request.Context(), &pb.MarkAsReadRequest{
		Id:       id,
		UserId:   accountID,
		UserType: userType,
	})
	if grpcErr != nil {
		response.GrpcErr(c, "notification", grpcErr)
		return
	}

	response.OK(c, nil)
}

// MarkAllNotificationsAsRead
// @Summary      標記全部通知為已讀
// @Description  將登入使用者的所有未讀通知標記為已讀
// @Tags         通知 (Notification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,data=object{updated=int64}}
// @Failure      401  {object}  object{code=int,message=string}
// @Router /app/notifications/read-all [patch]
func (h *Holder) MarkAllNotificationsAsRead(c *gin.Context) {
	rawAccountID := c.GetHeader("X-Account-Id")
	userType, ok := resolveInboxUserType(c)
	if !ok {
		response.BadRequest(c, "invalid login role")
		return
	}
	accountID, ok := h.resolveInboxUserID(c, rawAccountID, userType)
	if !ok {
		return
	}

	resp, err := h.Notification.MarkAllAsRead(c.Request.Context(), &pb.MarkAllAsReadRequest{
		UserId:   accountID,
		UserType: userType,
	})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, "", nil)
		return
	}

	response.OK(c, gin.H{"updated": resp.Updated})
}

// DeleteNotificationsRequest 批次刪除通知請求
type DeleteNotificationsRequest struct {
	IDs []string `json:"ids" example:"7447972893359674515,7447658697518167773"`
}

// DeleteNotifications
// @Summary      批次刪除通知
// @Description  軟刪除指定的通知（只刪除屬於自己的，靜默忽略越權 ID）
// @Tags         通知 (Notification)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  DeleteNotificationsRequest  true  "要刪除的通知 ID 列表（字串陣列）"
// @Success      200  {object}  object{code=int,data=object{deleted=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Router /app/notifications [delete]
func (h *Holder) DeleteNotifications(c *gin.Context) {
	rawAccountID := c.GetHeader("X-Account-Id")
	userType, ok := resolveInboxUserType(c)
	if !ok {
		response.BadRequest(c, "invalid login role")
		return
	}
	accountID, ok := h.resolveInboxUserID(c, rawAccountID, userType)
	if !ok {
		return
	}

	var req DeleteNotificationsRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		response.BadRequest(c, "ids must not be empty")
		return
	}

	resp, err := h.Notification.DeleteNotifications(c.Request.Context(), &pb.DeleteNotificationsRequest{
		Ids:      req.IDs,
		UserId:   accountID,
		UserType: userType,
	})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, "", nil)
		return
	}

	response.OK(c, gin.H{"deleted": resp.Deleted})
}

// resolveInboxUserID 是 `notifications.user_id` 身分空間契約的**讀取端 SSOT**：
//
//	user_type='seller' → user_id 是「當前」store_id（surrogate）
//	user_type='buyer'  → user_id 是「當前」buyer_id（surrogate）
//
// 兩端現均以解析後的 surrogate 為準，因此任何要寫進／讀出 `notifications` 的路徑
// （template 的 target_user_field、device token 註冊、SSE topic）都必須以它為準，
// 而不是各自挑一個「看起來對」的 id。填錯不會有任何錯誤訊號：首任買家／賣家
// （id == account_id）測起來全綠，re-key 過的使用者永遠讀不到那些列。
//
// 字串形式供已經拿著 X-Account-Id header 的 handler 使用；需要 int64 的
// （SSE topic）走 resolveInboxUserIDInt，兩者共用同一份解析。
func (h *Holder) resolveInboxUserID(c *gin.Context, rawAccountID, userType string) (string, bool) {
	id, err := strconv.ParseInt(rawAccountID, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid account id")
		return "", false
	}
	userID, ok := h.resolveInboxUserIDInt(c, id, userType)
	if !ok {
		return "", false
	}
	return strconv.FormatInt(userID, 10), true
}

// resolveInboxUserIDInt 是 resolveInboxUserID 的 int64 形式，語意完全相同。
// 兩者共用一份實作是刻意的：SSE 訂閱端與收件匣讀取端一旦各寫一份，就會像
// 修復前那樣默默分岔（訂閱端用 account_id、發布端用 buyer_id ⇒ 買家的即時
// 通知永遠送不到，而 REST 輪詢照常有資料，症狀只有「重新整理才看得到」）。
func (h *Holder) resolveInboxUserIDInt(c *gin.Context, accountID int64, userType string) (int64, bool) {
	userID, err := h.resolveNotificationUserID(c.Request.Context(), accountID, userType)
	if err != nil {
		if errors.Is(err, buyerid.ErrNoCurrentBuyer) {
			response.Forbidden(c, "buyer access required")
			return 0, false
		}
		if errors.Is(err, storeid.ErrNoCurrentStore) {
			response.Forbidden(c, "seller access required")
			return 0, false
		}
		response.GrpcErr(c, "buyer", err)
		return 0, false
	}
	return userID, true
}

// resolveNotificationUserID 是上面那條對稱契約的**唯一實作**，也是本檔註解裡
// 「任何要寫進／讀出 notifications 的路徑都必須以它為準」那句話的落點。
//
// 它刻意**不吃 *gin.Context、不寫 HTTP 回應**，只回 (userID, error)：
// device token 的 OAuth 註冊路徑跑在 syncutil.SafeGo 的 goroutine 裡、手上只有
// context.Background()，拿不到 gin.Context —— 而那正是修復前唯一沒照這份契約走的
// 呼叫端（gateway/internal/handler/auth.go 的 Google／Apple 兩處）。
// 修復前它一律送 account_id（非解析值），對 re-key 過的買家（buyer_id <> account_id）
// 寫出一列**永遠查不到**的 device_tokens，且完全無聲。
//
// prod 2026-08-26 實測：4 個買家各有一組同 token 的重複列 —— 一列 account_id
// （舊路徑寫的）、一列 buyer_id（/app/notifications/device-token 寫的）。前者連
// 登出都清不掉，因為 RemoveDeviceToken 同樣以解析後的 buyer_id 為鍵。
//
// 本次修復（2026-08-30）同步將賣家側也改成解析 store surrogate，使買賣兩端對稱。
func (h *Holder) resolveNotificationUserID(ctx context.Context, accountID int64, userType string) (int64, error) {
	if userType == "buyer" {
		buyerID, err := h.BuyerID.CurrentBuyerID(ctx, identity.AccountID(accountID))
		if err != nil {
			return 0, err
		}
		return int64(buyerID), nil
	}
	storeID, err := h.Store.CurrentStoreID(ctx, identity.AccountID(accountID))
	if err != nil {
		return 0, err
	}
	return int64(storeID), nil
}

// resolveInboxUserType 從 loginRole 判斷 user_type（buyer/seller）。
// account_type=3 的用戶可同時擁有 buyer 和 seller 通知，
// 必須用 loginRole 區分當前角色，而非 account_type。
func resolveInboxUserType(c *gin.Context) (string, bool) {
	loginRoleRaw, _ := c.Get("loginRole")
	loginRole, _ := loginRoleRaw.(int)
	switch loginRole {
	case 1:
		return "buyer", true
	case 2:
		return "seller", true
	default:
		return "", false
	}
}
