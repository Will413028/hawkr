// Package extract 把買家的一則自然語言訊息轉成 slots.Patch：一次模型呼叫，職責只有「抽出訊息裡明確提到的欄位」，
// 控制流程（下一題問什麼）不在模型手上。呼叫內最多重試一次（A9-D28(2)、ADR E7）：platform/llm 關掉了 SDK 的重試，重試只在這一層。
package extract

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"time"

	adkmodel "google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
)

// 時間預算（A9-D28(2)）：單次 30 秒、含重試與退避總共 70 秒，低於 run 的 2 分鐘 job 上限。
const (
	AttemptTimeout = 30 * time.Second
	TotalBudget    = 70 * time.Second
	// BackoffBase 是重試前退避的基準，實際等待是 BackoffBase 加上最多 BackoffBase 的隨機抖動。
	BackoffBase = 500 * time.Millisecond
	// MaxMessageRunes 是買家訊息進入 prompt 前的長度上限；自由文字沒有天然上限，邊界設在序列化進 prompt 的這一點。
	MaxMessageRunes = 2000
)

// Shape 是結構化輸出的取得方式：兩個 provider 不能共用（DeepSeek 的 response schema 路徑會被強制成 strict mode，所有欄位變必填）。
type Shape int

const (
	// ShapeSchema 走原生 response schema（Gemini）。
	ShapeSchema Shape = iota
	// ShapeToolCall 強制模型呼叫一個 tool，用它的參數當結果（DeepSeek）。
	ShapeToolCall
)

// ShapeFor 依 provider 選結構化輸出的取得方式；不認得的 provider 回錯誤。
func ShapeFor(p llm.Provider) (Shape, error) {
	switch p {
	case llm.Gemini:
		return ShapeSchema, nil
	case llm.DeepSeek:
		return ShapeToolCall, nil
	}
	return 0, fmt.Errorf("extract：不認得的 provider %q", p)
}

const toolName = "extract_slot_patch"

// Error 是抽取失敗。Unavailable 為真代表模型服務沒有給出可用答案（逾時、429、5xx，重試用盡）；
// 其餘（4xx、停用、安全過濾等帶 ErrorCode 的拒答、結束原因不是 STOP、重試後仍格式錯誤或 OTHER／MALFORMED_FUNCTION_CALL）為假。
// 回答只在結束原因為 STOP 時成立：有內容但被截斷或過濾（MAX_TOKENS、SAFETY 等）的回應不採用。
// 重試規則：逾時、429、5xx、連線層錯誤、格式錯誤與 ErrorCode 或結束原因為 OTHER、MALFORMED_FUNCTION_CALL、UNSPECIFIED、空值（輸出不明或壞掉）重試一次；
// 4xx、停用與 nonRetryableCodes 內的 ErrorCode 或結束原因（安全過濾、截斷等，同樣輸入重試只會得到同樣結果）不重試。
type Error struct {
	Unavailable bool
	Err         error
}

func (e *Error) Error() string { return fmt.Sprintf("extract：%v", e.Err) }
func (e *Error) Unwrap() error { return e.Err }

// Extractor 包住一個 model.LLM。
type Extractor struct {
	llm       adkmodel.LLM
	modelName string
	shape     Shape

	attemptTimeout, totalBudget, backoff time.Duration
	sleep                                func(ctx context.Context, d time.Duration) error
	jitter                               func() float64
}

// New 建立 Extractor。
func New(l adkmodel.LLM, modelName string, shape Shape) *Extractor {
	return &Extractor{
		llm: l, modelName: modelName, shape: shape,
		attemptTimeout: AttemptTimeout, totalBudget: TotalBudget, backoff: BackoffBase,
		sleep: sleepCtx, jitter: rand.Float64,
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Extract 抽出 message 裡明確提到的欄位。ctx 被取消時立刻回 ctx 的錯誤（不是 *Error）。
func (e *Extractor) Extract(ctx context.Context, message string, known slots.Set) (slots.Patch, error) {
	if r := []rune(message); len(r) > MaxMessageRunes {
		message = string(r[:MaxMessageRunes])
	}
	req, err := e.request(message, known)
	if err != nil {
		return slots.Patch{}, &Error{Err: err}
	}
	budget, cancel := context.WithTimeout(ctx, e.totalBudget)
	defer cancel()

	patch, ae := e.attempt(ctx, budget, req)
	if ae == nil {
		return patch, nil
	}
	if ctx.Err() != nil {
		return slots.Patch{}, ctx.Err()
	}
	if !ae.retryable || budget.Err() != nil {
		return slots.Patch{}, &Error{Unavailable: ae.unavailable, Err: ae.err}
	}
	wait := time.Duration(float64(e.backoff) * (1 + e.jitter()))
	if serr := e.sleep(budget, wait); serr != nil {
		if ctx.Err() != nil {
			return slots.Patch{}, ctx.Err()
		}
		return slots.Patch{}, &Error{Unavailable: ae.unavailable, Err: ae.err}
	}
	patch, ae = e.attempt(ctx, budget, req)
	if ae == nil {
		return patch, nil
	}
	if ctx.Err() != nil {
		return slots.Patch{}, ctx.Err()
	}
	return slots.Patch{}, &Error{Unavailable: ae.unavailable, Err: ae.err}
}

var nonRetryableCodes = map[string]bool{
	"SAFETY": true, "PROHIBITED_CONTENT": true, "RECITATION": true, "BLOCKLIST": true, "SPII": true,
	"MAX_TOKENS": true, "content_filter": true,
	"LANGUAGE": true, "IMAGE_SAFETY": true, "UNEXPECTED_TOOL_CALL": true, "TOO_MANY_TOOL_CALLS": true,
}

type attemptErr struct {
	err                    error
	retryable, unavailable bool
}

func (e *Extractor) attempt(parent, budget context.Context, req *adkmodel.LLMRequest) (slots.Patch, *attemptErr) {
	ctx, cancel := context.WithTimeout(budget, e.attemptTimeout)
	defer cancel()
	var resp *adkmodel.LLMResponse
	var callErr error
	for r, err := range e.llm.GenerateContent(ctx, req, false) {
		resp, callErr = r, err
	}
	if callErr != nil {
		return slots.Patch{}, classify(parent, ctx, callErr)
	}
	if resp != nil && resp.ErrorCode != "" {
		return slots.Patch{}, &attemptErr{
			err:       fmt.Errorf("模型回應帶錯誤 %s：%s", resp.ErrorCode, resp.ErrorMessage),
			retryable: !nonRetryableCodes[resp.ErrorCode],
		}
	}
	if resp != nil && resp.FinishReason != genai.FinishReasonStop {
		err := fmt.Errorf("模型結束原因 %q", resp.FinishReason)
		if detail := llm.FinishDetail(resp); detail != "" {
			err = fmt.Errorf("模型結束原因 %q：%s", resp.FinishReason, detail)
		}
		return slots.Patch{}, &attemptErr{
			err:       err,
			retryable: !nonRetryableCodes[string(resp.FinishReason)],
		}
	}
	patch, err := e.parse(resp)
	if err != nil {
		return slots.Patch{}, &attemptErr{err: err, retryable: true}
	}
	return patch, nil
}

func classify(parent, attempt context.Context, err error) *attemptErr {
	if errors.Is(err, llm.ErrDisabled) {
		return &attemptErr{err: err}
	}
	if parent.Err() == nil && (errors.Is(err, context.DeadlineExceeded) || attempt.Err() != nil && errors.Is(attempt.Err(), context.DeadlineExceeded)) {
		return &attemptErr{err: err, retryable: true, unavailable: true}
	}
	if code, ok := llm.HTTPStatus(err); ok {
		if code == http.StatusTooManyRequests || code >= 500 {
			return &attemptErr{err: err, retryable: true, unavailable: true}
		}
		return &attemptErr{err: err}
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return &attemptErr{err: err, retryable: true, unavailable: true}
	}
	return &attemptErr{err: err}
}

var instruction = fmt.Sprintf(`你是採購需求解析器。從買家的訊息中抽出明確提到的採購條件。

規則：
- 只抽訊息中明確出現的資訊；沒提到就不要放進輸出，禁止推測或補預設值
- quantity 與 budget_per_unit 是整數；budget_per_unit 是「每一單位的新台幣預算」
- unit 只能用允許清單中的值：%s；買家說「公斤/kg」用 KG、「台斤」用 CATTY、「公克/克/g」用 GRAM
- delivery_city／delivery_date 照買家原話抽出，不要自行換算、正規化或補年份
  （例如買家說「桃園」就填「桃園」、說「8/20」就填「8/20」；正規化由系統處理）
- 買家明確說「可以接受類似品項」時 exact_match_required 才填 false，否則不要填`,
	strings.Join(slots.AllowedUnits, "/"))

func schema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"item":                 {Type: genai.TypeString, MaxLength: genai.Ptr(int64(slots.MaxTextLen))},
			"packaging":            {Type: genai.TypeString, MaxLength: genai.Ptr(int64(slots.MaxTextLen))},
			"quantity":             {Type: genai.TypeInteger, Minimum: genai.Ptr(float64(1)), Maximum: genai.Ptr(float64(slots.MaxQuantity))},
			"budget_per_unit":      {Type: genai.TypeInteger, Minimum: genai.Ptr(float64(1)), Maximum: genai.Ptr(float64(slots.MaxBudgetPerUnit))},
			"unit":                 {Type: genai.TypeString, Enum: slots.AllowedUnits},
			"delivery_city":        {Type: genai.TypeString},
			"delivery_date":        {Type: genai.TypeString},
			"exact_match_required": {Type: genai.TypeBoolean},
		},
	}
}

type known struct {
	Item               *string `json:"item,omitempty"`
	Quantity           *int32  `json:"quantity,omitempty"`
	Unit               *string `json:"unit,omitempty"`
	Packaging          *string `json:"packaging,omitempty"`
	BudgetPerUnit      *int64  `json:"budget_per_unit,omitempty"`
	DeliveryCity       *string `json:"delivery_city,omitempty"`
	DeliveryDate       *string `json:"delivery_date,omitempty"`
	ExactMatchRequired *bool   `json:"exact_match_required,omitempty"`
}

func (e *Extractor) request(message string, s slots.Set) (*adkmodel.LLMRequest, error) {
	k := known{Item: s.Item, Quantity: s.Quantity, Unit: s.Unit, Packaging: s.Packaging, BudgetPerUnit: s.BudgetPerUnit, DeliveryCity: s.DeliveryCity, ExactMatchRequired: s.ExactMatchRequired}
	if s.DeliveryDate != nil {
		d := s.DeliveryDate.Format(slots.DateLayout)
		k.DeliveryDate = &d
	}
	knownJSON, err := json.Marshal(k)
	if err != nil {
		return nil, fmt.Errorf("組已知需求：%w", err)
	}
	cfg := &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(instruction, genai.RoleUser),
		Temperature:       genai.Ptr(float32(0)),
	}
	switch e.shape {
	case ShapeToolCall:
		cfg.Tools = []*genai.Tool{{FunctionDeclarations: []*genai.FunctionDeclaration{{
			Name:        toolName,
			Description: "把買家訊息中明確提到的採購條件寫入這個 tool 的參數。",
			Parameters:  schema(),
		}}}}
		cfg.ToolConfig = &genai.ToolConfig{FunctionCallingConfig: &genai.FunctionCallingConfig{Mode: genai.FunctionCallingConfigModeAny}}
	case ShapeSchema:
		cfg.ResponseMIMEType = "application/json"
		cfg.ResponseSchema = schema()
	}
	text := fmt.Sprintf("目前已知的需求（JSON，勿重複輸出已正確的欄位）：\n%s\n\n買家訊息：\n%s", knownJSON, message)
	return &adkmodel.LLMRequest{
		Model:    e.modelName,
		Contents: []*genai.Content{genai.NewContentFromText(text, genai.RoleUser)},
		Config:   cfg,
	}, nil
}

type wire struct {
	Item               *string `json:"item"`
	Packaging          *string `json:"packaging"`
	Quantity           *int64  `json:"quantity"`
	BudgetPerUnit      *int64  `json:"budget_per_unit"`
	Unit               *string `json:"unit"`
	DeliveryCity       *string `json:"delivery_city"`
	DeliveryDate       *string `json:"delivery_date"`
	ExactMatchRequired *bool   `json:"exact_match_required"`
}

var errMalformed = errors.New("模型回應格式錯誤")

func (e *Extractor) parse(resp *adkmodel.LLMResponse) (slots.Patch, error) {
	if resp == nil || resp.Content == nil {
		return slots.Patch{}, fmt.Errorf("%w：回應為空", errMalformed)
	}
	var raw []byte
	switch e.shape {
	case ShapeToolCall:
		for _, part := range resp.Content.Parts {
			if part.FunctionCall != nil && part.FunctionCall.Name == toolName {
				b, err := json.Marshal(part.FunctionCall.Args)
				if err != nil {
					return slots.Patch{}, fmt.Errorf("%w：tool call 參數編碼失敗：%w", errMalformed, err)
				}
				raw = b
				break
			}
		}
		if raw == nil {
			return slots.Patch{}, fmt.Errorf("%w：沒有預期的 tool call", errMalformed)
		}
	case ShapeSchema:
		var sb strings.Builder
		for _, part := range resp.Content.Parts {
			if !part.Thought {
				sb.WriteString(part.Text)
			}
		}
		raw = []byte(strings.TrimSpace(sb.String()))
		if len(raw) == 0 {
			return slots.Patch{}, fmt.Errorf("%w：回應沒有文字", errMalformed)
		}
	}
	if len(raw) == 0 || raw[0] != '{' {
		return slots.Patch{}, fmt.Errorf("%w：不是 JSON 物件", errMalformed)
	}
	var w wire
	if err := json.Unmarshal(raw, &w); err != nil {
		return slots.Patch{}, fmt.Errorf("%w：%w", errMalformed, err)
	}
	return slots.Patch{
		Item: w.Item, Packaging: w.Packaging, Quantity: w.Quantity, BudgetPerUnit: w.BudgetPerUnit,
		Unit: w.Unit, DeliveryCity: w.DeliveryCity, DeliveryDate: w.DeliveryDate, ExactMatchRequired: w.ExactMatchRequired,
	}, nil
}
