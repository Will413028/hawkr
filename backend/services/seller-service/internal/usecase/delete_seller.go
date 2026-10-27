package usecase

import (
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"go.uber.org/zap"
)

// DeleteSellerUseCase handles seller deletion.
type DeleteSellerUseCase struct {
	sellerRepo domain.SellerRepository
	logger     *zap.Logger
}

// NewDeleteSellerUseCase creates a new DeleteSellerUseCase.
func NewDeleteSellerUseCase(
	sellerRepo domain.SellerRepository,
	logger *zap.Logger,
) *DeleteSellerUseCase {
	return &DeleteSellerUseCase{
		sellerRepo: sellerRepo,
		logger:     logger,
	}
}
