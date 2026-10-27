package grpc

import (
	"context"
	"errors"
	"testing"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase"
	"go.uber.org/zap"
)

type mockTokenRepo struct {
	upsertErr      error
	deactivateErr  error
	activeTokenErr error
	activeTokens   []*domain.DeviceToken
}

func (m *mockTokenRepo) Upsert(_ context.Context, _ int64, _, _, _ string) error {
	return m.upsertErr
}
func (m *mockTokenRepo) Deactivate(_ context.Context, _ int64, _, _ string) error {
	return m.deactivateErr
}
func (m *mockTokenRepo) DeactivateByToken(_ context.Context, _ string) error { return nil }
func (m *mockTokenRepo) DeactivateByUser(_ context.Context, _ int64, _ string) (int64, error) {
	return 0, nil
}
func (m *mockTokenRepo) GetActiveByUser(_ context.Context, _ int64, _ string) ([]*domain.DeviceToken, error) {
	return m.activeTokens, m.activeTokenErr
}
func (m *mockTokenRepo) GetActiveByUserType(_ context.Context, _ string, _, _ int64) ([]*domain.DeviceToken, error) {
	return nil, nil
}

// --- tests ---

func TestRegisterDeviceToken_Success(t *testing.T) {
	repo := &mockTokenRepo{}
	s := &Server{
		registerUC: usecase.NewRegisterTokenUseCase(repo),
		logger:     zap.NewNop(),
	}

	resp, err := s.RegisterDeviceToken(context.Background(), &pb.RegisterDeviceTokenRequest{
		UserId:   "1",
		UserType: "buyer",
		Token:    "fcm-token-123",
		Platform: "ios",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d: %s", resp.Code, resp.Message)
	}
}

func TestRegisterDeviceToken_InvalidPlatform(t *testing.T) {
	s := &Server{logger: zap.NewNop()}

	resp, err := s.RegisterDeviceToken(context.Background(), &pb.RegisterDeviceTokenRequest{
		UserId:   "1",
		UserType: "buyer",
		Token:    "fcm-token-123",
		Platform: "windows",
	})
	if err != nil {
		t.Fatalf("unexpected gRPC error: %v", err)
	}
	if resp.Code != int32(pkgerrors.CodeInvalidInput) {
		t.Fatalf("expected resp.Code=%d, got %d", pkgerrors.CodeInvalidInput, resp.Code)
	}
}

func TestRegisterDeviceToken_RepoError(t *testing.T) {
	repo := &mockTokenRepo{upsertErr: errors.New("db error")}
	s := &Server{
		registerUC: usecase.NewRegisterTokenUseCase(repo),
		logger:     zap.NewNop(),
	}

	_, err := s.RegisterDeviceToken(context.Background(), &pb.RegisterDeviceTokenRequest{
		UserId:   "1",
		UserType: "buyer",
		Token:    "fcm-token-123",
		Platform: "ios",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRemoveDeviceToken_Success(t *testing.T) {
	repo := &mockTokenRepo{}
	s := &Server{
		removeUC: usecase.NewRemoveTokenUseCase(repo),
		logger:   zap.NewNop(),
	}

	resp, err := s.RemoveDeviceToken(context.Background(), &pb.RemoveDeviceTokenRequest{
		UserId:   "1",
		UserType: "buyer",
		Token:    "fcm-token-123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d: %s", resp.Code, resp.Message)
	}
}

func TestRemoveDeviceToken_RepoError(t *testing.T) {
	repo := &mockTokenRepo{deactivateErr: errors.New("db error")}
	s := &Server{
		removeUC: usecase.NewRemoveTokenUseCase(repo),
		logger:   zap.NewNop(),
	}

	_, err := s.RemoveDeviceToken(context.Background(), &pb.RemoveDeviceTokenRequest{
		UserId:   "1",
		UserType: "buyer",
		Token:    "fcm-token-123",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
