package dto

// StyleItem 是回傳給前端的單一風格
type StyleItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Label        string `json:"label"`
	Prompt       string `json:"prompt"`
	ThumbnailKey string `json:"thumbnailKey"`
	SortOrder    string `json:"sortOrder"`
}

// AdminStyleItem 是回傳給 Admin 的單一風格（含額外欄位）
type AdminStyleItem struct {
	StyleItem
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Category  string `json:"category"`
	IsDefault bool   `json:"isDefault"`
	Enabled   bool   `json:"enabled"`
}

// StyleListResponse 是 GET /ai/styles 的回應
type StyleListResponse struct {
	DefaultPrompt string      `json:"defaultPrompt"`
	WeeklyUsage   string      `json:"weeklyUsage"`
	WeeklyLimit   string      `json:"weeklyLimit"`
	Styles        []StyleItem `json:"styles"`
}

// CreateStyleRequest 是 POST /admin/ai/styles 的請求
type CreateStyleRequest struct {
	Name         string `json:"name" binding:"required"`
	Label        string `json:"label" binding:"required"`
	Prompt       string `json:"prompt" binding:"required"`
	ThumbnailKey string `json:"thumbnailKey"`
	SortOrder    string `json:"sortOrder"`
	Category     string `json:"category" binding:"required,oneof=product post"`
	IsDefault    bool   `json:"isDefault"`
	Enabled      bool   `json:"enabled"`
}

// UpdateStyleRequest 是 PUT /admin/ai/styles/:id 的請求
type UpdateStyleRequest struct {
	Label        *string `json:"label"`
	Prompt       *string `json:"prompt"`
	ThumbnailKey *string `json:"thumbnailKey"`
	SortOrder    *string `json:"sortOrder"`
	IsDefault    *bool   `json:"isDefault"`
	Enabled      *bool   `json:"enabled"`
}
