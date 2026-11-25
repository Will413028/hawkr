package llm

import (
	"errors"

	"github.com/openai/openai-go/v3"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/model/openaimodel"
	"google.golang.org/genai"
)

// HTTPStatus 取出 provider 錯誤帶的 HTTP 狀態碼（DeepSeek 的 openai 相容 API、Gemini 的 genai）；不是 provider 的 HTTP 錯誤回 ok=false。
// 呼叫端（模組）不能 import provider SDK，分類重試與失敗原因靠它。
func HTTPStatus(err error) (code int, ok bool) {
	var oe *openai.Error
	if errors.As(err, &oe) {
		return oe.StatusCode, true
	}
	var ge genai.APIError
	if errors.As(err, &ge) {
		return ge.Code, true
	}
	var gp *genai.APIError
	if errors.As(err, &gp) && gp != nil {
		return gp.Code, true
	}
	return 0, false
}

// FinishDetail 取出 provider 對結束原因的補充說明（例如 openai 相容 API 的 finish_message）；沒有、不是字串或 resp 為 nil 時回空字串。
// 呼叫端（模組）不能 import provider SDK，回報結束原因時靠它。
func FinishDetail(resp *model.LLMResponse) string {
	if resp == nil {
		return ""
	}
	detail, _ := resp.CustomMetadata[openaimodel.FinishMessageKey].(string)
	return detail
}
