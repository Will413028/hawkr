package usecase

import (
	"context"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

// PreferenceItem represents a single notification preference with display info.
type PreferenceItem struct {
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type categoryMetaInfo struct {
	Title       string
	Description string
}

// categoryMetaByUserType holds per-user-type display info for each category.
var categoryMetaByUserType = map[string]map[string]categoryMetaInfo{
	"seller": {
		domain.CategoryOrderUpdate:     {Title: "新訂單更新", Description: "新訂單與款項撥付通知"},
		domain.CategoryRefundUpdate:    {Title: "退貨訂單更新", Description: "退貨申請與商品寄回通知"},
		domain.CategorySystem:          {Title: "系統公告", Description: "平台規範與重要通知"},
		domain.CategoryChat:            {Title: "聊天通知", Description: "聊天訊息提醒"},
		domain.CategoryFollower:        {Title: "追蹤/收藏/瀏覽通知", Description: "潛在商機提醒"},
		domain.CategoryPost:            {Title: "顧客貼文通知", Description: "追蹤賣家發文與貼文提醒"},
		domain.CategoryShipmentAnomaly: {Title: "配送異常通知", Description: "出貨/配送異常即時提醒（強制接收，無法關閉）"},
	},
	"buyer": {
		domain.CategoryOrderUpdate:  {Title: "新訂單更新", Description: "當訂單狀態改變時發送通知"},
		domain.CategoryRefundUpdate: {Title: "退貨訂單更新", Description: "退貨申請與包裹狀態"},
		domain.CategorySystem:       {Title: "系統公告", Description: "平台規範與重要通知"},
		domain.CategoryChat:         {Title: "聊天通知", Description: "聊天訊息提醒"},
		domain.CategoryCoupon:       {Title: "優惠卷通知", Description: "優惠卷發送通知與到期通知"},
		domain.CategoryPost:         {Title: "賣家貼文通知", Description: "追蹤賣家發文與貼文提醒"},
		domain.CategoryMessage:      {Title: "賣家留言通知", Description: "留言提醒"},
		domain.CategoryRestock:      {Title: "補貨通知", Description: "追蹤的售完商品補貨時通知"},
		// shipment_anomaly 不對 buyer 開放（template target_user_type='seller'）
	},
}

// NotificationPreferenceUseCase handles notification preference operations.
type NotificationPreferenceUseCase struct {
	prefRepo domain.NotificationPreferenceRepository
}

// NewNotificationPreferenceUseCase creates a new use case.
func NewNotificationPreferenceUseCase(prefRepo domain.NotificationPreferenceRepository) *NotificationPreferenceUseCase {
	return &NotificationPreferenceUseCase{prefRepo: prefRepo}
}

// GetPreferences returns all categories with their enabled state for a user.
// Categories without a DB record default to enabled.
func (uc *NotificationPreferenceUseCase) GetPreferences(ctx context.Context, userID int64, userType string) ([]PreferenceItem, error) {
	prefs, err := uc.prefRepo.GetByUser(ctx, userID, userType)
	if err != nil {
		return nil, err
	}

	// Build lookup from DB records
	enabledMap := make(map[string]bool)
	for _, p := range prefs {
		enabledMap[p.Category] = p.Enabled
	}

	categories := domain.CategoriesForUserType(userType)
	metaMap := categoryMetaByUserType[userType]
	if metaMap == nil {
		metaMap = categoryMetaByUserType["seller"]
	}

	result := make([]PreferenceItem, 0, len(categories))
	for _, cat := range categories {
		enabled := true // default
		if v, exists := enabledMap[cat]; exists {
			enabled = v
		}
		meta := metaMap[cat]
		result = append(result, PreferenceItem{
			Category:    cat,
			Enabled:     enabled,
			Title:       meta.Title,
			Description: meta.Description,
		})
	}
	return result, nil
}

// UpdatePreference updates a single category's enabled state.
//
// 設計書 12 §3.1：賣家對 forced category（如 shipment_anomaly）強制 on，
// 嘗試關閉時回 InvalidInput（gRPC INVALID_ARGUMENT）並不寫入 DB；允許重複設成 enabled=true。
func (uc *NotificationPreferenceUseCase) UpdatePreference(ctx context.Context, userID int64, userType, category string, enabled bool) error {
	if !enabled && domain.IsForcedCategory(userType, category) {
		return pkgerrors.NewInvalidInput("此類通知為強制接收，無法關閉")
	}
	return uc.prefRepo.Upsert(ctx, userID, userType, category, enabled)
}

// CheckCategoryEnabled returns false if the user has explicitly disabled the given category.
// Defaults to true (enabled) when no preference record exists or on DB error (fail-open).
func (uc *NotificationPreferenceUseCase) CheckCategoryEnabled(ctx context.Context, userID int64, userType, category string) (bool, error) {
	pref, err := uc.prefRepo.GetByCategory(ctx, userID, userType, category)
	if err != nil {
		return true, err
	}
	if pref == nil {
		return true, nil
	}
	return pref.Enabled, nil
}
