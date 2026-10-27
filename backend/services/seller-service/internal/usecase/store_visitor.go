package usecase

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/go-pkg/pagination"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

type StoreVisitorUseCase struct {
	repo domain.StoreVisitorRepository
}

func NewStoreVisitorUseCase(repo domain.StoreVisitorRepository) *StoreVisitorUseCase {
	return &StoreVisitorUseCase{repo: repo}
}

// RecordVisit 記錄一次商店瀏覽（UPSERT，冪等）
func (u *StoreVisitorUseCase) RecordVisit(ctx context.Context, storeID, visitorBuyerID int64) error {
	return u.repo.UpsertVisit(ctx, storeID, visitorBuyerID)
}

// GetVisitors 取得商店訪客列表（分頁）
func (u *StoreVisitorUseCase) GetVisitors(ctx context.Context, storeID int64, page, pageSize int32) (*dto.StoreVisitorList, error) {
	page, pageSize, offset := pagination.NormalizePage(page, pageSize, domain.DefaultPageSize, domain.MaxPageSize)

	total, err := u.repo.Count(ctx, storeID)
	if err != nil {
		return nil, err
	}

	rows, err := u.repo.List(ctx, storeID, pageSize, offset)
	if err != nil {
		return nil, err
	}

	items := make([]dto.StoreVisitorItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.StoreVisitorItem{
			VisitorBuyerID: row.VisitorBuyerID,
			VisitCount:     row.VisitCount,
			LastVisitedAt:  row.LastVisitedAt.Unix(),
		})
	}

	return &dto.StoreVisitorList{
		Records:  items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// calendarWeekBounds 回傳本週日 00:00 UTC 和上週日 00:00 UTC（週日~週六為一週）
func calendarWeekBounds(now time.Time) (thisWeekStart, lastWeekStart time.Time) {
	daysSinceSunday := int(now.Weekday()) // Sunday=0
	thisWeekStart = time.Date(now.Year(), now.Month(), now.Day()-daysSinceSunday, 0, 0, 0, 0, time.UTC)
	lastWeekStart = thisWeekStart.AddDate(0, 0, -7)
	return
}
