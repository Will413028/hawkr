package testkit

import "github.com/Will413028/hawkr/ai-service/internal/procurement/domain"

// ConfirmedResolution 是分類樹已確認品項的解析結果 fixture，分類 ID 固定為 "9001"。
func ConfirmedResolution(label string) *domain.ItemResolution {
	return &domain.ItemResolution{
		Kind: domain.ItemResolvedCategoryConfirmed, Label: label, CategoryID: "9001",
	}
}
