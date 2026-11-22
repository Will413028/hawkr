package image

import "context"

// SanitizeFailing 在指定階段注入失敗，驗證錯誤的分類。
func SanitizeFailing(ctx context.Context, d []byte, lim Limits, stage string, err error) (Result, error) {
	return sanitize(ctx, d, lim, func(s string) error {
		if s == stage {
			return err
		}
		return nil
	})
}

const (
	StageColour  = stageColour
	StageAutorot = stageAutorot
	StageEncode  = stageEncode
)
