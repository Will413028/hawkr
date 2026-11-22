package image

import (
	"context"

	"github.com/cshum/vipsgen/vips"
)

// SanitizeFailing 在指定階段注入失敗，驗證錯誤的分類。
func SanitizeFailing(ctx context.Context, d []byte, lim Limits, stage string, err error) (Result, error) {
	return sanitize(ctx, d, lim, func(s string) error {
		if s == stage {
			return err
		}
		return nil
	})
}

// AlphaOpaque 供測試直接驗證 alpha 滿值的判定。
func AlphaOpaque(a *vips.Image) (float64, error) { return alphaOpaque(a) }

const (
	StageColour  = stageColour
	StageAutorot = stageAutorot
	StageAlpha   = stageAlpha
	StageFlatten = stageFlatten
	StageEncode  = stageEncode
)
