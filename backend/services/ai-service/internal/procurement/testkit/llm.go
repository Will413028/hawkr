package testkit

import (
	"context"
	"iter"

	"google.golang.org/adk/v2/model"
)

// FakeLLM 是 model.LLM 的測試替身：GenerateContent 不打任何網路，直接回傳
// 建構時指定的單一 response/error，模擬 stream=false 的單次呼叫。
type FakeLLM struct {
	Resp    *model.LLMResponse
	Err     error
	LastReq *model.LLMRequest
}

func (f *FakeLLM) Name() string { return "fake" }

func (f *FakeLLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	f.LastReq = req
	return func(yield func(*model.LLMResponse, error) bool) {
		yield(f.Resp, f.Err)
	}
}
