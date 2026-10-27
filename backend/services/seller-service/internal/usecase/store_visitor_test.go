package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
)

func TestStoreVisitorUC_RecordVisit_Success(t *testing.T) {
	var called bool
	repo := &mockStoreVisitorRepo{
		UpsertVisitFn: func(ctx context.Context, storeID, visitorBuyerID int64) error {
			called = true
			if storeID != 10 || visitorBuyerID != 20 {
				t.Errorf("got storeID=%d visitorID=%d, want 10, 20", storeID, visitorBuyerID)
			}
			return nil
		},
	}

	uc := NewStoreVisitorUseCase(repo)
	if err := uc.RecordVisit(context.Background(), 10, 20); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("UpsertVisit was not called")
	}
}

func TestStoreVisitorUC_RecordVisit_RepoError(t *testing.T) {
	want := errors.New("db error")
	repo := &mockStoreVisitorRepo{
		UpsertVisitFn: func(ctx context.Context, storeID, visitorBuyerID int64) error {
			return want
		},
	}

	uc := NewStoreVisitorUseCase(repo)
	if err := uc.RecordVisit(context.Background(), 10, 20); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestStoreVisitorUC_GetVisitors_Success(t *testing.T) {
	now := time.Now()
	repo := &mockStoreVisitorRepo{
		CountFn: func(ctx context.Context, storeID int64) (int64, error) {
			return 3, nil
		},
		ListFn: func(ctx context.Context, storeID int64, limit, offset int32) ([]*domain.StoreVisitorRow, error) {
			return []*domain.StoreVisitorRow{
				{VisitorBuyerID: 1, VisitCount: 5, LastVisitedAt: now},
				{VisitorBuyerID: 2, VisitCount: 2, LastVisitedAt: now},
				{VisitorBuyerID: 3, VisitCount: 1, LastVisitedAt: now},
			}, nil
		},
	}

	uc := NewStoreVisitorUseCase(repo)
	result, err := uc.GetVisitors(context.Background(), 99, 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 3 {
		t.Errorf("Total = %d, want 3", result.Total)
	}
	if len(result.Records) != 3 {
		t.Errorf("len(Records) = %d, want 3", len(result.Records))
	}
	if result.Page != 1 {
		t.Errorf("Page = %d, want 1", result.Page)
	}
	if result.PageSize != 20 {
		t.Errorf("PageSize = %d, want 20", result.PageSize)
	}
}

func TestStoreVisitorUC_GetVisitors_PageDefaults(t *testing.T) {
	repo := &mockStoreVisitorRepo{
		CountFn: func(ctx context.Context, storeID int64) (int64, error) {
			return 0, nil
		},
		ListFn: func(ctx context.Context, storeID int64, limit, offset int32) ([]*domain.StoreVisitorRow, error) {
			if limit != 20 {
				t.Errorf("limit = %d, want 20 (default pageSize)", limit)
			}
			if offset != 0 {
				t.Errorf("offset = %d, want 0 (page 1)", offset)
			}
			return nil, nil
		},
	}

	uc := NewStoreVisitorUseCase(repo)
	result, err := uc.GetVisitors(context.Background(), 99, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Page != 1 {
		t.Errorf("Page = %d, want 1 (auto-corrected)", result.Page)
	}
	if result.PageSize != 20 {
		t.Errorf("PageSize = %d, want 20 (auto-corrected)", result.PageSize)
	}
}

func TestStoreVisitorUC_GetVisitors_ClampsAndOffsets(t *testing.T) {
	var gotLimit, gotOffset int32
	repo := &mockStoreVisitorRepo{
		CountFn: func(_ context.Context, _ int64) (int64, error) {
			return 0, nil
		},
		ListFn: func(_ context.Context, _ int64, limit, offset int32) ([]*domain.StoreVisitorRow, error) {
			gotLimit = limit
			gotOffset = offset
			return nil, nil
		},
	}

	result, err := NewStoreVisitorUseCase(repo).GetVisitors(context.Background(), 99, 3, 999)
	if err != nil {
		t.Fatalf("GetVisitors() error = %v", err)
	}
	if gotLimit != domain.MaxPageSize || gotOffset != 2*domain.MaxPageSize {
		t.Errorf("repository pagination = (%d, %d), want (%d, %d)", gotLimit, gotOffset, domain.MaxPageSize, 2*domain.MaxPageSize)
	}
	if result.Page != 3 || result.PageSize != domain.MaxPageSize {
		t.Errorf("response pagination = (%d, %d), want (%d, %d)", result.Page, result.PageSize, 3, domain.MaxPageSize)
	}
}

func TestStoreVisitorUC_GetVisitors_CountError(t *testing.T) {
	want := errors.New("count error")
	repo := &mockStoreVisitorRepo{
		CountFn: func(ctx context.Context, storeID int64) (int64, error) {
			return 0, want
		},
	}

	uc := NewStoreVisitorUseCase(repo)
	_, err := uc.GetVisitors(context.Background(), 99, 1, 20)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
