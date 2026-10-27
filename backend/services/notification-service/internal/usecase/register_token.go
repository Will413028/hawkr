package usecase

import (
	"context"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/usecase/dto"
)

// RegisterTokenUseCase handles device token registration.
type RegisterTokenUseCase struct {
	tokenRepo domain.DeviceTokenRepository
}

// NewRegisterTokenUseCase creates a RegisterTokenUseCase.
func NewRegisterTokenUseCase(tokenRepo domain.DeviceTokenRepository) *RegisterTokenUseCase {
	return &RegisterTokenUseCase{tokenRepo: tokenRepo}
}

// Execute registers or refreshes a device token.
func (u *RegisterTokenUseCase) Execute(ctx context.Context, req *dto.RegisterDeviceTokenRequest) error {
	if err := u.tokenRepo.Upsert(ctx, req.UserID, req.UserType, req.Token, req.Platform); err != nil {
		return err
	}
	logger.Info(ctx, "device token registered",
		zap.Int64("user_id", req.UserID),
		zap.String("user_type", req.UserType),
		zap.String("platform", req.Platform),
	)
	return nil
}
