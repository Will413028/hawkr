package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
	"go.uber.org/zap"
)

type testCase struct {
	name    string
	message string

	wantFields []domain.SlotKey

	forbidFields []domain.SlotKey
}

var allSlotKeys = []domain.SlotKey{
	domain.SlotItem, domain.SlotPackaging, domain.SlotQuantity,
	domain.SlotBudgetPerUnit, domain.SlotUnit, domain.SlotDeliveryCity,
	domain.SlotDeliveryDate, domain.SlotExactMatchRequired,
}

func forbidAllExcept(want ...domain.SlotKey) []domain.SlotKey {
	wantSet := make(map[domain.SlotKey]bool, len(want))
	for _, k := range want {
		wantSet[k] = true
	}
	var out []domain.SlotKey
	for _, k := range allSlotKeys {
		if !wantSet[k] {
			out = append(out, k)
		}
	}
	return out
}

var testCases = []testCase{
	{
		name:    "黃金句-未正規化",
		message: "我要訂寶島梨禮盒 50 盒，單盒預算 800 元，8/20 送到桃園",
		wantFields: []domain.SlotKey{
			domain.SlotItem, domain.SlotQuantity, domain.SlotUnit,
			domain.SlotBudgetPerUnit, domain.SlotDeliveryCity,
		},
	},
	{
		name:    "已正規化-對照組",
		message: "每盒預算 800 元，2026-08-20 送到桃園市，我要訂寶島梨禮盒 50 盒",
		wantFields: []domain.SlotKey{
			domain.SlotItem, domain.SlotQuantity, domain.SlotUnit,
			domain.SlotBudgetPerUnit, domain.SlotDeliveryCity, domain.SlotDeliveryDate,
		},
	},
	{
		name:         "模糊句-不該亂補",
		message:      "幫我找公司要用的水果箱",
		wantFields:   []domain.SlotKey{domain.SlotItem},
		forbidFields: forbidAllExcept(domain.SlotItem),
	},
}

func main() {
	model := flag.String("model", "", "EXTRACTOR_MODEL 的值；留空＝沿用環境變數。合法值見 llmprovider 的登記表")
	n := flag.Int("n", 10, "每句重送次數")
	flag.Parse()

	if *n < 1 {
		fmt.Println("錯誤：--n 必須 >= 1（收到", *n, "）")
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Println("錯誤：讀取設定失敗：", err)
		return
	}
	if *model != "" {
		cfg.ExtractorModel = *model
	}

	log, _ := zap.NewDevelopment()
	extractor := usecase.NewSlotExtractor(llmprovider.NewProcurement(cfg, log))
	if _, probeErr := extractor.Extract(context.Background(), "測試", domain.Slots{}); errors.Is(probeErr, usecase.ErrProcurementLLMDisabled) {
		fmt.Printf("錯誤：EXTRACTOR_MODEL=%q 未開通（模型不在登記表、或對應的 API key 未設定），無法 benchmark\n", cfg.ExtractorModel)
		return
	}

	fmt.Printf("=== Model: %s, N=%d ===\n\n", cfg.ExtractorModel, *n)

	for _, tc := range testCases {
		successes := 0
		var totalLatency time.Duration

		for i := 0; i < *n; i++ {
			start := time.Now()
			result, err := extractor.Extract(context.Background(), tc.message, domain.Slots{})
			latency := time.Since(start)
			totalLatency += latency

			if err != nil {
				continue
			}
			if patchCoversFields(result.Patch, tc.wantFields) && patchExcludesFields(result.Patch, tc.forbidFields) {
				successes++
			}
		}

		avgLatency := totalLatency / time.Duration(*n)
		fmt.Printf("%-20s  成功率: %d/%d  平均延遲: %s\n", tc.name, successes, *n, avgLatency)
	}
}

func slotFieldPresent(p domain.SlotPatch, k domain.SlotKey) bool {
	switch k {
	case domain.SlotItem:
		return p.Item != nil
	case domain.SlotPackaging:
		return p.Packaging != nil
	case domain.SlotQuantity:
		return p.Quantity != nil
	case domain.SlotUnit:
		return p.Unit != nil
	case domain.SlotBudgetPerUnit:
		return p.BudgetPerUnit != nil
	case domain.SlotDeliveryCity:
		return p.DeliveryCity != nil
	case domain.SlotDeliveryDate:
		return p.DeliveryDate != nil
	case domain.SlotExactMatchRequired:
		return p.ExactMatchRequired != nil
	}
	return false
}

func patchCoversFields(p domain.SlotPatch, want []domain.SlotKey) bool {
	for _, k := range want {
		if !slotFieldPresent(p, k) {
			return false
		}
	}
	return true
}

func patchExcludesFields(p domain.SlotPatch, forbid []domain.SlotKey) bool {
	for _, k := range forbid {
		if slotFieldPresent(p, k) {
			return false
		}
	}
	return true
}
