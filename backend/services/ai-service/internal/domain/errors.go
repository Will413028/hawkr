package domain

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidInput indicates invalid input parameters
	ErrInvalidInput = errors.New("invalid input")

	// ErrImageProcessingFailed indicates image processing failed
	ErrImageProcessingFailed = errors.New("image processing failed")

	// ErrRateLimitExceeded indicates rate limit was exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// Gemini candidate 側 finishReason。影像生成的失敗語意分佈在這幾個值上，
// 彼此的處置方式不同（可否重試、要不要請使用者換圖），不可壓成同一種錯誤。
const (
	FinishReasonStop            = "STOP"
	FinishReasonNoImage         = "NO_IMAGE"
	FinishReasonImageSafety     = "IMAGE_SAFETY"
	FinishReasonImageProhibited = "IMAGE_PROHIBITED_CONTENT"
	FinishReasonImageRecitation = "IMAGE_RECITATION"
	FinishReasonImageOther      = "IMAGE_OTHER"
)

// ProviderRefusalError 表示 AI provider 收下了請求、但沒有回傳圖片。
//
// FinishReason / BlockReason 是 provider 的原始判定碼，本型別只負責原樣攜帶，
// 不在這層翻成平台語意或使用者文案——對應「PayUni 錯誤回應收斂規範」的
// provider SoT 層，以及該規範 §8.3「禁止自創 error_type 做錯誤分類，provider
// 錯誤碼本身就是分類依據」。平台業務碼與文案由 gRPC handler 依 CodeMeta 映射。
type ProviderRefusalError struct {
	// FinishReason 是 candidate 側原因（NO_IMAGE / IMAGE_SAFETY /
	// IMAGE_PROHIBITED_CONTENT / IMAGE_RECITATION / IMAGE_OTHER）。
	FinishReason string
	// BlockReason 是 prompt 側原因（SAFETY / BLOCKLIST / PROHIBITED_CONTENT /
	// IMAGE_SAFETY），只在整個請求在送進模型前就被擋下時出現。
	BlockReason string
}

func (e *ProviderRefusalError) Error() string {
	if e.BlockReason != "" {
		return fmt.Sprintf("ai provider refused: blockReason=%s", e.BlockReason)
	}
	if e.FinishReason != "" {
		return fmt.Sprintf("ai provider refused: finishReason=%s", e.FinishReason)
	}
	return "ai provider returned no image"
}

// Reason 回傳用於分類與 metric label 的原始判定碼：prompt 側優先（它代表請求
// 根本沒進模型），否則用 candidate 側。兩者皆空時回 FinishReasonImageOther，
// 讓呼叫端永遠拿得到一個可用的分類鍵，不必自己處理空字串。
func (e *ProviderRefusalError) Reason() string {
	if e.BlockReason != "" {
		return e.BlockReason
	}
	if e.FinishReason != "" {
		return e.FinishReason
	}
	return FinishReasonImageOther
}

// Retryable 回報「同一份輸入原封不動重送」是否可能成功。
//
// NO_IMAGE / IMAGE_OTHER 是模型的非決定性行為（prod log 實測：同一張圖、同一個
// prompt 連打 5 次，3 次成功 2 次 NO_IMAGE），重試會成功；安全性與版權判定則是
// 對內容本身的確定性判斷，重送幾次都一樣，只能請使用者換圖或改描述。
// prompt 側被擋（BlockReason 非空）一律不重試，理由同上。
func (e *ProviderRefusalError) Retryable() bool {
	if e.BlockReason != "" {
		return false
	}
	switch e.FinishReason {
	case FinishReasonNoImage, FinishReasonImageOther, "":
		// 空值＝provider 回了無法歸類的空結果，比照非決定性失敗處理。
		return true
	default:
		return false
	}
}

// ProviderUnavailableError 表示呼叫 AI provider 這件事本身失敗（連線失敗、逾時、
// provider 回 5xx、回應無法解析），與「provider 有回應但拒絕產圖」
// （ProviderRefusalError）是不同層次的問題：前者是基礎設施狀況、後者是內容判定。
type ProviderUnavailableError struct {
	Err error
}

func (e *ProviderUnavailableError) Error() string {
	return fmt.Sprintf("ai provider unavailable: %v", e.Err)
}

func (e *ProviderUnavailableError) Unwrap() error { return e.Err }
