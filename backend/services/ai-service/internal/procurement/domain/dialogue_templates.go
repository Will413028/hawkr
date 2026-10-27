// services/ai-service/internal/procurement/domain/dialogue_templates.go
package domain

import "strings"

// TemplateRegistryVersion 是有意義的版本號——Phase 2 把 RenderFunc 的回傳型別
// 從 string 換成 []Block（真正拆分語意，不是包裝），版本號同步遞增。
const TemplateRegistryVersion = "phase2-v1"

// genericFallbackText 是 RenderPlan 找不到專屬 entry 時的安全預設。
const genericFallbackText = "已收到你的訊息。"

type SafeRenderArgs struct {
	MissingRequiredLabels []string
	SalientFactLabels     []string
	NextQuestionPrompt    string
	RejectedLabels        []string
}

type renderFunc func(args SafeRenderArgs) []Block

var templates = map[ActCode]renderFunc{
	ActAckAndAskRequired: func(a SafeRenderArgs) []Block {
		var blocks []Block
		if len(a.SalientFactLabels) > 0 {
			blocks = append(blocks, Block{Kind: BlockUnderstood, Text: "已記錄：" + strings.Join(a.SalientFactLabels, "、")})
		} else {
			blocks = append(blocks, Block{Kind: BlockUnderstood, Text: "已收到你的訊息。"})
		}
		if len(a.MissingRequiredLabels) > 0 {
			blocks = append(blocks, Block{Kind: BlockNextStep, Text: "還想請你補充：" + strings.Join(a.MissingRequiredLabels, "、")})
		}
		return blocks
	},
	ActAckAndAskOptional: func(a SafeRenderArgs) []Block {
		next := "再跟你確認最後一件事。"
		if a.NextQuestionPrompt != "" {
			next = a.NextQuestionPrompt
		}
		return []Block{
			{Kind: BlockUnderstood, Text: "好的，主要資訊都到齊了。"},
			{Kind: BlockNextStep, Text: next},
		}
	},
	ActCorrectionApplied: func(a SafeRenderArgs) []Block {
		if len(a.SalientFactLabels) > 0 {
			return []Block{{Kind: BlockUnderstood, Text: "已經幫你更新：" + strings.Join(a.SalientFactLabels, "、")}}
		}
		return []Block{{Kind: BlockUnderstood, Text: "好的，已經幫你更新了。"}}
	},
	ActInputRejected: func(a SafeRenderArgs) []Block {
		warning := "有幾個內容我沒辦法直接採用。"
		if len(a.RejectedLabels) > 0 {
			warning = "有幾個內容我沒辦法直接採用：" + strings.Join(a.RejectedLabels, "、")
		}
		return []Block{
			{Kind: BlockWarning, Text: warning},
			{Kind: BlockNextStep, Text: "麻煩再確認一下。"},
		}
	},
	ActExtractionDegraded: func(SafeRenderArgs) []Block {
		return []Block{
			{Kind: BlockWarning, Text: "需求解析暫時無法使用。"},
			{Kind: BlockContext, Text: "已保留你目前的需求內容，可以繼續補充或稍後再試。"},
		}
	},
	ActRequestBasicsReady: func(SafeRenderArgs) []Block {
		return []Block{
			{Kind: BlockUnderstood, Text: "好的，需求已經齊全了。"},
			{Kind: BlockNextStep, Text: "我現在就去幫你找找看合適的候選。"},
		}
	},
	ActPricePreviewAvailable: func(SafeRenderArgs) []Block {
		return []Block{
			{Kind: BlockContext, Text: "已經有市場參考價可以看了。"},
			{Kind: BlockNextStep, Text: "要不要先確認一下希望詢價區間？"},
		}
	},
	ActRfqReconfirmRequired: func(SafeRenderArgs) []Block {
		return []Block{
			{Kind: BlockWarning, Text: "需求內容有更新，之前確認的 RFQ 快照已經過期。"},
			{Kind: BlockNextStep, Text: "需要重新確認一次。"},
		}
	},
	ActPublicationApprovalRequired: func(SafeRenderArgs) []Block {
		return []Block{
			{Kind: BlockComplete, Text: "RFQ 快照已經確認好了。"},
			{Kind: BlockRationale, Text: "等你核准後才會正式送出給賣家。"},
		}
	},
	ActItemNeedsDisambiguation: func(a SafeRenderArgs) []Block {
		next := "找到幾個相符的品項，麻煩選一下你要的是哪一個。"
		if a.NextQuestionPrompt != "" {
			next = a.NextQuestionPrompt
		}
		return []Block{{Kind: BlockNextStep, Text: next}}
	},
}

func RenderPlan(plan DialoguePlanV1, args SafeRenderArgs) []Block {
	fn, ok := templates[plan.ActCode]
	if !ok {
		return []Block{{Kind: BlockUnderstood, Text: genericFallbackText}}
	}
	return fn(args)
}

// TextFallback 把 blocks 依序 join 成純文字，供舊消費端／wire 上的 textFallback
// 欄位使用——這是唯一的 join 邏輯，turnPayload.Reply 與 wire 的 textFallback
// 都呼叫這個函式，避免兩份文案各自維護而漂移。
func TextFallback(blocks []Block) string {
	texts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		texts = append(texts, b.Text)
	}
	return strings.Join(texts, " ")
}
