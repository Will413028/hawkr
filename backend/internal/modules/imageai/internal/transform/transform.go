// Package transform 是賣家的一次 AI 修圖：驗證輸入、先佔同時修圖的名額、確認店家未退場並扣額度、呼叫圖片模型，失敗時退回額度。
package transform

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

const (
	busyRetryAfter = 10 * time.Second
	providerBusyRetryAfter = 30 * time.Second
)

var allowedMimeTypes = []string{"image/jpeg", "image/png", "image/webp"}

// Slots 是同時修圖的名額。
type Slots interface {
	TryAcquire() (release func(), ok bool)
}

// Quota 是修圖額度。
type Quota interface {
	Consume(ctx context.Context, storeID int64) (model.Usage, error)
	Refund(ctx context.Context, storeID int64, periodStart time.Time) error
}

// Guard 確認店家尚未退場：在呼叫方的 transaction 內鎖住店家列，持有到 transaction 結束；已退場與不存在回 NotFound。
type Guard interface {
	LockNotRetired(ctx context.Context, storeID int64) error
}

// Tx 開一個 transaction 執行 fn。
type Tx func(ctx context.Context, fn func(ctx context.Context) error) error

// Deps 是 Service 的依賴。
type Deps struct {
	Slots     Slots
	Quota     Quota
	Guard     Guard
	Tx        Tx
	Generator llm.ImageGenerator
	Log       *slog.Logger
}

// Service 執行修圖。
type Service struct{ d Deps }

// New 建立 Service。
func New(d Deps) *Service {
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	return &Service{d: d}
}

// Transform 以提示詞修一張圖。順序固定：驗證輸入；先佔同時修圖的名額（滿了回 resource_exhausted 帶 RetryInfo，不扣額度）；
// 在同一個 transaction 內確認店家未退場並扣額度；呼叫模型（transaction 之外）；模型呼叫失敗退回額度。名額最後才釋放。
// 扣額度之後程序終止的那一次不退回（A9-D38）。
func (s *Service) Transform(ctx context.Context, in model.TransformInput) (model.TransformOutput, error) {
	l := uitext.LocaleFrom(ctx)
	mime, prompt, err := validate(l, in)
	if err != nil {
		return model.TransformOutput{}, err
	}
	release, ok := s.d.Slots.TryAcquire()
	if !ok {
		return model.TransformOutput{}, apperr.ResourceExhausted(busyRetryAfter, "%s", text.TransformBusy(l))
	}
	defer release()

	usage, err := s.charge(ctx, l, in.StoreID)
	if err != nil {
		return model.TransformOutput{}, err
	}
	res, err := s.generate(ctx, l, llm.ImageRequest{Image: in.Image, MimeType: mime, Prompt: prompt})
	if err != nil {
		if rerr := s.d.Quota.Refund(ctx, in.StoreID, usage.PeriodStart); rerr != nil {
			s.d.Log.ErrorContext(ctx, "imageai：退回修圖額度失敗", "store_id", in.StoreID, "error", rerr)
		}
		return model.TransformOutput{}, err
	}
	return model.TransformOutput{Image: res.Image, MimeType: res.MimeType, Usage: usage}, nil
}

func validate(l uitext.Locale, in model.TransformInput) (mime, prompt string, err error) {
	if len(in.Image) == 0 {
		return "", "", apperr.InvalidInputOn([]string{"Image"}, "%s", text.TransformImageEmpty(l))
	}
	if len(in.Image) > model.MaxImageBytes {
		return "", "", apperr.InvalidInputOn([]string{"Image"}, "%s", text.TransformImageTooLarge(l, text.TransformImageTooLargeArgs{Max: model.MaxImageBytes >> 20}))
	}
	prompt = trimtext.Trim(in.Prompt)
	if n := utf8.RuneCountInString(prompt); n == 0 || n > model.MaxPromptRunes {
		return "", "", apperr.InvalidInputOn([]string{"Prompt"}, "%s", text.TransformPromptLength(l, text.TransformPromptLengthArgs{Max: model.MaxPromptRunes}))
	}
	declared := strings.ToLower(strings.TrimSpace(in.MimeType))
	if !slices.Contains(allowedMimeTypes, declared) {
		return "", "", apperr.InvalidInputOn([]string{"MimeType"}, "%s", text.TransformMimeUnsupported(l))
	}
	sniffed, _, _ := strings.Cut(http.DetectContentType(in.Image), ";")
	if !slices.Contains(allowedMimeTypes, sniffed) {
		return "", "", apperr.InvalidInputOn([]string{"Image"}, "%s", text.TransformMimeUnsupported(l))
	}
	if sniffed != declared {
		return "", "", apperr.InvalidInputOn([]string{"MimeType"}, "%s", text.TransformMimeMismatch(l))
	}
	return sniffed, prompt, nil
}

func (s *Service) charge(ctx context.Context, l uitext.Locale, storeID int64) (model.Usage, error) {
	var usage model.Usage
	err := s.d.Tx(ctx, func(ctx context.Context) error {
		if err := s.d.Guard.LockNotRetired(ctx, storeID); err != nil {
			return err
		}
		u, err := s.d.Quota.Consume(ctx, storeID)
		usage = u
		return err
	})
	switch {
	case err == nil:
		return usage, nil
	case errors.Is(err, model.ErrQuotaExhausted):
		return usage, apperr.ResourceExhausted(0, "%s", text.TransformQuotaExceeded(l)).
			WithReason(apperr.ReasonImageQuotaExceeded).
			WithMetadata("remaining", "0").
			WithMetadata("resetTime", usage.ResetTime.UTC().Format(time.RFC3339))
	case apperr.KindOf(err) == apperr.KindNotFound:
		return usage, apperr.PermissionDenied("%s", text.TransformStoreUnavailable(l))
	}
	return usage, err
}

func (s *Service) generate(ctx context.Context, l uitext.Locale, req llm.ImageRequest) (llm.ImageResult, error) {
	for attempt := 1; ; attempt++ {
		res, err := s.d.Generator.GenerateImage(ctx, req)
		if err != nil {
			return llm.ImageResult{}, providerError(ctx, l, err)
		}
		if len(res.Image) > 0 {
			return res, nil
		}
		switch refusal(res) {
		case refusalSafety:
			return llm.ImageResult{}, apperr.InvalidInput("%s", text.TransformSafetyBlocked(l)).WithReason(apperr.ReasonImageSafetyBlocked)
		case refusalRecitation:
			return llm.ImageResult{}, apperr.InvalidInput("%s", text.TransformRecitationBlocked(l)).WithReason(apperr.ReasonImageRecitationBlocked)
		case refusalNone:
		}
		if attempt == 2 {
			return llm.ImageResult{}, apperr.Unavailable("%s", text.TransformNotProduced(l)).WithReason(apperr.ReasonImageNotProduced)
		}
	}
}

func providerError(ctx context.Context, l uitext.Locale, err error) error {
	if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
		return err
	}
	if code, ok := llm.HTTPStatus(err); ok && code == http.StatusTooManyRequests {
		return apperr.WithCause(apperr.ResourceExhausted(providerBusyRetryAfter, "%s", text.TransformProviderBusy(l)), err)
	}
	return apperr.WithCause(apperr.Unavailable("%s", text.TransformProviderUnavailable(l)), err)
}

const blockReasonUnspecified = "BLOCKED_REASON_UNSPECIFIED"

type refusalKind int

const (
	refusalNone refusalKind = iota
	refusalSafety
	refusalRecitation
)

func refusal(r llm.ImageResult) refusalKind {
	if r.BlockReason != "" && r.BlockReason != blockReasonUnspecified {
		return refusalSafety
	}
	switch r.FinishReason {
	case "IMAGE_SAFETY", "IMAGE_PROHIBITED_CONTENT", "SAFETY", "PROHIBITED_CONTENT", "BLOCKLIST", "SPII":
		return refusalSafety
	case "IMAGE_RECITATION", "RECITATION":
		return refusalRecitation
	}
	return refusalNone
}
