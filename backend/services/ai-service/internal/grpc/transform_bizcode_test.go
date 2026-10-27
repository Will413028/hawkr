package grpc

import (
	"errors"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
)

// provider 的每一種判定碼都必須落到自己的業務碼上。舊碼把 finishReason != "STOP"
// 全部壓成「安全性檢查」+ HTTP 500，NO_IMAGE 因此被誤導成「請更換照片」。
func TestAiTransformBizError(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode int32
		wantMsg  string
		wantOK   bool
	}{
		{
			name:     "NO_IMAGE 是可重試的沒產圖，不是安全性阻擋",
			err:      &domain.ProviderRefusalError{FinishReason: domain.FinishReasonNoImage},
			wantCode: bizcode.AiTransformNoImage,
			wantMsg:  msgAiTransformNoImage,
			wantOK:   true,
		},
		{
			name:     "IMAGE_OTHER 併入沒產圖",
			err:      &domain.ProviderRefusalError{FinishReason: domain.FinishReasonImageOther},
			wantCode: bizcode.AiTransformNoImage,
			wantMsg:  msgAiTransformNoImage,
			wantOK:   true,
		},
		{
			name:     "IMAGE_SAFETY 才是安全性阻擋",
			err:      &domain.ProviderRefusalError{FinishReason: domain.FinishReasonImageSafety},
			wantCode: bizcode.AiTransformSafetyBlocked,
			wantMsg:  msgAiTransformSafetyBlocked,
			wantOK:   true,
		},
		{
			name:     "IMAGE_PROHIBITED_CONTENT 併入安全性阻擋",
			err:      &domain.ProviderRefusalError{FinishReason: domain.FinishReasonImageProhibited},
			wantCode: bizcode.AiTransformSafetyBlocked,
			wantMsg:  msgAiTransformSafetyBlocked,
			wantOK:   true,
		},
		{
			name:     "prompt 側 blockReason 一律安全性阻擋",
			err:      &domain.ProviderRefusalError{BlockReason: "SAFETY"},
			wantCode: bizcode.AiTransformSafetyBlocked,
			wantMsg:  msgAiTransformSafetyBlocked,
			wantOK:   true,
		},
		{
			name:     "IMAGE_RECITATION 有自己的文案",
			err:      &domain.ProviderRefusalError{FinishReason: domain.FinishReasonImageRecitation},
			wantCode: bizcode.AiTransformRecitation,
			wantMsg:  msgAiTransformRecitation,
			wantOK:   true,
		},
		{
			name:     "呼叫 provider 失敗是基礎設施問題",
			err:      &domain.ProviderUnavailableError{Err: errors.New("timeout")},
			wantCode: bizcode.AiProviderUnavailable,
			wantMsg:  msgAiProviderUnavailable,
			wantOK:   true,
		},
		{
			name:   "無法歸類的錯誤不吃業務碼",
			err:    errors.New("resolve limit: db down"),
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, msg, ok := aiTransformBizError(tc.err)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if code != tc.wantCode {
				t.Errorf("code = %d, want %d", code, tc.wantCode)
			}
			if msg != tc.wantMsg {
				t.Errorf("msg = %q, want %q", msg, tc.wantMsg)
			}
		})
	}
}

// 包過一層（例如 fmt.Errorf("%w")）仍要能分類，否則 usecase 一加 context 就退化成 500。
func TestAiTransformBizError_UnwrapsWrappedErrors(t *testing.T) {
	wrapped := errors.Join(errors.New("context"), &domain.ProviderRefusalError{FinishReason: domain.FinishReasonNoImage})
	code, _, ok := aiTransformBizError(wrapped)
	if !ok || code != bizcode.AiTransformNoImage {
		t.Errorf("code = %d ok = %v, want %d true", code, ok, bizcode.AiTransformNoImage)
	}
}
