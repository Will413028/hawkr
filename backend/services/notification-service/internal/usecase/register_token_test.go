package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/notification-service/internal/usecase/dto"
)

func TestRegisterTokenUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	req := &dto.RegisterDeviceTokenRequest{
		UserID:   42,
		UserType: "buyer",
		Token:    "fcm-token-abc",
		Platform: "ios",
	}

	t.Run("success", func(t *testing.T) {
		repo := &mockTokenRepo{}
		uc := NewRegisterTokenUseCase(repo)

		if err := uc.Execute(ctx, req); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("repo error propagated", func(t *testing.T) {
		repoErr := errors.New("db connection lost")
		repo := &mockTokenRepo{upsertErr: repoErr}
		uc := NewRegisterTokenUseCase(repo)

		err := uc.Execute(ctx, req)
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
	})
}
