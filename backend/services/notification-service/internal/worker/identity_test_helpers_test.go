package worker

import (
	"context"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
)

// keyedTokenRepo only returns rows for the exact user ID registered in the
// fixture. This makes an identity regression fail instead of accidentally
// passing because the fake returns a token for every user.
type keyedTokenRepo struct {
	tokensByUser map[int64][]*domain.DeviceToken
}

func (r *keyedTokenRepo) Upsert(_ context.Context, _ int64, _, _, _ string) error { return nil }
func (r *keyedTokenRepo) Deactivate(_ context.Context, _ int64, _, _ string) error {
	return nil
}
func (r *keyedTokenRepo) DeactivateByToken(_ context.Context, _ string) error { return nil }
func (r *keyedTokenRepo) DeactivateByUser(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}
func (r *keyedTokenRepo) GetActiveByUser(_ context.Context, userID int64, _ string) ([]*domain.DeviceToken, error) {
	return r.tokensByUser[userID], nil
}
func (r *keyedTokenRepo) GetActiveByUserType(_ context.Context, _ string, _, _ int64) ([]*domain.DeviceToken, error) {
	return nil, nil
}

// keyedPrefRepo mirrors the post-migration preference key: (user_id,
// category), with user_id being the canonical buyer_id or store_id.
type keyedPrefRepo struct {
	prefs map[int64]map[string]*domain.NotificationPreference
}

func (r *keyedPrefRepo) Upsert(_ context.Context, _ int64, _, _ string, _ bool) error {
	return nil
}
func (r *keyedPrefRepo) GetByUser(_ context.Context, _ int64, _ string) ([]*domain.NotificationPreference, error) {
	return nil, nil
}
func (r *keyedPrefRepo) GetByCategory(_ context.Context, userID int64, _, category string) (*domain.NotificationPreference, error) {
	if byCategory, ok := r.prefs[userID]; ok {
		return byCategory[category], nil
	}
	return nil, nil
}

func newKeyedSendPush(tokens *keyedTokenRepo, prefs *keyedPrefRepo) (*usecase.SendPushUseCase, *capturingPushRepo, *capturingNotifier) {
	pushRepo := &capturingPushRepo{}
	notifier := &capturingNotifier{}
	uc := usecase.NewSendPushUseCase(tokens, pushRepo, prefs, notifier, func(error) bool { return false })
	return uc, pushRepo, notifier
}
