// Package usecase 是 procurement 域的編排層。
// LLM 只在這裡出現，且職責很窄：把自然語言轉成 typed SlotPatch。
// 控制流程（下一題問什麼、要不要重算）不在 LLM 手上。
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	aidomain "github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"
)

// ExtractTimeout 是單次抽取的硬上限。逾時不是錯誤路徑而是降級路徑：
// 呼叫端拿空 patch 續行，使用者看到的是固定問法而不是錯誤畫面。
// 首次加重試（RetryTimeout）合計不得超過 gateway streamTurnDeadline（120s）的一半，
// 否則 extract 卡住時先到的是 gateway 斷線。dev 2026-09-08～09-15 分佈：p50 0.85s／p99 5.7s／最大 <10s。
const ExtractTimeout = 30 * time.Second

// MaxMessageRunes 是單則使用者訊息進入 prompt 前的長度上限。
// 使用者自由文字沒有天然上限，邊界要設在序列化進 prompt 的這一點。
const MaxMessageRunes = 2000

// shortIntentMaxLen bounds the model's Activity short-intent sentence
// (spec §2.1/§2.3) — a short phrase, not a paragraph. Enforced twice: as
// an advisory genai.Schema.MaxLength here, and authoritatively as a Go
// rune-count check in the guard (Task 3's guardShortIntent).
const shortIntentMaxLen = 60

// extractInstruction 把單位允許清單 domain.AllowedUnits 動態注入，保證 prompt
// 與 domain 規則同步。變數而非常數，由 init() 初始化。
var extractInstruction string

func init() {
	extractInstruction = fmt.Sprintf(`你是採購需求解析器。從買家的訊息中抽出明確提到的採購條件。

規則：
- 只抽訊息中明確出現的資訊；沒提到就不要放進輸出，禁止推測或補預設值
- quantity 與 budget_per_unit 是整數；budget_per_unit 是「每一單位的新台幣預算」
- unit 只能用允許清單中的值：%s；買家說「公斤/kg」用 KG、「台斤」用 CATTY、「公克/克/g」用 GRAM
- delivery_city／delivery_date 照買家原話抽出，不要自行換算、正規化或補年份
  （例如買家說「桃園」就填「桃園」、說「8/20」就填「8/20」；正規化由系統處理）
- 買家明確說「可以接受類似品項」時 exact_match_required 才填 false，否則不要填
- shortIntent 用一句話陳述你現在正在幫買家做什麼（例：「正在讀你的需求，抓品項、數量和交期」）；
  不得包含數字、單位、金額、店家名稱、庫存或資格判斷——只講意圖，不講事實或結果；
  超過長度上限或沒有把握時可以留空`,
		strings.Join(domain.AllowedUnits, "/"))
}

// ExtractResult is SlotExtractor.Extract()'s return value: the extracted
// slot patch plus the model's raw (un-guarded) Activity short-intent
// sentence. ShortIntent may be empty (model chose not to provide one,
// or the field was absent from an older/malformed response) — Task 3's
// guard decides what to do with an empty value, this type just carries
// whatever the model said.
type ExtractResult struct {
	Patch       domain.SlotPatch
	ShortIntent string
	// Attempt and MaxAttempts are set only by retryingExtractor (see
	// extractor_provider.go) — providers never set them (parseExtractResult
	// leaves them zero). 1-indexed: Attempt=1 means no retry happened,
	// Attempt=2 means the one allowed retry fired.
	Attempt     int
	MaxAttempts int
}

// SlotExtractor 把一則自然語言訊息轉成 ExtractResult（SlotPatch + Activity
// 短意圖句）。
type SlotExtractor interface {
	Extract(ctx context.Context, message string, current domain.Slots) (ExtractResult, error)
}

// newGeminiSlotExtractor 走 ADK 官方的 model.LLM Gemini 實作（內部就是包一層
// genai.NewClient，跟本檔改版前直接用的是同一顆 client），請求語意
// （ResponseSchema: buildSlotSchema()）不變，只換薄的建構外殼——見
// docs/superpowers/specs/2026-08-30-procurement-slot-extractor-model-llm-design.md §1.4。
// newGeminiSlotExtractor 組裝 gemini 的 slot extractor。client 由呼叫端傳入
// （internal/llmprovider 是唯一建構點），這裡只挑 gemini 專屬的 schema／parse。
func newGeminiSlotExtractor(llm model.LLM, modelName string) SlotExtractor {
	return &modelLLMSlotExtractor{
		llm:           llm,
		modelName:     modelName,
		applyConfig:   applySchemaConfig,
		parseResponse: schemaParse,
		classifyErr:   classifyGeminiExtractErr,
	}
}

// classifyGeminiExtractErr 把逾時包成 aidomain.ProviderUnavailableError 供上層
// retryingExtractor 判斷是否可重試。genai SDK 不像 DeepSeek／Gemma 的 OpenAI 相容
// HTTP API 一樣能直接讀 HTTP status code，僅逾時可用 errors.Is 可靠判定；其餘錯誤
// 保守視為不可重試。Gemini 目前不是 dev/prod 現役的 EXTRACTOR_MODEL（見
// k8s/values/ai-service.yaml），分類精細度暫時不必等同 DeepSeek/Gemma。
func classifyGeminiExtractErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &aidomain.ProviderUnavailableError{Err: err}
	}
	return err
}

// buildSlotSchema 定義 structured output 的 JSON schema。
// 沒有任何欄位是必填——抽不到就留空，逼模型硬填只會製造看似合理的錯誤值。
func buildSlotSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			string(domain.SlotItem):               {Type: genai.TypeString, MaxLength: genai.Ptr(int64(domain.MaxTextLen))},
			string(domain.SlotPackaging):          {Type: genai.TypeString, MaxLength: genai.Ptr(int64(domain.MaxTextLen))},
			string(domain.SlotQuantity):           {Type: genai.TypeInteger, Minimum: genai.Ptr(float64(1)), Maximum: genai.Ptr(float64(domain.MaxQuantity))},
			string(domain.SlotBudgetPerUnit):      {Type: genai.TypeInteger, Minimum: genai.Ptr(float64(1)), Maximum: genai.Ptr(float64(domain.MaxBudgetPerUnit))},
			string(domain.SlotUnit):               {Type: genai.TypeString, Enum: domain.AllowedUnits},
			string(domain.SlotDeliveryCity):       {Type: genai.TypeString},
			string(domain.SlotDeliveryDate):       {Type: genai.TypeString},
			string(domain.SlotExactMatchRequired): {Type: genai.TypeBoolean},
			"shortIntent":                         {Type: genai.TypeString, MaxLength: genai.Ptr(int64(shortIntentMaxLen))},
		},
	}
}

// parseExtractResult 把模型輸出的 JSON 轉成 ExtractResult。未知 key 由
// encoding/json 自動忽略；slot 欄位值的合法性由 domain.Slots.Apply 把關，
// shortIntent 的合法性由呼叫端的 guard 把關（Task 3）——這裡只負責形狀。
func parseExtractResult(raw []byte) (ExtractResult, error) {
	var wire struct {
		domain.SlotPatch
		ShortIntent string `json:"shortIntent"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return ExtractResult{}, fmt.Errorf("slot patch JSON 解析失敗: %w", err)
	}
	// item_resolution 不是模型能填的欄位：它是 crop_gate 查分類樹之後的判定
	// 結果，不是自然語言裡抽得出來的東西。buildSlotSchema 的 Properties 是白名單、
	// 本來就沒有它，但 wire 是嵌入整個 domain.SlotPatch 反序列化的——白名單只約束
	// 模型「應該」輸出什麼，擋不住它多吐一個 key（tool-call 模式的 Args 更是如此）。
	// 在唯一的解碼出口清成 nil，讓「模型不得決定品項分類」成為結構事實而非約定。
	wire.ItemResolution = nil
	return ExtractResult{Patch: wire.SlotPatch, ShortIntent: wire.ShortIntent}, nil
}
