package usecase

import (
	"context"
	"math"
	"testing"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/usecase/dto"
)

func TestSellerUseCase_ListForAdminNormalizesPagination(t *testing.T) {
	tests := []struct {
		name                            string
		page, limit                     int32
		wantPage, wantLimit, wantOffset int32
	}{
		{name: "zero values", page: 0, limit: 0, wantPage: 1, wantLimit: domain.DefaultLimit, wantOffset: 0},
		{name: "negative values", page: -1, limit: -5, wantPage: 1, wantLimit: domain.DefaultLimit, wantOffset: 0},
		{name: "invalid page keeps valid limit", page: 0, limit: 25, wantPage: 1, wantLimit: 25, wantOffset: 0},
		{name: "valid page defaults invalid limit", page: 3, limit: 0, wantPage: 3, wantLimit: domain.DefaultLimit, wantOffset: 20},
		{name: "positive values preserved", page: 3, limit: 9999, wantPage: 3, wantLimit: 9999, wantOffset: 19998},
		{name: "offset overflow falls back to safe defaults", page: 3, limit: math.MaxInt32/2 + 1, wantPage: 1, wantLimit: domain.DefaultLimit, wantOffset: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var listFilter, countFilter domain.SellerListFilter
			repo := &mockSellerRepo{
				ListForAdminFn: func(_ context.Context, filter domain.SellerListFilter) ([]*domain.AdminSellerRow, error) {
					listFilter = filter
					return nil, nil
				},
				CountForAdminFn: func(_ context.Context, filter domain.SellerListFilter) (int64, error) {
					countFilter = filter
					return 0, nil
				},
			}
			uc := &SellerUseCase{sellerRepo: repo}

			got, err := uc.ListForAdmin(context.Background(), &dto.AdminListSellersRequest{
				Page:  tt.page,
				Limit: tt.limit,
			})
			if err != nil {
				t.Fatalf("ListForAdmin() error = %v", err)
			}
			if got.Page != tt.wantPage || got.Limit != tt.wantLimit {
				t.Errorf("response pagination = page %d, limit %d; want page %d, limit %d", got.Page, got.Limit, tt.wantPage, tt.wantLimit)
			}
			if listFilter.Limit != tt.wantLimit || listFilter.Offset != tt.wantOffset {
				t.Errorf("list filter pagination = limit %d, offset %d; want limit %d, offset %d", listFilter.Limit, listFilter.Offset, tt.wantLimit, tt.wantOffset)
			}
			if countFilter.Limit != tt.wantLimit || countFilter.Offset != tt.wantOffset {
				t.Errorf("count filter pagination = limit %d, offset %d; want limit %d, offset %d", countFilter.Limit, countFilter.Offset, tt.wantLimit, tt.wantOffset)
			}
		})
	}
}
