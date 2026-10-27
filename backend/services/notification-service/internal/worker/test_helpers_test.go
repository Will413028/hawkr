package worker

import (
	"context"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
)

// capturingPushRepo captures PushNotificationRecords passed to Create.
type capturingPushRepo struct {
	record  *domain.PushNotificationRecord   // last record (backward compat)
	records []*domain.PushNotificationRecord // all records
}

func (r *capturingPushRepo) ExistsProcessed(_ context.Context, _, _ string, _ int64, _ string) (bool, error) {
	return false, nil
}

func (r *capturingPushRepo) Create(_ context.Context, record *domain.PushNotificationRecord) error {
	r.record = record
	r.records = append(r.records, record)
	return nil
}

// alwaysActiveTokenRepo returns one test token for any user.
type alwaysActiveTokenRepo struct{}

func (r *alwaysActiveTokenRepo) Upsert(_ context.Context, _ int64, _, _, _ string) error { return nil }
func (r *alwaysActiveTokenRepo) Deactivate(_ context.Context, _ int64, _, _ string) error {
	return nil
}
func (r *alwaysActiveTokenRepo) DeactivateByToken(_ context.Context, _ string) error { return nil }
func (r *alwaysActiveTokenRepo) DeactivateByUser(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}
func (r *alwaysActiveTokenRepo) GetActiveByUser(_ context.Context, _ int64, _ string) ([]*domain.DeviceToken, error) {
	return []*domain.DeviceToken{{Token: "test-token"}}, nil
}

func (r *alwaysActiveTokenRepo) GetActiveByUserType(_ context.Context, _ string, _, _ int64) ([]*domain.DeviceToken, error) {
	return []*domain.DeviceToken{{Token: "broadcast-token"}}, nil
}

// noopPreferenceRepo always returns nil (all categories enabled by default).
type noopPreferenceRepo struct{}

func (r *noopPreferenceRepo) Upsert(_ context.Context, _ int64, _, _ string, _ bool) error {
	return nil
}
func (r *noopPreferenceRepo) GetByUser(_ context.Context, _ int64, _ string) ([]*domain.NotificationPreference, error) {
	return nil, nil
}
func (r *noopPreferenceRepo) GetByCategory(_ context.Context, _ int64, _, _ string) (*domain.NotificationPreference, error) {
	return nil, nil
}

// alwaysSuccessNotifier always reports success.
type alwaysSuccessNotifier struct{}

func (n *alwaysSuccessNotifier) Send(_ context.Context, _ []string, _, _, _ string, _ map[string]string) (map[string]error, error) {
	return map[string]error{}, nil
}

func (n *alwaysSuccessNotifier) SendDataOnly(_ context.Context, _ []string, _ map[string]string) (map[string]error, error) {
	return map[string]error{}, nil
}

func (n *alwaysSuccessNotifier) SendToTopic(_ context.Context, _, _, _, _ string, _ map[string]string) (string, error) {
	return "test-msg-id", nil
}

// newTestSendPushWithRepo creates a SendPushUseCase with test dependencies
// and returns the capturing push repo for assertions.
func newTestSendPushWithRepo() (*usecase.SendPushUseCase, *capturingPushRepo) {
	pushRepo := &capturingPushRepo{}
	uc := usecase.NewSendPushUseCase(
		&alwaysActiveTokenRepo{},
		pushRepo,
		&noopPreferenceRepo{},
		&alwaysSuccessNotifier{},
		func(error) bool { return false },
	)
	return uc, pushRepo
}

// capturingInboxRepo captures InboxItems passed to Create.
type capturingInboxRepo struct {
	createdItem *domain.InboxItem
}

func (r *capturingInboxRepo) Create(_ context.Context, item *domain.InboxItem) (int64, error) {
	r.createdItem = item
	return 1, nil // 1 row inserted (not deduped)
}
func (r *capturingInboxRepo) ListByUser(_ context.Context, _ int64, _ string, _ int64, _ int32, _ string) ([]*domain.InboxItem, error) {
	return nil, nil
}
func (r *capturingInboxRepo) CountByUser(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}
func (r *capturingInboxRepo) CountUnread(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}
func (r *capturingInboxRepo) MarkAsRead(_ context.Context, _, _ int64, _ string) (int64, error) {
	return 0, nil
}
func (r *capturingInboxRepo) MarkAllAsRead(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}
func (r *capturingInboxRepo) SoftDeleteByIDs(_ context.Context, _ []int64, _ int64, _ string) (int64, error) {
	return 0, nil
}

func (r *capturingInboxRepo) RedactChatBySender(_ context.Context, _ []int64) (int64, error) {
	return 0, nil
}

func (r *capturingInboxRepo) SoftDeleteByHistoricalBuyerIDs(_ context.Context, _ []int64) (int64, error) {
	return 0, nil
}

func (r *capturingInboxRepo) SoftDeleteByHistoricalStoreIDs(_ context.Context, _ []int64) (int64, error) {
	return 0, nil
}

// newTestInboxUC creates a NotificationInboxUseCase with a capturing repo.
// buyerClient/sellerClient are nil — this helper is used only by
// template_handler_test.go tests that never exercise
// RedactChatNotificationsByAccount.
func newTestInboxUC() (*usecase.NotificationInboxUseCase, *capturingInboxRepo) {
	repo := &capturingInboxRepo{}
	uc := usecase.NewNotificationInboxUseCase(repo, nil, nil, nil, nil)
	return uc, repo
}

// noTokenRepo returns no device tokens for any user.
type noTokenRepo struct{}

func (r *noTokenRepo) Upsert(_ context.Context, _ int64, _, _, _ string) error  { return nil }
func (r *noTokenRepo) Deactivate(_ context.Context, _ int64, _, _ string) error { return nil }
func (r *noTokenRepo) DeactivateByToken(_ context.Context, _ string) error      { return nil }
func (r *noTokenRepo) DeactivateByUser(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}
func (r *noTokenRepo) GetActiveByUser(_ context.Context, _ int64, _ string) ([]*domain.DeviceToken, error) {
	return nil, nil
}
func (r *noTokenRepo) GetActiveByUserType(_ context.Context, _ string, _, _ int64) ([]*domain.DeviceToken, error) {
	return nil, nil
}
