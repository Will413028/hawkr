package dto

type QuotaResponse struct {
	StoreID     string `json:"storeId"`
	WeeklyLimit string `json:"weeklyLimit"`
	WeeklyUsage string `json:"weeklyUsage"`
	Memo        string `json:"memo"`
	UpdatedAt   string `json:"updatedAt"`
	IsCustom    bool   `json:"isCustom"`
}

type UpdateQuotaRequest struct {
	WeeklyLimit string `json:"weeklyLimit" binding:"required"`
	Memo        string `json:"memo"`
}
