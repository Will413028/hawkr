package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/Will413028/hawkr/notification-service/internal/usecase/dto"
)

func TestRemoveTokenUseCase_Execute(t *testing.T) {
	ctx := context.Background()

	req := &dto.RemoveDeviceTokenRequest{
		UserID:   99,
		UserType: "seller",
		Token:    "fcm-token-xyz",
	}

	t.Run("success", func(t *testing.T) {
		repo := &mockTokenRepo{}
		uc := NewRemoveTokenUseCase(repo)

		if err := uc.Execute(ctx, req); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("repo error propagated", func(t *testing.T) {
		repoErr := errors.New("deactivate failed")
		repo := &mockTokenRepo{deactivateErr: repoErr}
		uc := NewRemoveTokenUseCase(repo)

		err := uc.Execute(ctx, req)
		if !errors.Is(err, repoErr) {
			t.Fatalf("expected %v, got %v", repoErr, err)
		}
	})
}
