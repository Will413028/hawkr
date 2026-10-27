package usecase

import "google.golang.org/adk/v2/model"

// newGemmaSlotExtractor 組裝 Gemma 的 slot extractor：走 OpenRouter 的 OpenAI 相容 Responses API 存取
// Gemma——GEMINI_API_KEY 打不到任何 Gemma model（見
// docs/superpowers/specs/2026-08-08-gemma-spike-findings.md），OpenRouter
// 是驗證過可行的替代路徑。跟 DeepSeek 共用同一套強制 tool-call 機制
// （model_llm_extractor.go），不用 Config.ResponseSchema——openaimodel 對
// 該路徑會強制 OpenAI strict-mode schema（所有欄位變 required），牴觸這裡
// 刻意允許欄位留空的設計，見
// docs/superpowers/specs/2026-08-30-procurement-slot-extractor-model-llm-design.md §1.3。
//
// client 由呼叫端傳入（internal/llmprovider 是唯一建構點），這裡只挑 Gemma
// 專屬的 config／parse。
func newGemmaSlotExtractor(llm model.LLM, modelName string) SlotExtractor {
	return &modelLLMSlotExtractor{
		llm:           llm,
		modelName:     modelName,
		applyConfig:   applyToolCallConfig,
		parseResponse: toolCallParse,
		classifyErr:   classifyOpenAICompatExtractErr,
	}
}
