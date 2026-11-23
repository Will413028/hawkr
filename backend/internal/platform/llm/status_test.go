package llm

import (
	"errors"
	"fmt"
	"testing"

	"github.com/openai/openai-go/v3"
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
