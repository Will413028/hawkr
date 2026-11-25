package llm

import (
	"errors"
	"fmt"
	"testing"

	"github.com/openai/openai-go/v3"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/openaimodel"
	"google.golang.org/genai"
)

func TestHTTPStatus(t *testing.T) {
	for name, c := range map[string]struct {
		err  error
		code int
		ok   bool
	}{
		"openai":      {&openai.Error{StatusCode: 429}, 429, true},
		"openai 包裝":   {fmt.Errorf("x: %w", &openai.Error{StatusCode: 503}), 503, true},
		"genai 值":     {genai.APIError{Code: 500}, 500, true},
		"genai 指標":    {&genai.APIError{Code: 400}, 400, true},
		"其他":          {errors.New("boom"), 0, false},
		"ErrDisabled": {ErrDisabled, 0, false},
	} {
		if code, ok := HTTPStatus(c.err); code != c.code || ok != c.ok {
			t.Errorf("%s：HTTPStatus = %d, %v，要 %d, %v", name, code, ok, c.code, c.ok)
		}
	}
}

func TestFinishDetail(t *testing.T) {
	for name, c := range map[string]struct {
		resp *model.LLMResponse
		want string
	}{
		"有說明":    {&model.LLMResponse{CustomMetadata: map[string]any{openaimodel.FinishMessageKey: "length"}}, "length"},
		"沒有 key": {&model.LLMResponse{CustomMetadata: map[string]any{"other": "x"}}, ""},
		"無 map":  {&model.LLMResponse{}, ""},
		"nil":    {nil, ""},
		"字面 key": {&model.LLMResponse{CustomMetadata: map[string]any{"openai_finish_message": "stop"}}, "stop"},
		"非字串":    {&model.LLMResponse{CustomMetadata: map[string]any{openaimodel.FinishMessageKey: 42}}, ""},
	} {
		if got := FinishDetail(c.resp); got != c.want {
			t.Errorf("%s：FinishDetail = %q，要 %q", name, got, c.want)
		}
	}
}
