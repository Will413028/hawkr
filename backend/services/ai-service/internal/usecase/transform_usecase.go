package usecase

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/observability"
	"github.com/Will413028/hawkr/ai-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/ai-service/internal/usecase/dto"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/httpclient"
	"github.com/Will413028/hawkr/go-pkg/identity"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	pkgtsid "github.com/Will413028/hawkr/go-pkg/tsid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const reservationTTL = 5 * time.Minute

type TransformUseCase struct {
	pool      *pgxpool.Pool
	txManager database.TxManager
	apiKey    string
	apiURL    string
	genID     func() int64

	admission     *transformAdmission
	admissionOnce sync.Once
}

func (uc *TransformUseCase) admitter() *transformAdmission {
	uc.admissionOnce.Do(func() {
		if uc.admission == nil {
			uc.admission = newTransformAdmission()
		}
	})
	return uc.admission
}

func NewTransformUseCase(pool *pgxpool.Pool, cfg *config.Config) *TransformUseCase {
	return &TransformUseCase{
		pool:      pool,
		txManager: database.NewTxManager(pool),

		apiKey: strings.TrimSpace(cfg.GeminiAPIKey),
		apiURL: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-image:generateContent",
		genID:  pkgtsid.NewGenerator(),
	}
}

func (uc *TransformUseCase) q(ctx context.Context) *gen.Queries {
	return gen.New(database.GetDBTX(ctx, uc.pool))
}

func parseWeekStart(weekStart string) (pgtype.Date, error) {
	t, err := time.Parse("2006-01-02", weekStart)
	if err != nil {
		var zeroDate pgtype.Date
		return zeroDate, fmt.Errorf("parse week start: %w", err)
	}
	return pgxutil.DateParam(&t), nil
}

func (uc *TransformUseCase) Transform(ctx context.Context, storeID identity.StoreID, imageData []byte, mimeType, prompt string) (*dto.TransformResponse, error) {
	if uc.apiKey == "" {
		return nil, fmt.Errorf("AI service not configured")
	}

	release, err := uc.admitter().admit(len(imageData))
	if err != nil {
		return nil, err
	}
	defer release()

	weekStart := currentWeekStart()

	limit, err := uc.resolveLimit(ctx, storeID)
	if err != nil {
		return nil, fmt.Errorf("resolve limit: %w", err)
	}
	if limit <= 0 {
		return nil, &QuotaExceededError{Usage: 0, Limit: limit}
	}

	reservationID, confirmedBefore, reserved, err := uc.reserveSlot(ctx, storeID, weekStart, limit)
	if err != nil {
		return nil, fmt.Errorf("reserve slot: %w", err)
	}
	if !reserved {
		return nil, &QuotaExceededError{Usage: limit, Limit: limit}
	}

	resultBase64, resultMime, err := uc.callGeminiWithRetry(ctx, imageData, mimeType, prompt)
	observability.RecordAITransform(ctx, transformOutcome(err))
	if err != nil {

		uc.releaseSlot(context.WithoutCancel(ctx), reservationID, storeID)

		return nil, err
	}

	if err := uc.confirmSlot(context.WithoutCancel(ctx), reservationID, storeID, weekStart); err != nil {

		pkglogger.Error(ctx, "confirm ai_usage slot failed", zap.Int64("store_id", int64(storeID)), zap.Error(err))
	}

	return &dto.TransformResponse{
		ImageBase64: resultBase64,
		MimeType:    resultMime,
		WeeklyUsage: strconv.Itoa(confirmedBefore + 1),
		WeeklyLimit: strconv.Itoa(limit),
	}, nil
}

func (uc *TransformUseCase) resolveLimit(ctx context.Context, storeID identity.StoreID) (int, error) {
	quota, err := uc.q(ctx).GetAiQuotaByStoreID(ctx, int64(storeID))
	if err == nil {
		return int(quota.WeeklyLimit), nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.DefaultWeeklyLimit, nil
	}
	return 0, err
}

func (uc *TransformUseCase) reserveSlot(ctx context.Context, storeID identity.StoreID, weekStart string, limit int) (int64, int, bool, error) {
	pgWeek, err := parseWeekStart(weekStart)
	if err != nil {
		return 0, 0, false, err
	}

	var reservationID int64
	var confirmedBefore int
	var reserved bool

	err = uc.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := uc.q(txCtx)

		if err := q.AcquireAiUsageLock(txCtx, int64(storeID)); err != nil {
			return err
		}

		usage, err := q.GetAiUsageByStoreIDAndWeek(txCtx, gen.GetAiUsageByStoreIDAndWeekParams{
			StoreID: int64(storeID), WeekStart: pgWeek,
		})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		confirmedBefore = int(usage.UsageCount)

		pending, err := q.CountPendingAiUsageReservations(txCtx, gen.CountPendingAiUsageReservationsParams{
			StoreID: int64(storeID), WeekStart: pgWeek,
		})
		if err != nil {
			return err
		}

		if confirmedBefore+int(pending) >= limit {
			return nil
		}

		if err := q.DeleteExpiredAiUsageReservations(txCtx); err != nil {
			return err
		}

		reservationID = uc.genID()
		if err := q.CreateAiUsageReservation(txCtx, gen.CreateAiUsageReservationParams{
			ID:        reservationID,
			StoreID:   int64(storeID),
			WeekStart: pgWeek,
			ExpiresAt: timestamptzParam(time.Now().Add(reservationTTL)),
		}); err != nil {
			return err
		}
		reserved = true
		return nil
	})
	if err != nil {
		return 0, 0, false, err
	}
	return reservationID, confirmedBefore, reserved, nil
}

func (uc *TransformUseCase) confirmSlot(ctx context.Context, reservationID int64, storeID identity.StoreID, weekStart string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	pgWeek, err := parseWeekStart(weekStart)
	if err != nil {
		return err
	}

	return uc.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := uc.q(txCtx)
		if err := q.AcquireAiUsageLock(txCtx, int64(storeID)); err != nil {
			return err
		}
		if err := q.DeleteAiUsageReservationByID(txCtx, reservationID); err != nil {
			return err
		}
		return q.IncrementAiUsage(txCtx, gen.IncrementAiUsageParams{
			StoreID: int64(storeID), WeekStart: pgWeek,
		})
	})
}

func (uc *TransformUseCase) releaseSlot(ctx context.Context, reservationID int64, storeID identity.StoreID) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	err := uc.txManager.WithTx(ctx, func(txCtx context.Context) error {
		q := uc.q(txCtx)
		if err := q.AcquireAiUsageLock(txCtx, int64(storeID)); err != nil {
			return err
		}
		return q.DeleteAiUsageReservationByID(txCtx, reservationID)
	})
	if err != nil {
		pkglogger.Error(ctx, "release ai_usage reservation failed", zap.Int64("reservation_id", reservationID), zap.Error(err))
	}
}

type QuotaExceededError struct {
	Usage int
	Limit int
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("本週 AI 美化次數已用完（%d/%d），下週一重置", e.Usage, e.Limit)
}

type geminiRequest struct {
	GenerationConfig geminiGenConfig `json:"generationConfig"`
	Contents         []geminiContent `json:"contents"`
}

type geminiGenConfig struct {
	ResponseModalities []string `json:"responseModalities,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	InlineData *geminiImageData `json:"inlineData,omitempty"`
	Text       string           `json:"text,omitempty"`
}

type geminiImageData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type geminiResponse struct {
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`

	PromptFeedback *struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback,omitempty"`
	Candidates []struct {
		FinishReason string `json:"finishReason,omitempty"`
		Content      struct {
			Parts []struct {
				InlineData *struct {
					MimeType string `json:"mimeType"`
					Data     string `json:"data"`
				} `json:"inlineData,omitempty"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

var geminiHTTPClient = httpclient.New(
	httpclient.WithTimeout(60*time.Second),
	httpclient.WithCheckRedirect(func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}),
)

func wrapPrompt(userPrompt string) string {
	return fmt.Sprintf("你是一個專業的圖片修改 AI。使用者提供了一張圖片，請根據以下描述對圖片進行視覺修改，並回傳修改後的圖片。若描述較抽象或為問句，請依描述的精神做出最合理的視覺調整。\n\n使用者的描述：%s", userPrompt)
}

func transformOutcome(err error) string {
	if err == nil {
		return "success"
	}
	var refusal *domain.ProviderRefusalError
	if errors.As(err, &refusal) {
		return refusal.Reason()
	}
	return "unavailable"
}

func (uc *TransformUseCase) callGeminiWithRetry(ctx context.Context, imageData []byte, mimeType, prompt string) (string, string, error) {
	b64, mime, err := uc.callGemini(ctx, imageData, mimeType, prompt)
	if err == nil {
		return b64, mime, nil
	}

	var refusal *domain.ProviderRefusalError
	if !errors.As(err, &refusal) || !refusal.Retryable() {
		return "", "", err
	}

	reason := refusal.Reason()
	pkglogger.Info(ctx, "retrying gemini transform after non-deterministic refusal",
		zap.String("reason", reason))

	b64, mime, retryErr := uc.callGemini(ctx, imageData, mimeType, prompt)
	observability.RecordAITransformRetry(ctx, reason, transformOutcome(retryErr))
	if retryErr != nil {
		return "", "", retryErr
	}
	return b64, mime, nil
}

func (uc *TransformUseCase) callGemini(ctx context.Context, imageData []byte, mimeType, prompt string) (string, string, error) {
	reqBody := geminiRequest{
		Contents: []geminiContent{{
			Parts: []geminiPart{
				{Text: wrapPrompt(prompt)},
				{InlineData: &geminiImageData{
					MimeType: mimeType,
					Data:     base64.StdEncoding.EncodeToString(imageData),
				}},
			},
		}},
		GenerationConfig: geminiGenConfig{
			ResponseModalities: []string{"IMAGE"},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uc.apiURL, bytes.NewReader(body))
	if err != nil {

		return "", "", &domain.ProviderUnavailableError{Err: fmt.Errorf("建立 Gemini API 請求失敗: %w", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", uc.apiKey)

	resp, err := geminiHTTPClient.Do(req)
	if err != nil {
		return "", "", &domain.ProviderUnavailableError{Err: fmt.Errorf("呼叫 Gemini API 失敗: %w", err)}
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", &domain.ProviderUnavailableError{Err: fmt.Errorf("讀取 Gemini 回應失敗: %w", err)}
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return "", "", &domain.ProviderUnavailableError{Err: fmt.Errorf("解析 Gemini 回應失敗: %w", err)}
	}

	if geminiResp.Error != nil {

		return "", "", &domain.ProviderUnavailableError{Err: fmt.Errorf("gemini 錯誤: %s", geminiResp.Error.Message)}
	}

	blockReason := ""
	if geminiResp.PromptFeedback != nil {
		blockReason = geminiResp.PromptFeedback.BlockReason
	}
	finishReason := ""
	if len(geminiResp.Candidates) > 0 {
		finishReason = geminiResp.Candidates[0].FinishReason
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		refusal := &domain.ProviderRefusalError{FinishReason: finishReason, BlockReason: blockReason}
		pkglogger.Warn(ctx, "gemini returned no image",
			zap.String("blockReason", blockReason),
			zap.String("finishReason", finishReason),
			zap.Bool("retryable", refusal.Retryable()))
		return "", "", refusal
	}

	for _, part := range geminiResp.Candidates[0].Content.Parts {
		if part.InlineData != nil {
			return part.InlineData.Data, part.InlineData.MimeType, nil
		}
	}

	if finishReason == "" || finishReason == domain.FinishReasonStop {
		finishReason = domain.FinishReasonNoImage
	}
	refusal := &domain.ProviderRefusalError{FinishReason: finishReason, BlockReason: blockReason}
	pkglogger.Warn(ctx, "gemini returned candidate without image part",
		zap.String("blockReason", blockReason),
		zap.String("finishReason", finishReason),
		zap.Bool("retryable", refusal.Retryable()))
	return "", "", refusal
}
