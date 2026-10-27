// Package llmprovider 是這個服務**唯一**允許建構 LLM client 的地方。
//
// 存在的理由是兩個已經各咬過一次的缺陷類別：
//
//  1. **provider quirk 在拷貝之間漂移**（2026-09-11，#2549 Phase 3）：
//     `SurfaceRealizer` 手刻了第四份 client 建構、漏掉 DeepSeek 的
//     `reasoning.effort=none`，於是強制 tool_choice 一律 HTTP 400，
//     表現出來是「敘述 100% 退回模板」的靜默失敗，單元測試與 CI 全綠。
//  2. **缺金鑰讓整個服務起不來**（backend#3329 之前的舊行為）：建構子在啟動期回 error
//     ⇒ fx abort ⇒ 59 支 RPC 全滅，包含完全不需要 LLM 的那些。
//     #3230 就是這個 class 引爆的結果：206 次請求全失敗、三天無人察覺。
//
// 兩者的共同根因是同一個：**client 建構與「開通了沒」的判定被複製成 N 份，
// 而「記得參加」是 opt-in 的**。修法不是再補一次拷貝，是把接縫從「每個消費者」
// 移到「共用相依」——消費者拿到的是一個**已經決定好開通與否**的 LLM，
// 自己不建、也不判。新增消費者因此自動繼承兩種保護，不需要記得。
//
// 同一個判準先前用在 repository/txguard（把 23505 偵測收走且不匯出，讓呼叫端
// 寫不出會打爆呼叫端交易的版本）。ADK 的 gemini.NewModel／openaimodel.NewModel
// 是外部套件、擋不住直接 import，那道殘餘的縫由 scripts/check-llm-client-
// construction.py 守（static-gates），與 txguard 的收尾方式相同。
package llmprovider

import (
	"context"
	"iter"
	"sort"
	"strconv"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/openai/openai-go/v3/option"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/model/openaimodel"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/ai-service/internal/config"
)

// ---------------------------------------------------------------------------
// 模型登記表
//
// EXTRACTOR_MODEL 只能從這張表裡選。**不是自由字串**，理由是模型名與 provider
// quirk 綁在一起：DeepSeek 的 reasoning.effort=none 之所以存在，是因為那些模型
// 預設進 Thinking mode 而 Thinking mode 不支援任何形式的強制 tool_choice。
// 開成自由字串，等於讓一個 env 的值就能把「靜默 100% HTTP 400 ⇒ 敘述全部退回
// 模板 ⇒ 不觸發任何告警」這個形狀帶回來（2026-09-11 已經發生過一次）。
//
// 未列出的值走 fail-closed：降級 ＋ ai_procurement_llm_config_incomplete=1，
// **吵但不靜默**。新增模型要先對正式端點實打驗過 quirk 再進表。
// ---------------------------------------------------------------------------

// DeepSeekModelName 是目前 DeepSeek 那一支的 canonical 名字。
//
// 匯出是為了讓消費者的測試能斷言 wire 上的 model 欄位，定義仍只有一份。
//
// 🔴 2026-09-11 改自 "deepseek-v4-flash"。那個名字已經不在 DeepSeek 的
// GET /models 清單裡，但仍是一個活著的別名——實打送它會成功，而回應的 model
// 欄位回報 "deepseek-flash"。這個 provider 已經退役過模型名一次（舊值
// deepseek-chat／deepseek-reasoner 已不存在），別名哪天收掉就是上面那個形狀。
const DeepSeekModelName = "deepseek-flash"

// defaultExtractorModel 是 EXTRACTOR_MODEL 未設定時的值。維持 gemini，與
// EXTRACTOR_PROVIDER 時代的 code fallback 語意一致（dev／prod 由 k8s values
// 明確設成 deepseek-flash）。
const defaultExtractorModel = "gemini-2.5-flash"

// modelSpec 是一個已驗過的模型：它屬於哪個 provider，以及送請求時要帶的
// provider quirk。
type modelSpec struct {
	provider string // "gemini"｜"deepseek"｜"gemma"
	baseURL  string // OpenAI 相容 provider 的端點；gemini 走 genai SDK，用不到
	// options 是這個模型專屬的 RequestOption。分成 func 而不是共用切片，是為了
	// 讓「哪個模型需要哪個 quirk」逐筆可讀——共用一份的話，下一個人加模型時
	// 會直接沿用，而那正是 quirk 漂移的起點。
	options func() []option.RequestOption
}

// deepSeekOptions 是 DeepSeek 系列共用的 RequestOption。
//
// 2026-09-11 對正式 /responses 端點逐一實打，deepseek-flash 與 deepseek-v4-pro
// **行為完全相同**：不帶 reasoning.effort=none 時，tool_choice:"required" 兩者
// 皆回 HTTP 400 "Thinking mode does not support this tool_choice"；帶上之後兩者
// 皆回 200、output 是 function_call、reasoning_tokens=0。
func deepSeekOptions() []option.RequestOption {
	return []option.RequestOption{
		// openai-go 預設 MaxRetries=2（自帶 backoff）。疊在 retryingExtractor 的
		// 外層單次重試上，一則買家訊息最多會打出 6 次上游請求，全部塞在同一個
		// ExtractTimeout(30s) 內。重試層只留 retryingExtractor 一層。
		option.WithMaxRetries(0),
		// openaimodel 沒有對應的 typed 欄位，用 option.WithJSONSet 直接往 request
		// body 塞這個 top-level 欄位。
		//
		// 🔴 這是 DeepSeek 專屬：Gemma（OpenRouter）不是 reasoning model，沒有
		// thinking mode 的概念，不得比照加上。
		option.WithJSONSet("reasoning", map[string]string{"effort": "none"}),
	}
}

// gemmaOptions：Gemma 不是 reasoning model，只需要關掉 SDK 自帶重試。
func gemmaOptions() []option.RequestOption {
	return []option.RequestOption{option.WithMaxRetries(0)}
}

// knownExtractorModels 是 EXTRACTOR_MODEL 的封閉值域。
var knownExtractorModels = map[string]modelSpec{
	"gemini-2.5-flash": {provider: "gemini"},
	DeepSeekModelName:  {provider: "deepseek", baseURL: "https://api.deepseek.com", options: deepSeekOptions},
	// 2026-09-11 實打驗過與 flash 的 quirk 完全相同，可直接切換；token 用量較高。
	"deepseek-v4-pro":                {provider: "deepseek", baseURL: "https://api.deepseek.com", options: deepSeekOptions},
	"google/gemma-4-26b-a4b-it:free": {provider: "gemma", baseURL: "https://openrouter.ai/api/v1", options: gemmaOptions},
}

// ErrDisabled 是 LLM 尚未開通時呼叫端該回的業務錯誤（9703）。
//
// 採購助手與通用聊天共用同一個碼：對使用者是同一句話、同一個處置，拆成兩個碼
// 只會讓前端多一個分支。
var ErrDisabled = pkgerrors.NewBusiness(
	bizcode.FeatureAiAssistantDisabled, "AI 助理尚未開通，請稍後再試")

var procurementLLMEnabled = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "ai_procurement_llm_enabled",
	Help: "1 when the procurement LLM is wired; 0 means RFQ turns fail closed with 9703",
})

// procurementLLMConfigIncomplete 答「這是不是有人設錯了」，與 enabled 分開：
//   - 預設 provider（gemini）沒給金鑰 ＝ 尚未開通（本支 0）——沒有人「選」過它
//   - **明確選了** deepseek／gemma 卻沒給對應金鑰 ＝ 配置錯誤（本支 1）
//   - EXTRACTOR_MODEL 不在登記表裡（打錯字，或未驗過 quirk 的模型）＝ 配置錯誤（本支 1）
var procurementLLMConfigIncomplete = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "ai_procurement_llm_config_incomplete",
	Help: "1 when EXTRACTOR_MODEL is unregistered, or names a model whose provider API key is missing",
})

// procurementLLMInfo 答「這個環境現在跑的是哪一顆模型」。
//
// 🔴 存在的理由：EXTRACTOR_MODEL 成為設定之後，「設定有沒有送到 pod」沒有任何
// 外部可觀測的答案——成功路徑不寫 log，而 ai_procurement_llm_enabled 分不出
// 「用了指定的模型」與「設定沒送達、退回預設的 gemini 且剛好也有金鑰」。
// 2026-09-11 改這個設定時實際撞到：deploy 完成後無法從外部確認模型是哪一顆。
// 告警 runbook 寫著「看 EXTRACTOR_MODEL 目前是哪個模型」，這支就是那個看法。
//
// 標籤是封閉值域（登記表四筆），基數安全。
var procurementLLMInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "ai_procurement_llm_info",
	Help: "1 for the extractor model/provider actually in use; absent when the LLM is disabled",
}, []string{"model", "provider"})

// procurementLLMBudgetLimit 答「這個環境自己宣告的 LLM token 治理預算是多少」
// （tokens／分鐘）。分母是**我們自己設的數字**，不是 provider 端的配額——issue
// #3519 spec §3.5／ruling R2 已定案：provider 端沒有可查詢的配額值不影響本支，
// 需要的從來就不是那個數字。值來自 LLM_TOKEN_BUDGET_PER_MINUTE
// （`k8s/values/ai-service.yaml` 注入），搭配
// `monitoring/prometheus/rules/service-alerts.yml` 的
// AIProcurementLLMTokenBudgetHigh：
//
//	sum by (env) (rate(procurement_llm_tokens_total{kind!="cached"}[5m])) * 60
//	/ max by (env) (ai_procurement_llm_budget_limit)
//
// 🔴 未設定 LLM_TOKEN_BUDGET_PER_MINUTE 時**刻意不 export 任何序列**，而不是
// export 0：這是 GaugeVec 而非 Gauge 正是為了這件事——GaugeVec 在沒有人呼叫
// WithLabelValues 之前不會產生任何 child，Collect 出來就是零筆，Prometheus
// 端看到的是「這個 env 沒有這條 series」。
//
// 若改成 export 0，後果與直覺相反：PromQL 的浮點除法遵循 IEEE 754，正數除以
// 0 是 `+Inf`，而 `+Inf > 0.4`（以及第二條告警的 `+Inf >= 1`）都成立——分母是
// 0 會讓兩條比值型告警**立刻且永久 fire**，不是永遠不 fire。一個永遠亮著的
// 「已經超出預算」訊號，跟長期沒人管的告警是同一種缺陷（本檔慣例：長期紅著
// 等於沒有告警），而且是假的——那個環境根本沒宣告過預算，不是用量真的超標。
// 缺席的序列才是誠實的那個：分母沒有任何 series 時，整條比值運算式沒有右手
// 邊可以配對，PromQL 對整條 vector expression 給不出任何結果，兩條告警都不會
// fire——查 PromQL 會看到「這個 env 沒有這條 series」，而不是一個信心滿滿卻
// 錯誤的 0。
//
// 沒有 provider／model 標籤維度：knownExtractorModels 任一時刻只會解析出一顆
// 生效的模型（互斥單選，見上方登記表），預算是對這一整條路徑宣告的單一數字，
// 不是逐 provider 各自一份——加維度只會在沒被選中的 provider 那格製造「缺席」
// 假訊號，換不到任何真正可分割的用量。
var procurementLLMBudgetLimit = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "ai_procurement_llm_budget_limit",
	Help: "Self-declared LLM token budget (tokens per minute) for the procurement LLM path; absent (not zero) when LLM_TOKEN_BUDGET_PER_MINUTE is unset or invalid",
}, []string{})

// applyLLMBudgetLimit 從 cfg.LLMTokenBudgetPerMinute 設定
// procurementLLMBudgetLimit，未設定或不是正數時讓它維持缺席（見該 gauge 的
// 註解）。每次呼叫先 Reset，讓結果只反映這次傳入的 cfg、不殘留前一次呼叫的
// 值——測試會用不同 cfg 反覆呼叫 newProcurement。
func applyLLMBudgetLimit(cfg *config.Config, log *zap.Logger) {
	procurementLLMBudgetLimit.Reset()
	raw := cfg.LLMTokenBudgetPerMinute
	if raw == "" {
		return
	}
	budget, err := strconv.ParseFloat(raw, 64)
	if err != nil || budget <= 0 {
		log.Error("LLM_TOKEN_BUDGET_PER_MINUTE 不是正數，ai_procurement_llm_budget_limit 維持缺席（不會 export 成 0）",
			zap.String("llm_token_budget_per_minute", raw),
			zap.String("impact", "AIProcurementLLMTokenBudgetHigh／AIProcurementLLMTokenBudgetExceeded 的分母沒有資料，兩條都不會 fire"))
		return
	}
	procurementLLMBudgetLimit.WithLabelValues().Set(budget)
}

// Procurement 是採購助手（RFQ 對話）使用的 LLM，provider 由 EXTRACTOR_MODEL
// 決定。**零值不可用**——一律由 NewProcurement 建構，它永遠不回 error。
//
// 欄位刻意不匯出：呼叫端只能透過方法取用，拿不到「自己換一顆 client」的入口。
type Procurement struct {
	llm       model.LLM
	modelName string
	provider  string
	enabled   bool
}

// LLM 永遠非 nil——未開通時是 fail-closed 的停用實作，不是 nil。呼叫端因此
// 不需要 nil 守門（漏一個就是 request 上的 nil pointer panic）。
func (p Procurement) LLM() model.LLM    { return p.llm }
func (p Procurement) ModelName() string { return p.modelName }

// Provider 回 "gemini"／"deepseek"／"gemma"，未開通或設定錯誤時回空字串。
// 消費者用它挑自己那一份 schema／parse 函式——那是 use-case 專屬的資料選擇，
// 與 client 建構無關，所以留在消費者身上不算拷貝。
func (p Procurement) Provider() string { return p.provider }
func (p Procurement) Enabled() bool    { return p.enabled }

// assistantProviderEnabled 答「通用 AI 助理（Gemini）到底接上了沒」（backend#3329）。
//
// 🔴 存在的理由：#3329 把 GEMINI_API_KEY 從「啟動必要條件」改成選填之後，「沒開通」
// 從一個會 CrashLoop 的狀態變成一個**完全安靜**的狀態。把 CrashLoop 換成靜默失能
// 不是修復（06-no-small-tails.md §3.9 第 6 種外殼）——這支 gauge 是那個差別。
//
// 🔴 #3519 Task 5 fix round 1：原本掛在 internal/client/gemini（承載真正呼叫
// Gemini SDK 的舊 adapter，Chat／ProductQA／GenerateImage／GenerateRecommendations
// 等死 RPC 的唯一 caller，已刪）。那次改動一度讓這支 gauge 變成無條件 Set(0)，
// 會讓 AIAssistantProviderDisabled 告警永久紅——**但 gauge 本身仍有效**：
// NewAssistant 是 AIAssistantService（BuyerChat／SellerChat 等仍在服務的路徑）
// 唯一的 LLM 建構點，跟舊 adapter 吃同一把 GEMINI_API_KEY，所以搬到這裡、跟著
// NewAssistant 的真實開通判定走，而不是刪掉或留著歸零。
//
// 0 ＝ AIAssistantService 的聊天路徑一律回 9703。採購助手（ai_procurement_llm_*
// 三支獨立 gauge）與其餘 RPC（RFQ 詢價、AI 配額 admin、影像處理）不碰它。
var assistantProviderEnabled = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "ai_assistant_provider_enabled",
	Help: "1 when the general-chat assistant LLM is wired; 0 means chat-related RPCs fail closed with 9703",
})

// Assistant 是通用聊天 agent（internal/agent）使用的 LLM，固定 Gemini。
type Assistant struct {
	llm       model.LLM
	modelName string
	enabled   bool
}

func (a Assistant) LLM() model.LLM    { return a.llm }
func (a Assistant) ModelName() string { return a.modelName }
func (a Assistant) Enabled() bool     { return a.enabled }

// NewProcurement 決定採購助手的 LLM 開通狀態並建構 client。
//
// **永遠不回 error**：這支是 fx.Provide 的直接入口，回 error 就是 fx abort。
// 設定不完整時回停用實作並把原因寫進 log 與兩支 gauge，服務照常啟動、其餘
// RPC 不受影響。
func NewProcurement(cfg *config.Config, log *zap.Logger) Procurement {
	return newProcurement(cfg, log, "")
}

// newProcurement 的 baseURLOverride 只給測試用（見 ProcurementForTest）。
func newProcurement(cfg *config.Config, log *zap.Logger, baseURLOverride string) Procurement {
	disabled := Procurement{llm: disabledLLM{}}

	// 預算宣告與「這個環境的 LLM 到底有沒有開通」是兩件獨立的事：即使下面任一
	// 分支提早 return disabled，值班的人仍需要知道這個環境宣告的預算是多少
	// （例如尚未開通但已先把預算寫進 values，準備開通時就有值可看）。所以放在
	// 所有早退分支之前，無條件執行一次。
	applyLLMBudgetLimit(cfg, log)

	// 🔴 遷移守門（2026-09-11 EXTRACTOR_PROVIDER → EXTRACTOR_MODEL）：舊名還設著
	// 而新名沒設，代表這個環境的設定沒跟上改名。**不當成 alias 放行**——放行的話
	// 服務會退回 defaultExtractorModel，而若該環境剛好也有預設 provider 的金鑰，
	// 一切「照常運作」，只是打去了另一家。改名當天本機 .env 就是這個狀態。
	//
	// 這道守門刻意 fail-closed 而非降級到舊行為：alias 讓舊名永遠有效，等於永遠
	// 不用遷移，而這個改名的目的正是消滅「provider 與 model 兩個旋鈕」的矛盾狀態。
	if cfg.StaleExtractorProvider != "" {
		procurementLLMEnabled.Set(0)
		procurementLLMConfigIncomplete.Set(1)
		log.Error("偵測到已移除的 EXTRACTOR_PROVIDER 仍被設定，而 EXTRACTOR_MODEL 未設定，採購助手 LLM 已停用",
			zap.String("stale_extractor_provider", cfg.StaleExtractorProvider),
			zap.Strings("known_models", knownModelNames()),
			zap.String("why", "放行會靜默退回預設模型——若這個環境剛好也有預設 provider 的金鑰，"+
				"一切看起來正常，只是打去了另一家"),
			zap.String("fix", "把 EXTRACTOR_PROVIDER 換成 EXTRACTOR_MODEL，值取自上面的 known_models"))
		return disabled
	}

	name := cfg.ExtractorModel
	if name == "" {
		name = defaultExtractorModel
	}
	spec, known := knownExtractorModels[name]
	if !known {
		// 打錯字或指定了沒驗過 quirk 的模型。在收斂進登記表之前，前者會讓整個
		// 服務起不來、後者會靜默送出錯誤的 request 形狀；現在兩者都是降級＋吵。
		procurementLLMEnabled.Set(0)
		procurementLLMConfigIncomplete.Set(1)
		log.Error("EXTRACTOR_MODEL 不在已驗過的模型清單裡，採購助手 LLM 已停用",
			zap.String("extractor_model", name),
			zap.Strings("known_models", knownModelNames()),
			zap.String("why", "模型名與 provider quirk 綁定（DeepSeek 的 reasoning.effort=none），"+
				"未驗過的模型可能靜默送出被端點拒絕的 request 形狀"),
			zap.String("impact", "RFQ 相關路徑回 9703；其餘 RPC 不受影響"))
		return disabled
	}

	apiKey := providerAPIKey(cfg, spec.provider)
	if apiKey == "" {
		procurementLLMEnabled.Set(0)
		// 預設模型沒給金鑰 ＝ 尚未開通（沒有人「選」過它）；明確選了別的卻沒給
		// 對應金鑰 ＝ 配置錯誤。只有後者需要有人去修。
		if name == defaultExtractorModel {
			procurementLLMConfigIncomplete.Set(0)
			log.Warn("GEMINI_API_KEY 未設定，採購助手 LLM 尚未開通，RFQ 相關路徑將回 9703",
				zap.String("extractor_model", name),
				zap.String("impact", "ai-service 其餘 RPC 正常服務"))
		} else {
			procurementLLMConfigIncomplete.Set(1)
			log.Error("EXTRACTOR_MODEL 指定的 provider 缺 API key，採購助手 LLM 已停用",
				zap.String("extractor_model", name),
				zap.String("provider", spec.provider),
				zap.String("missing_key", providerKeyEnvName(spec.provider)),
				zap.String("impact", "RFQ 相關路徑回 9703；其餘 RPC 不受影響"),
				zap.String("fix", "補 hawkr-ai secret 的對應 key，或把 EXTRACTOR_MODEL 改回 "+defaultExtractorModel))
		}
		return disabled
	}

	llm, err := buildLLM(name, spec, apiKey, baseURLOverride)
	if err != nil {
		// 金鑰有設但 SDK 建不起來（格式錯、端點問題）。在 #3329 之前這同樣是 fx abort。
		procurementLLMEnabled.Set(0)
		procurementLLMConfigIncomplete.Set(1)
		log.Error("採購助手 LLM 建立失敗，已停用",
			zap.String("extractor_model", name),
			zap.Error(err),
			zap.String("impact", "RFQ 相關路徑回 9703；其餘 RPC 不受影響"))
		return disabled
	}
	procurementLLMEnabled.Set(1)
	procurementLLMConfigIncomplete.Set(0)
	procurementLLMInfo.Reset() // 只留當前這一組，不讓舊值在同一個 process 內殘留
	procurementLLMInfo.WithLabelValues(name, spec.provider).Set(1)
	log.Info("採購助手 LLM 已開通",
		zap.String("extractor_model", name),
		zap.String("provider", spec.provider))
	return Procurement{llm: llm, modelName: name, provider: spec.provider, enabled: true}
}

func knownModelNames() []string {
	names := make([]string, 0, len(knownExtractorModels))
	for k := range knownExtractorModels {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func providerAPIKey(cfg *config.Config, provider string) string {
	switch provider {
	case "deepseek":
		return cfg.DeepSeekAPIKey
	case "gemma":
		return cfg.OpenRouterAPIKey
	default:
		return cfg.GeminiAPIKey
	}
}

func providerKeyEnvName(provider string) string {
	switch provider {
	case "deepseek":
		return "DEEPSEEK_API_KEY"
	case "gemma":
		return "OPENROUTER_API_KEY"
	default:
		return "GEMINI_API_KEY"
	}
}

// buildLLM 是**全服務唯一**的採購側 client 建構點。
//
// baseURLOverride 只給測試覆寫用：空字串＝登記表裡的官方端點。gemini 走 genai
// SDK、沒有等價覆寫，對它無作用。
func buildLLM(name string, spec modelSpec, apiKey, baseURLOverride string) (model.LLM, error) {
	if spec.provider == "gemini" {
		return gemini.NewModel(context.Background(), name, &genai.ClientConfig{APIKey: apiKey})
	}
	baseURL := spec.baseURL
	if baseURLOverride != "" {
		baseURL = baseURLOverride
	}
	return openaimodel.NewModel(context.Background(), name, &openaimodel.ClientConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Options: spec.options(),
	})
}

// NewAssistant 決定通用聊天 agent 的 LLM 開通狀態。理由同 NewProcurement：
// 永遠不回 error。#3329 盤點時漏掉 internal/agent.NewAgents 這條路徑，於是
// 「ai-service 不設 GEMINI_API_KEY 也能啟動」在那之後仍然是假的。
//
// 🔴 三個分支都要呼叫 assistantProviderEnabled.Set(...)：這是
// AIAssistantProviderDisabled 告警唯一的資料來源，漏設一個分支＝該分支的
// 狀態對告警永遠不可見。
func NewAssistant(cfg *config.Config, log *zap.Logger) Assistant {
	if cfg.GeminiAPIKey == "" {
		assistantProviderEnabled.Set(0)
		log.Warn("GEMINI_API_KEY 未設定，通用聊天 agent 尚未開通，相關 RPC 將回 9703",
			zap.String("impact", "ai-service 其餘 RPC 正常服務"))
		return Assistant{llm: disabledLLM{}}
	}
	llm, err := gemini.NewModel(context.Background(), cfg.GeminiModel, &genai.ClientConfig{APIKey: cfg.GeminiAPIKey})
	if err != nil {
		assistantProviderEnabled.Set(0)
		log.Error("通用聊天 agent 的 Gemini model 建立失敗，已停用",
			zap.Error(err),
			zap.String("impact", "agent 相關 RPC 回 9703；其餘 RPC 不受影響"))
		return Assistant{llm: disabledLLM{}}
	}
	assistantProviderEnabled.Set(1)
	return Assistant{llm: llm, modelName: cfg.GeminiModel, enabled: true}
}

// disabledLLM 是 model.LLM 的 fail-closed 停用實作。
//
// 🔴 回停用實作而不是 nil：消費者直接呼叫 LLM 的方法，nil 介面會變成第一個
// 請求上的 nil pointer panic，而停用實作給的是乾淨的 9703。
type disabledLLM struct{}

func (disabledLLM) Name() string { return "disabled" }

func (disabledLLM) GenerateContent(context.Context, *model.LLMRequest, bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) { yield(nil, ErrDisabled) }
}
