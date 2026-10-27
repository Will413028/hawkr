package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase/dto"
)

// SendPushUseCase handles sending push notifications.
type SendPushUseCase struct {
	tokenRepo  domain.DeviceTokenRepository
	pushRepo   domain.PushNotificationRepository
	prefRepo   domain.NotificationPreferenceRepository
	notifier   domain.PushNotifier
	deactivate domain.TokenDeactivationPolicy
}

// NewSendPushUseCase creates a SendPushUseCase. deactivate classifies per-token
// FCM errors that mean the token should be deactivated (injected so the usecase
// stays free of FCM/infrastructure deps).
func NewSendPushUseCase(
	tokenRepo domain.DeviceTokenRepository,
	pushRepo domain.PushNotificationRepository,
	prefRepo domain.NotificationPreferenceRepository,
	notifier domain.PushNotifier,
	deactivate domain.TokenDeactivationPolicy,
) *SendPushUseCase {
	return &SendPushUseCase{
		tokenRepo:  tokenRepo,
		pushRepo:   pushRepo,
		prefRepo:   prefRepo,
		notifier:   notifier,
		deactivate: deactivate,
	}
}

// Execute sends a push notification to all active tokens of the target user.
// Idempotent: skips if this notification was already processed for the recipient
// (any prior attempt — sent/partial/skipped/failed), keyed on identity
// (event_type, reference_id, user), not on delivery success.
func (u *SendPushUseCase) Execute(ctx context.Context, req *dto.SendPushRequest) error {
	if req.ReferenceID != "" {
		exists, err := u.pushRepo.ExistsProcessed(ctx, req.EventType, req.ReferenceID, req.UserID, req.UserType)
		if err != nil {
			logger.Warn(ctx, "failed to check idempotency", zap.Error(err))
		} else if exists {
			logger.Info(ctx, "push already processed, skipping",
				zap.String("event_type", req.EventType),
				zap.String("reference_id", req.ReferenceID),
				zap.Int64("user_id", req.UserID),
			)
			return nil
		}
	}

	// Check notification preference using request's Category
	// (sourced from template.Category by caller; empty = fail-open).
	if req.Category != "" {
		pref, err := u.prefRepo.GetByCategory(ctx, req.UserID, req.UserType, req.Category)
		if err != nil {
			logger.Warn(ctx, "failed to check notification preference", zap.Error(err))
		} else if pref != nil && !pref.Enabled {
			logger.Info(ctx, "push skipped: category disabled by user",
				zap.String("event_type", req.EventType),
				zap.String("category", req.Category),
				zap.Int64("user_id", req.UserID),
			)
			return nil
		}
	}

	tokens, err := u.tokenRepo.GetActiveByUser(ctx, req.UserID, req.UserType)
	if err != nil {
		return err
	}

	record := &domain.PushNotificationRecord{
		UserID:      req.UserID,
		UserType:    req.UserType,
		EventType:   req.EventType,
		ReferenceID: req.ReferenceID,
		Title:       req.Title,
		Body:        req.Body,
		Data:        req.Data,
	}

	if len(tokens) == 0 {
		logger.Info(ctx, "no active device tokens, skipping push",
			zap.Int64("user_id", req.UserID),
			zap.String("event_type", req.EventType),
		)
		record.Status = domain.StatusSkipped
		record.ErrorLog = "no active device tokens"
		return u.pushRepo.Create(ctx, record)
	}

	tokenValues := make([]string, len(tokens))
	for i, t := range tokens {
		tokenValues[i] = t.Token
	}

	var failures map[string]error
	if req.Silent {
		failures, err = u.notifier.SendDataOnly(ctx, tokenValues, req.Data)
	} else {
		failures, err = u.notifier.Send(ctx, tokenValues, req.Title, req.Body, req.ImageURL, req.Data)
	}
	if errors.Is(err, domain.ErrPushDisabled) {
		logger.Info(ctx, "push disabled, skipping push",
			zap.String("event_type", req.EventType),
			zap.Bool("silent", req.Silent),
		)
		record.Status = domain.StatusSkipped
		record.ErrorLog = err.Error()
		_ = u.pushRepo.Create(ctx, record)
		return nil
	}
	if err != nil {
		logger.Error(ctx, "FCM send failed", zap.Error(err), zap.String("event_type", req.EventType), zap.Bool("silent", req.Silent))
		record.Status = domain.StatusFailed
		record.ErrorLog = err.Error()
		_ = u.pushRepo.Create(ctx, record)
		return nil // don't propagate; already recorded
	}

	// Deactivate tokens whose FCM error means they are permanently undeliverable.
	for token, ferr := range failures {
		if u.deactivate != nil && u.deactivate(ferr) {
			logger.Info(ctx, "deactivating invalid FCM token", zap.String("token_prefix", token[:min(8, len(token))]))
			_ = u.tokenRepo.DeactivateByToken(ctx, token)
		}
	}

	switch {
	case len(failures) == 0:
		record.Status = domain.StatusSent
	case len(failures) < len(tokenValues):
		record.Status = domain.StatusPartial
	default:
		record.Status = domain.StatusFailed
	}

	// 記錄 per-token 失敗原因（含 partial），否則死 token 造成的長期 partial
	// 在 DB 裡毫無失敗碼可查，問題會悄悄悶燒。
	if len(failures) > 0 {
		record.ErrorLog = summarizeFailures(failures)
	}

	logger.Info(ctx, "push notification sent",
		zap.String("event_type", req.EventType),
		zap.String("status", record.Status),
		zap.Int("total", len(tokenValues)),
		zap.Int("failed", len(failures)),
	)

	return u.pushRepo.Create(ctx, record)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// summarizeFailures 把 per-token 失敗聚合成 error_log 字串：失敗數 + 每個 token
// 前綴 + 失敗訊息。只記前綴（非完整 token）以免洩漏/膨脹。
func summarizeFailures(failures map[string]error) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d token(s) failed:", len(failures))
	for token, ferr := range failures {
		fmt.Fprintf(&b, " %s=%v;", token[:min(8, len(token))], ferr)
	}
	return b.String()
}
