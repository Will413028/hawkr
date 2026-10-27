package usecase_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
)

type fakeCategoryMappingRepo struct {
	getFn                 func(ctx context.Context, categoryID string) (*domain.CategoryMapping, error)
	activeGroupNameFn     func(ctx context.Context, groupCode string) (*string, error)
	activeCommodityNameFn func(ctx context.Context, commodityID int64) (*string, error)
	upsertFn              func(ctx context.Context, m domain.CategoryMapping) (*domain.CategoryMapping, error)
	listByStatusesFn      func(ctx context.Context, statuses []domain.CategoryMappingStatus) ([]domain.CategoryMapping, error)
	listAllCategoryIDsFn  func(ctx context.Context) ([]string, error)
	varieties             map[string][]domain.CropVariety
}

func (f *fakeCategoryMappingRepo) Get(ctx context.Context, categoryID string) (*domain.CategoryMapping, error) {
	return f.getFn(ctx, categoryID)
}
func (f *fakeCategoryMappingRepo) ActiveGroupName(ctx context.Context, groupCode string) (*string, error) {
	return f.activeGroupNameFn(ctx, groupCode)
}
func (f *fakeCategoryMappingRepo) ActiveCommodityName(ctx context.Context, commodityID int64) (*string, error) {
	if f.activeCommodityNameFn == nil {
		return nil, nil
	}
	return f.activeCommodityNameFn(ctx, commodityID)
}
func (f *fakeCategoryMappingRepo) Upsert(ctx context.Context, m domain.CategoryMapping) (*domain.CategoryMapping, error) {
	if f.upsertFn != nil {
		return f.upsertFn(ctx, m)
	}
	return &m, nil // 預設行為：原樣回傳輸入值，等同「寫入成功」，不需要每個測試都填 upsertFn
}
func (f *fakeCategoryMappingRepo) ListByStatuses(ctx context.Context, statuses []domain.CategoryMappingStatus) ([]domain.CategoryMapping, error) {
	return f.listByStatusesFn(ctx, statuses)
}
func (f *fakeCategoryMappingRepo) ListAllCategoryIDs(ctx context.Context) ([]string, error) {
	return f.listAllCategoryIDsFn(ctx)
}
func (f *fakeCategoryMappingRepo) ListResolvedCategoryGroupMappings(ctx context.Context) ([]domain.CategoryGroupMapping, error) {
	return nil, nil
}
func (f *fakeCategoryMappingRepo) ListResolvedCategoryCommodityMappings(ctx context.Context) ([]domain.CategoryCommodityMapping, error) {
	return nil, nil
}

func (f *fakeCategoryMappingRepo) ListResolvedCategoryCropVarietyMappings(ctx context.Context) ([]domain.CategoryCropVarietyMapping, error) {
	return nil, nil
}

func (f *fakeCategoryMappingRepo) ListCategoryCropVarieties(ctx context.Context, categoryID string) ([]domain.CropVariety, error) {
	return f.varieties[categoryID], nil
}

func TestResolveCategoryMapping_NoRow_ReturnsUnmapped(t *testing.T) {
	repo := &fakeCategoryMappingRepo{
		getFn: func(ctx context.Context, categoryID string) (*domain.CategoryMapping, error) {
			return nil, nil
		},
	}
	u := usecase.NewResolveCategoryMapping(repo)
	got, err := u.Get(context.Background(), "cat-unknown")
	require.NoError(t, err)
	require.Equal(t, domain.CategoryMappingUnmapped, got.Status)
	require.Nil(t, got.GroupCode)
}

func TestResolveCategoryMapping_ResolvedButGroupInactive_DegradesToUnmapped(t *testing.T) {
	groupCode := "group-stale"
	repo := &fakeCategoryMappingRepo{
		getFn: func(ctx context.Context, categoryID string) (*domain.CategoryMapping, error) {
			return &domain.CategoryMapping{
				CategoryID: categoryID, Status: domain.CategoryMappingResolved,
				GroupCode: &groupCode, MappingVersion: "pricebook-category-v1",
			}, nil
		},
		activeGroupNameFn: func(ctx context.Context, code string) (*string, error) {
			return nil, nil // 已停用／不存在
		},
	}
	u := usecase.NewResolveCategoryMapping(repo)
	got, err := u.Get(context.Background(), "cat-1")
	require.NoError(t, err)
	require.Equal(t, domain.CategoryMappingUnmapped, got.Status, "目標 group 已停用時必須降級為 unmapped，不能回傳 stale 值")
	require.Nil(t, got.GroupCode)
}

func TestResolveCategoryMapping_ExplicitUnmappedRow_ReturnsAsIs(t *testing.T) {
	repo := &fakeCategoryMappingRepo{
		getFn: func(ctx context.Context, categoryID string) (*domain.CategoryMapping, error) {
			return &domain.CategoryMapping{
				CategoryID: categoryID, Status: domain.CategoryMappingUnmapped, MappingVersion: "pricebook-category-v1",
			}, nil
		},
	}
	u := usecase.NewResolveCategoryMapping(repo)
	got, err := u.Get(context.Background(), "cat-explicit-unmapped")
	require.NoError(t, err)
	require.Equal(t, domain.CategoryMappingUnmapped, got.Status)
	require.Nil(t, got.GroupCode)
}

func TestResolveCategoryMapping_Ambiguous_ReturnsAsIs(t *testing.T) {
	repo := &fakeCategoryMappingRepo{
		getFn: func(ctx context.Context, categoryID string) (*domain.CategoryMapping, error) {
			return &domain.CategoryMapping{
				CategoryID: categoryID, Status: domain.CategoryMappingAmbiguous, MappingVersion: "pricebook-category-v1",
			}, nil
		},
	}
	u := usecase.NewResolveCategoryMapping(repo)
	got, err := u.Get(context.Background(), "cat-ambiguous")
	require.NoError(t, err)
	require.Equal(t, domain.CategoryMappingAmbiguous, got.Status)
	require.Nil(t, got.GroupCode)
}

func TestResolveCategoryMapping_ResolvedActiveGroup_ReturnsGroupSelection(t *testing.T) {
	groupCode := "group-veg"
	groupName := "測試大類"
	repo := &fakeCategoryMappingRepo{
		getFn: func(ctx context.Context, categoryID string) (*domain.CategoryMapping, error) {
			return &domain.CategoryMapping{
				CategoryID: categoryID, Status: domain.CategoryMappingResolved,
				GroupCode: &groupCode, MappingVersion: "pricebook-category-v1",
			}, nil
		},
		activeGroupNameFn: func(ctx context.Context, code string) (*string, error) {
			return &groupName, nil
		},
	}
	u := usecase.NewResolveCategoryMapping(repo)
	got, err := u.Get(context.Background(), "cat-group")
	require.NoError(t, err)
	require.Equal(t, domain.CategoryMappingResolved, got.Status)
	require.NotNil(t, got.GroupCode)
	require.Equal(t, groupCode, *got.GroupCode)
	require.NotNil(t, got.Label)
	require.Equal(t, groupName, *got.Label)
}

func TestResolveCategoryMapping_ResolvedCommodityRow_DoesNotPanic(t *testing.T) {
	repo := &fakeCategoryMappingRepo{
		getFn: func(ctx context.Context, categoryID string) (*domain.CategoryMapping, error) {
			commodityID := int64(958000000000000001)
			return &domain.CategoryMapping{
				CategoryID: categoryID, Status: domain.CategoryMappingResolved,
				GroupCode: nil, CommodityID: &commodityID,
				MappingVersion: usecase.CategoryMappingVersion,
			}, nil
		},
		activeGroupNameFn: func(ctx context.Context, groupCode string) (*string, error) {
			t.Fatalf("commodity 映射不該去查 group（groupCode=%q）", groupCode)
			return nil, nil
		},
		activeCommodityNameFn: func(ctx context.Context, commodityID int64) (*string, error) {
			name := "毛豬（規格豬）"
			return &name, nil
		},
	}

	got, err := usecase.NewResolveCategoryMapping(repo).Get(context.Background(), "2008")
	require.NoError(t, err)
	require.Equal(t, domain.CategoryMappingResolved, got.Status)
	require.Nil(t, got.GroupCode)
	require.NotNil(t, got.CommodityID)
	require.NotNil(t, got.Label)
	require.Equal(t, "毛豬（規格豬）", *got.Label)
}

func TestResolveCategoryMapping_ResolvedButCommodityInactive_DegradesToUnmapped(t *testing.T) {
	commodityID := int64(958000000000000002)
	repo := &fakeCategoryMappingRepo{
		getFn: func(ctx context.Context, categoryID string) (*domain.CategoryMapping, error) {
			return &domain.CategoryMapping{
				CategoryID: categoryID, Status: domain.CategoryMappingResolved,
				CommodityID: &commodityID, MappingVersion: usecase.CategoryMappingVersion,
			}, nil
		},
		activeCommodityNameFn: func(ctx context.Context, id int64) (*string, error) { return nil, nil },
	}

	got, err := usecase.NewResolveCategoryMapping(repo).Get(context.Background(), "2008")
	require.NoError(t, err)
	require.Equal(t, domain.CategoryMappingUnmapped, got.Status)
	require.Nil(t, got.CommodityID)
}
