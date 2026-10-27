package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	aidomain "github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/llmprovider"
	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/zhtw"
)

func NewSlotExtractor(p llmprovider.Procurement) SlotExtractor {
	if !p.Enabled() {
		return disabledSlotExtractor{}
	}
	return NewRetryingExtractor(newBaseSlotExtractor(p))
}

func newBaseSlotExtractor(p llmprovider.Procurement) SlotExtractor {
	switch p.Provider() {
	case "deepseek":
		return newDeepSeekSlotExtractor(p.LLM(), p.ModelName())
	case "gemma":
		return newGemmaSlotExtractor(p.LLM(), p.ModelName())
	default: // gemini——Enabled() 為真時 Provider() 必為三者之一
		return newGeminiSlotExtractor(p.LLM(), p.ModelName())
	}
}

const RetryTimeout = ExtractTimeout

type retryingExtractor struct {
	inner SlotExtractor
}

func NewRetryingExtractor(inner SlotExtractor) SlotExtractor {
	return &retryingExtractor{inner: inner}
}

const maxExtractAttempts = 2

func (r *retryingExtractor) Extract(ctx context.Context, message string, current domain.Slots) (ExtractResult, error) {
	start := time.Now()
	result, err := r.inner.Extract(ctx, message, current)
	attempt := 1

	var unavailable *aidomain.ProviderUnavailableError
	if err != nil && errors.As(err, &unavailable) {
		retryCtx, cancel := context.WithTimeout(ctx, RetryTimeout)
		result, err = r.inner.Extract(retryCtx, message, current)
		cancel()
		attempt = 2
	}

	observability.RecordProcurementExtractDuration(ctx, time.Since(start).Seconds())
	observability.RecordProcurementExtract(ctx, extractResultLabel(err))
	if err == nil {
		normalizeChineseFields(message, &result.Patch)
		result.Attempt = attempt
		result.MaxAttempts = maxExtractAttempts
	}
	return result, err
}

func normalizeChineseFields(message string, patch *domain.SlotPatch) {
	normalize := func(s *string) *string {
		if s == nil {
			return nil
		}
		if *s != "" && strings.Contains(message, *s) {
			return s
		}
		v := zhtw.ToTraditional(*s)
		return &v
	}
	patch.Item = normalize(patch.Item)
	patch.Packaging = normalize(patch.Packaging)
}

func extractResultLabel(err error) string {
	if err == nil {
		return "success"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "error"
}
