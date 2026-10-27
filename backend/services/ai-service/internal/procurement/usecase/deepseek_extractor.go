package usecase

import "google.golang.org/adk/v2/model"

// newDeepSeekSlotExtractor 走 DeepSeek 的 OpenAI 相容 Responses API，用
// 強制 tool-call 拿 schema-guaranteed 結構化輸出——DeepSeek 的
// response_format 沒有 json_schema，唯一 schema 保證的通道是 tool-call
// arguments，見
// docs/superpowers/specs/2026-08-30-procurement-slot-extractor-model-llm-design.md §0/§1。
//
// client 由呼叫端傳入（internal/llmprovider 是唯一建構點，DeepSeek 的
// reasoning.effort=none 也在那裡），這裡只挑 DeepSeek 專屬的 tool-call
// config／parse。
func newDeepSeekSlotExtractor(llm model.LLM, modelName string) SlotExtractor {
	return &modelLLMSlotExtractor{
		llm:           llm,
		modelName:     modelName,
		applyConfig:   applyToolCallConfig,
		parseResponse: toolCallParse,
		classifyErr:   classifyOpenAICompatExtractErr,
	}
}
