package usecase

import (
	"context"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

// --- mockTokenRepo ---

type mockTokenRepo struct {
	upsertErr           error
	deactivateErr       error
	activeTokensErr     error
	deactivateByTokenFn func(token string) error
	deactivateByUserFn  func(userID int64, userType string) (int64, error)
	activeTokens        []*domain.DeviceToken
}

func (m *mockTokenRepo) Upsert(_ context.Context, _ int64, _, _, _ string) error {
	return m.upsertErr
}

func (m *mockTokenRepo) Deactivate(_ context.Context, _ int64, _, _ string) error {
	return m.deactivateErr
}

func (m *mockTokenRepo) DeactivateByToken(_ context.Context, token string) error {
	if m.deactivateByTokenFn != nil {
		return m.deactivateByTokenFn(token)
	}
	return nil
}

func (m *mockTokenRepo) DeactivateByUser(_ context.Context, userID int64, userType string) (int64, error) {
	if m.deactivateByUserFn != nil {
		return m.deactivateByUserFn(userID, userType)
	}
	return 0, nil
}

func (m *mockTokenRepo) GetActiveByUser(_ context.Context, _ int64, _ string) ([]*domain.DeviceToken, error) {
	return m.activeTokens, m.activeTokensErr
}

func (m *mockTokenRepo) GetActiveByUserType(_ context.Context, _ string, _, _ int64) ([]*domain.DeviceToken, error) {
	return nil, nil
}

// --- mockPushRepo ---

type mockPushRepo struct {
	existsErr       error
	createErr       error
	createdRecord   *domain.PushNotificationRecord
	existsProcessed bool
	// captured args from the last ExistsProcessed call (assert identity grain)
	gotEventType   string
	gotReferenceID string
	gotUserID      int64
	gotUserType    string
}

func (m *mockPushRepo) ExistsProcessed(_ context.Context, eventType, referenceID string, userID int64, userType string) (bool, error) {
	m.gotEventType, m.gotReferenceID, m.gotUserID, m.gotUserType = eventType, referenceID, userID, userType
	return m.existsProcessed, m.existsErr
}

func (m *mockPushRepo) Create(_ context.Context, record *domain.PushNotificationRecord) error {
	m.createdRecord = record
	return m.createErr
}

// --- mockPrefRepo ---

type mockPrefRepo struct {
	getCatErr error
	prefByCat *domain.NotificationPreference
	prefs     []*domain.NotificationPreference
}

func (m *mockPrefRepo) Upsert(_ context.Context, _ int64, _, _ string, _ bool) error { return nil }
func (m *mockPrefRepo) GetByUser(_ context.Context, _ int64, _ string) ([]*domain.NotificationPreference, error) {
	return m.prefs, nil
}
func (m *mockPrefRepo) GetByCategory(_ context.Context, _ int64, _, _ string) (*domain.NotificationPreference, error) {
	return m.prefByCat, m.getCatErr
}

// --- mockNotifier ---

type mockNotifier struct {
	failures map[string]error
	err      error
}

func (m *mockNotifier) Send(_ context.Context, _ []string, _, _, _ string, _ map[string]string) (map[string]error, error) {
	return m.failures, m.err
}

func (m *mockNotifier) SendDataOnly(_ context.Context, _ []string, _ map[string]string) (map[string]error, error) {
	return m.failures, m.err
}

func (m *mockNotifier) SendToTopic(_ context.Context, _, _, _, _ string, _ map[string]string) (string, error) {
	return "test-msg-id", m.err
}
