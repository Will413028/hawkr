package extract

import (
	"context"
	"errors"
	"iter"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	adkmodel "google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"github.com/Will413028/hawkr/backend/internal/modules/procurement/internal/slots"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
)

// step 是替身 LLM 一次呼叫的回應。
type step struct {
	resp  *adkmodel.LLMResponse
	err   error
	block bool // 阻塞到 ctx 結束，模擬逾時
}

type fakeLLM struct {
	steps []step
	calls atomic.Int32
	reqs  []*adkmodel.LLMRequest
}

func (f *fakeLLM) Name() string { return "fake" }

func (f *fakeLLM) GenerateContent(ctx context.Context, req *adkmodel.LLMRequest, _ bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	n := int(f.calls.Add(1)) - 1
	f.reqs = append(f.reqs, req)
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		s := f.steps[min(n, len(f.steps)-1)]
		if s.block {
			<-ctx.Done()
			yield(nil, ctx.Err())
			return
		}
		yield(s.resp, s.err)
	}
}

func text(s string) step {
	return step{resp: &adkmodel.LLMResponse{Content: genai.NewContentFromText(s, genai.RoleModel), FinishReason: genai.FinishReasonStop}}
}

func toolCall(args map[string]any) step {
	return step{resp: &adkmodel.LLMResponse{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{{FunctionCall: &genai.FunctionCall{Name: toolName, Args: args}}}}, FinishReason: genai.FinishReasonStop}}
}

func newTest(f *fakeLLM, shape Shape) *Extractor {
	e := New(f, "m", shape)
	e.backoff = 0
	e.sleep = func(context.Context, time.Duration) error { return nil }
	e.jitter = func() float64 { return 0 }
	return e
}

func strPtr(s string) *string { return &s }

func TestSchemaShapeParsesJSONAndIgnoresResolution(t *testing.T) {
	f := &fakeLLM{steps: []step{text(`{"item":"小白菜","quantity":50,"unit":"KG","delivery_city":"桃園","item_resolution":{"kind":"CATEGORY_CONFIRMED","label":"x"}}`)}}
	p, err := newTest(f, ShapeSchema).Extract(context.Background(), "我要50公斤小白菜送桃園", slots.Set{})
	if err != nil {
		t.Fatalf("Extract：%v", err)
	}
	if p.Item == nil || *p.Item != "小白菜" || p.Quantity == nil || *p.Quantity != 50 || p.Unit == nil || *p.Unit != "KG" || p.DeliveryCity == nil || *p.DeliveryCity != "桃園" {
		t.Errorf("patch = %+v", p)
	}
	if p.Resolution != nil {
		t.Error("模型不得決定品項解析")
	}
	cfg := f.reqs[0].Config
	if cfg.ResponseSchema == nil || cfg.ResponseMIMEType != "application/json" || len(cfg.Tools) != 0 {
		t.Errorf("schema 形狀的請求不對：%+v", cfg)
	}
}

func TestToolCallShapeForcesTheToolAndParsesArgs(t *testing.T) {
	f := &fakeLLM{steps: []step{toolCall(map[string]any{"item": "高麗菜", "budget_per_unit": 30, "exact_match_required": false})}}
	p, err := newTest(f, ShapeToolCall).Extract(context.Background(), "高麗菜 一斤30", slots.Set{})
	if err != nil {
		t.Fatalf("Extract：%v", err)
	}
	if p.Item == nil || *p.Item != "高麗菜" || p.BudgetPerUnit == nil || *p.BudgetPerUnit != 30 || p.ExactMatchRequired == nil || *p.ExactMatchRequired {
		t.Errorf("patch = %+v", p)
	}
	cfg := f.reqs[0].Config
	if len(cfg.Tools) != 1 || cfg.ToolConfig == nil || cfg.ToolConfig.FunctionCallingConfig.Mode != genai.FunctionCallingConfigModeAny || len(cfg.ToolConfig.FunctionCallingConfig.AllowedFunctionNames) != 0 || cfg.ResponseSchema != nil {
		t.Errorf("tool 形狀的請求不對：%+v", cfg)
	}
}

func TestEmptyExtractionIsSuccess(t *testing.T) {
	f := &fakeLLM{steps: []step{text(`{}`)}}
	p, err := newTest(f, ShapeSchema).Extract(context.Background(), "你好", slots.Set{})
	if err != nil || p != (slots.Patch{}) || f.calls.Load() != 1 {
		t.Errorf("patch = %+v, err = %v, calls = %d：空抽取是成功", p, err, f.calls.Load())
	}
}

func TestMalformedThenValidRetriesExactlyOnce(t *testing.T) {
	for name, bad := range map[string]step{
		"壞 JSON":     text(`{"item":`),
		"不是物件":       text(`null`),
		"型別不合":       text(`{"quantity":"很多"}`),
		"沒有文字":       {resp: &adkmodel.LLMResponse{Content: &genai.Content{}, FinishReason: genai.FinishReasonStop}},
		"沒有 content": {resp: &adkmodel.LLMResponse{FinishReason: genai.FinishReasonStop}},
	} {
		f := &fakeLLM{steps: []step{bad, text(`{"item":"蘋果"}`)}}
		p, err := newTest(f, ShapeSchema).Extract(context.Background(), "蘋果", slots.Set{})
		if err != nil || p.Item == nil || *p.Item != "蘋果" || f.calls.Load() != 2 {
			t.Errorf("%s：patch = %+v, err = %v, calls = %d，要重試一次後成功", name, p, err, f.calls.Load())
		}
	}
}

func TestToolCallMissingIsMalformedAndRetried(t *testing.T) {
	f := &fakeLLM{steps: []step{text(`純文字`), toolCall(map[string]any{"item": "蘋果"})}}
	if p, err := newTest(f, ShapeToolCall).Extract(context.Background(), "蘋果", slots.Set{}); err != nil || p.Item == nil || f.calls.Load() != 2 {
		t.Errorf("patch = %+v, err = %v, calls = %d", p, err, f.calls.Load())
	}
}

func TestTwoMalformedAnswersFailAsNotUnavailable(t *testing.T) {
	f := &fakeLLM{steps: []step{text(`{`)}}
	_, err := newTest(f, ShapeSchema).Extract(context.Background(), "x", slots.Set{})
	var e *Error
	if !errors.As(err, &e) || e.Unavailable || f.calls.Load() != 2 {
		t.Errorf("err = %v, calls = %d：兩次格式錯誤要失敗，且不是服務不可用", err, f.calls.Load())
	}
}

func TestRetryableStatusesRetryOnceAndExhaustionIsUnavailable(t *testing.T) {
	for name, apiErr := range map[string]error{
		"429":          genai.APIError{Code: 429},
		"503":          genai.APIError{Code: 503},
		"genai 500":    genai.APIError{Code: 500},
		"genai 429 指標": &genai.APIError{Code: 429},
	} {
		f := &fakeLLM{steps: []step{{err: apiErr}}}
		_, err := newTest(f, ShapeSchema).Extract(context.Background(), "x", slots.Set{})
		var e *Error
		if !errors.As(err, &e) || !e.Unavailable || f.calls.Load() != 2 {
			t.Errorf("%s：err = %v, calls = %d，要重試一次並回服務不可用", name, err, f.calls.Load())
		}
		f = &fakeLLM{steps: []step{{err: apiErr}, text(`{"item":"蘋果"}`)}}
		if p, err := newTest(f, ShapeSchema).Extract(context.Background(), "x", slots.Set{}); err != nil || p.Item == nil {
			t.Errorf("%s：重試後成功，err = %v", name, err)
		}
	}
}

func TestNonRetryableErrorsAreNotRetried(t *testing.T) {
	for name, callErr := range map[string]error{
		"400":       genai.APIError{Code: 400},
		"401":       genai.APIError{Code: 401},
		"genai 403": genai.APIError{Code: 403},
		"停用":        llm.ErrDisabled,
		"未知錯誤":      errors.New("boom"),
	} {
		f := &fakeLLM{steps: []step{{err: callErr}}}
		_, err := newTest(f, ShapeSchema).Extract(context.Background(), "x", slots.Set{})
		var e *Error
		if !errors.As(err, &e) || e.Unavailable || f.calls.Load() != 1 {
			t.Errorf("%s：err = %v, calls = %d，不得重試也不是服務不可用", name, err, f.calls.Load())
		}
	}
}

func TestAttemptTimeoutRetriesThenIsUnavailable(t *testing.T) {
	f := &fakeLLM{steps: []step{{block: true}}}
	e := newTest(f, ShapeSchema)
	e.attemptTimeout = 5 * time.Millisecond
	_, err := e.Extract(context.Background(), "x", slots.Set{})
	var ee *Error
	if !errors.As(err, &ee) || !ee.Unavailable || f.calls.Load() != 2 {
		t.Errorf("err = %v, calls = %d", err, f.calls.Load())
	}
}

func TestParentCancellationIsReturnedAsIsWithoutRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	f := &fakeLLM{steps: []step{{block: true}}}
	go func() {
		for f.calls.Load() == 0 {
			time.Sleep(time.Millisecond)
		}
		cancel()
	}()
	_, err := newTest(f, ShapeSchema).Extract(ctx, "x", slots.Set{})
	if !errors.Is(err, context.Canceled) || f.calls.Load() != 1 {
		t.Errorf("err = %v, calls = %d：買家取消不得重試也不得包成失敗分類", err, f.calls.Load())
	}
	var ee *Error
	if errors.As(err, &ee) {
		t.Error("取消不是抽取失敗")
	}
}

func TestMessageIsTruncatedAtTheRuneLimitAndKnownSlotsAreSent(t *testing.T) {
	f := &fakeLLM{steps: []step{text(`{}`)}}
	long := strings.Repeat("菜", MaxMessageRunes+500)
	known := slots.Set{Item: strPtr("小白菜")}
	if _, err := newTest(f, ShapeSchema).Extract(context.Background(), long, known); err != nil {
		t.Fatal(err)
	}
	sent := f.reqs[0].Contents[0].Parts[0].Text
	// 已知需求 JSON 裡的「小白菜」含一個「菜」。
	if got := strings.Count(sent, "菜"); got != MaxMessageRunes+1 {
		t.Errorf("「菜」共 %d 個，要 %d 個（截斷後的訊息加已知需求裡的 1 個）", got, MaxMessageRunes+1)
	}
	if !strings.Contains(sent, `"item":"小白菜"`) {
		t.Errorf("prompt 沒帶已知需求：%s", sent)
	}
}

func TestShapeFor(t *testing.T) {
	if s, err := ShapeFor(llm.Gemini); err != nil || s != ShapeSchema {
		t.Errorf("Gemini = %v, %v", s, err)
	}
	if s, err := ShapeFor(llm.DeepSeek); err != nil || s != ShapeToolCall {
		t.Errorf("DeepSeek = %v, %v", s, err)
	}
	if _, err := ShapeFor("other"); err == nil {
		t.Error("不認得的 provider 要回錯誤")
	}
}

func TestErrorCodeResponseIsNotRetriedAndNotUnavailable(t *testing.T) {
	for _, code := range []string{"SAFETY", "PROHIBITED_CONTENT", "RECITATION", "BLOCKLIST", "SPII", "MAX_TOKENS", "content_filter"} {
		f := &fakeLLM{steps: []step{{resp: &adkmodel.LLMResponse{ErrorCode: code}}, text(`{"item":"蘋果"}`)}}
		_, err := newTest(f, ShapeSchema).Extract(context.Background(), "蘋果", slots.Set{})
		var ee *Error
		if !errors.As(err, &ee) || ee.Unavailable || f.calls.Load() != 1 {
			t.Errorf("%s：err = %v, calls = %d，要一次失敗、不重試、不算服務不可用", code, err, f.calls.Load())
		}
	}
}

func TestUnclearOutputErrorCodesRetryOnceAndAreNotUnavailable(t *testing.T) {
	for _, code := range []string{"OTHER", "MALFORMED_FUNCTION_CALL"} {
		ok := &fakeLLM{steps: []step{{resp: &adkmodel.LLMResponse{ErrorCode: code}}, text(`{"item":"蘋果"}`)}}
		if _, err := newTest(ok, ShapeSchema).Extract(context.Background(), "蘋果", slots.Set{}); err != nil || ok.calls.Load() != 2 {
			t.Errorf("%s：err = %v, calls = %d，要重試一次後成功", code, err, ok.calls.Load())
		}
		bad := &fakeLLM{steps: []step{{resp: &adkmodel.LLMResponse{ErrorCode: code}}, {resp: &adkmodel.LLMResponse{ErrorCode: code}}}}
		_, err := newTest(bad, ShapeSchema).Extract(context.Background(), "蘋果", slots.Set{})
		var ee *Error
		if !errors.As(err, &ee) || ee.Unavailable || bad.calls.Load() != 2 {
			t.Errorf("%s：err = %v, calls = %d，用盡後要失敗且不算服務不可用", code, err, bad.calls.Load())
		}
	}
}

func TestNonStopFinishReasonIsNotAnAnswer(t *testing.T) {
	withReason := func(r genai.FinishReason) step {
		return step{resp: &adkmodel.LLMResponse{Content: genai.NewContentFromText(`{"item":"蘋果"}`, genai.RoleModel), FinishReason: r}}
	}
	for name, tc := range map[string]struct {
		reason genai.FinishReason
		calls  int32
	}{
		"MAX_TOKENS":  {genai.FinishReasonMaxTokens, 1},
		"SAFETY":      {genai.FinishReasonSafety, 1},
		"LANGUAGE":    {genai.FinishReasonLanguage, 1},
		"UNSPECIFIED": {genai.FinishReasonUnspecified, 2},
		"空值":          {"", 2},
		"OTHER":       {genai.FinishReasonOther, 2},
	} {
		f := &fakeLLM{steps: []step{withReason(tc.reason)}}
		_, err := newTest(f, ShapeSchema).Extract(context.Background(), "蘋果", slots.Set{})
		var ee *Error
		if !errors.As(err, &ee) || ee.Unavailable || f.calls.Load() != tc.calls {
			t.Errorf("%s：err = %v, calls = %d，要呼叫 %d 次後失敗且不算服務不可用", name, err, f.calls.Load(), tc.calls)
		}
	}
	f := &fakeLLM{steps: []step{withReason(genai.FinishReasonStop)}}
	if p, err := newTest(f, ShapeSchema).Extract(context.Background(), "蘋果", slots.Set{}); err != nil || p.Item == nil || *p.Item != "蘋果" || f.calls.Load() != 1 {
		t.Errorf("STOP：patch = %+v, err = %v, calls = %d", p, err, f.calls.Load())
	}
	f = &fakeLLM{steps: []step{withReason(genai.FinishReasonOther), withReason(genai.FinishReasonStop)}}
	if p, err := newTest(f, ShapeSchema).Extract(context.Background(), "蘋果", slots.Set{}); err != nil || p.Item == nil || *p.Item != "蘋果" || f.calls.Load() != 2 {
		t.Errorf("OTHER 後 STOP：patch = %+v, err = %v, calls = %d", p, err, f.calls.Load())
	}
}

func TestNonStopFinishReasonOnToolCall(t *testing.T) {
	f := &fakeLLM{steps: []step{toolCall(map[string]any{"item": "蘋果"})}}
	if p, err := newTest(f, ShapeToolCall).Extract(context.Background(), "蘋果", slots.Set{}); err != nil || p.Item == nil || f.calls.Load() != 1 {
		t.Errorf("STOP：patch = %+v, err = %v, calls = %d", p, err, f.calls.Load())
	}
	cut := toolCall(map[string]any{"item": "蘋果"})
	cut.resp.FinishReason = genai.FinishReasonMaxTokens
	f = &fakeLLM{steps: []step{cut}}
	_, err := newTest(f, ShapeToolCall).Extract(context.Background(), "蘋果", slots.Set{})
	var ee *Error
	if !errors.As(err, &ee) || ee.Unavailable || f.calls.Load() != 1 {
		t.Errorf("MAX_TOKENS：err = %v, calls = %d，要呼叫 1 次後失敗", err, f.calls.Load())
	}
}

func TestFinishReasonErrorCarriesProviderDetail(t *testing.T) {
	cut := text(`{"item":"蘋果"}`)
	cut.resp.FinishReason = genai.FinishReasonMaxTokens
	cut.resp.CustomMetadata = map[string]any{"openai_finish_message": "length"}
	f := &fakeLLM{steps: []step{cut}}
	_, err := newTest(f, ShapeSchema).Extract(context.Background(), "蘋果", slots.Set{})
	if err == nil || !strings.Contains(err.Error(), "length") {
		t.Errorf("err = %v，要帶供應商的結束說明", err)
	}
}
