package usecase

import (
	"context"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase/dto"
)

// RemoveTokenUseCase handles device token removal (logout).
type RemoveTokenUseCase struct {
	tokenRepo domain.DeviceTokenRepository
}

// NewRemoveTokenUseCase creates a RemoveTokenUseCase.
func NewRemoveTokenUseCase(tokenRepo domain.DeviceTokenRepository) *RemoveTokenUseCase {
	return &RemoveTokenUseCase{tokenRepo: tokenRepo}
}

// Execute deactivates a device token (idempotent).
func (u *RemoveTokenUseCase) Execute(ctx context.Context, req *dto.RemoveDeviceTokenRequest) error {
	if err := u.tokenRepo.Deactivate(ctx, req.UserID, req.UserType, req.Token); err != nil {
		return err
	}
	logger.Info(ctx, "device token deactivated",
		zap.Int64("user_id", req.UserID),
		zap.String("user_type", req.UserType),
	)
	return nil
}
