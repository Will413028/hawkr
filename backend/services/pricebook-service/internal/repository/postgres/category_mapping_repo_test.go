//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	pbpostgres "github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres/gen"
)

func TestCategoryMappingRepo_Get_NoRowReturnsNilNil(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := pbpostgres.NewCategoryMappingRepository(pool, zap.NewNop())
	got, err := repo.Get(ctx, "nonexistent-category-id")
	require.NoError(t, err)
	require.Nil(t, got)
}

func TestCategoryMappingRepo_UpsertThenGet_RoundTrips(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	mustUpsertCropGroupWithCategory(ctx, t, q, "cmr-group-1", "測試大類", "")

	repo := pbpostgres.NewCategoryMappingRepository(pool, zap.NewNop())
	groupCode := "cmr-group-1"
	got, err := repo.Upsert(ctx, domain.CategoryMapping{
		CategoryID:     "cat-1",
		Status:         domain.CategoryMappingResolved,
		GroupCode:      &groupCode,
		MappingVersion: "pricebook-category-v1",
		UpdatedBy:      123,
	})
	require.NoError(t, err)
	require.Equal(t, "cat-1", got.CategoryID)
	require.False(t, got.UpdatedAt.IsZero())

	fetched, err := repo.Get(ctx, "cat-1")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, domain.CategoryMappingResolved, fetched.Status)
	require.Equal(t, "cmr-group-1", *fetched.GroupCode)
}

func TestCategoryMappingRepo_ActiveGroupName_ActiveGroupReturnsName(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	mustUpsertCropGroupWithCategory(ctx, t, q, "cmr-group-active", "測試大類-active", "")

	repo := pbpostgres.NewCategoryMappingRepository(pool, zap.NewNop())
	name, err := repo.ActiveGroupName(ctx, "cmr-group-active")
	require.NoError(t, err)
	require.NotNil(t, name)
	require.Equal(t, "測試大類-active", *name)
}

func TestCategoryMappingRepo_ActiveGroupName_InactiveGroupReturnsNil(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	mustUpsertCropGroupWithCategory(ctx, t, q, "cmr-group-inactive", "測試大類-inactive", "")
	_, err := pool.Exec(ctx, `UPDATE crop_group SET active = false WHERE group_code = $1`, "cmr-group-inactive")
	require.NoError(t, err)

	repo := pbpostgres.NewCategoryMappingRepository(pool, zap.NewNop())
	name, err := repo.ActiveGroupName(ctx, "cmr-group-inactive")
	require.NoError(t, err)
	require.Nil(t, name)
}

// TestCategoryMappingRepo_ListResolvedCategoryGroupMappings_ExcludesInactiveGroup
// covers final whole-branch review finding 1: a resolved category_id whose
// target crop_group has since been deactivated must NOT appear in
// ListResolvedCategoryGroupMappings — the daily rollup batch's L3 input —
// otherwise GET /category-price-rollup would keep reporting a computed price
// for a category that GET /category-resolution (ResolveCategoryMapping,
// via ActiveGroupName above) already fail-closes to unmapped. A resolved
// mapping whose group is still active must still be returned.
func TestCategoryMappingRepo_ListResolvedCategoryGroupMappings_ExcludesInactiveGroup(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	mustUpsertCropGroupWithCategory(ctx, t, q, "cmr-group-lrcgm-active", "測試大類-lrcgm-active", "")
	mustUpsertCropGroupWithCategory(ctx, t, q, "cmr-group-lrcgm-inactive", "測試大類-lrcgm-inactive", "")
	_, err := pool.Exec(ctx, `UPDATE crop_group SET active = false WHERE group_code = $1`, "cmr-group-lrcgm-inactive")
	require.NoError(t, err)

	repo := pbpostgres.NewCategoryMappingRepository(pool, zap.NewNop())
	activeGroup := "cmr-group-lrcgm-active"
	_, err = repo.Upsert(ctx, domain.CategoryMapping{
		CategoryID:     "cat-lrcgm-active",
		Status:         domain.CategoryMappingResolved,
		GroupCode:      &activeGroup,
		MappingVersion: "pricebook-category-v1",
		UpdatedBy:      123,
	})
	require.NoError(t, err)
	inactiveGroup := "cmr-group-lrcgm-inactive"
	_, err = repo.Upsert(ctx, domain.CategoryMapping{
		CategoryID:     "cat-lrcgm-inactive",
		Status:         domain.CategoryMappingResolved,
		GroupCode:      &inactiveGroup,
		MappingVersion: "pricebook-category-v1",
		UpdatedBy:      123,
	})
	require.NoError(t, err)

	got, err := repo.ListResolvedCategoryGroupMappings(ctx)
	require.NoError(t, err)

	// Unbounded read may include rows from other fixtures/migrations (cf.
	// ListByStatuses above) — assert presence/absence of our own rows, not
	// exact length.
	byCategoryID := map[string]string{}
	for _, m := range got {
		byCategoryID[m.CategoryID] = m.GroupCode
	}
	require.Equal(t, "cmr-group-lrcgm-active", byCategoryID["cat-lrcgm-active"],
		"resolved mapping targeting a still-active group must be present")
	_, stillPresent := byCategoryID["cat-lrcgm-inactive"]
	require.False(t, stillPresent,
		"resolved mapping targeting a deactivated group must be excluded, not just left with a stale group_code")
}

func TestCategoryMappingRepo_ListByStatuses_OnlyReturnsMatchingStatus(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	mustUpsertCropGroupWithCategory(ctx, t, q, "cmr-group-list-1", "測試大類-list", "")

	repo := pbpostgres.NewCategoryMappingRepository(pool, zap.NewNop())
	groupCode := "cmr-group-list-1"
	_, err := repo.Upsert(ctx, domain.CategoryMapping{
		CategoryID:     "cat-resolved",
		Status:         domain.CategoryMappingResolved,
		GroupCode:      &groupCode,
		MappingVersion: "pricebook-category-v1",
		UpdatedBy:      123,
	})
	require.NoError(t, err)
	_, err = repo.Upsert(ctx, domain.CategoryMapping{
		CategoryID:     "cat-unmapped",
		Status:         domain.CategoryMappingUnmapped,
		MappingVersion: "pricebook-category-v1",
		UpdatedBy:      123,
	})
	require.NoError(t, err)

	got, err := repo.ListByStatuses(ctx, []domain.CategoryMappingStatus{domain.CategoryMappingUnmapped})
	require.NoError(t, err)

	// Extract category IDs from result; unbounded read may include rows from
	// migrations (cf. dbtest_test.go package comment on bounded vs unbounded reads).
	// Assert our test fixture is present and resolved row is absent.
	categoryIDs := make([]string, len(got))
	for i, row := range got {
		categoryIDs[i] = row.CategoryID
		require.Equal(t, domain.CategoryMappingUnmapped, row.Status, "ListByStatuses(unmapped) must only return unmapped rows")
	}
	require.Contains(t, categoryIDs, "cat-unmapped", "test fixture must be present in unmapped results")
	require.NotContains(t, categoryIDs, "cat-resolved", "resolved row must not appear in unmapped results")
}

func TestCategoryMappingRepo_ListAllCategoryIDs_ContainsUpsertedID(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := pbpostgres.NewCategoryMappingRepository(pool, zap.NewNop())
	_, err := repo.Upsert(ctx, domain.CategoryMapping{
		CategoryID:     "cat-listall",
		Status:         domain.CategoryMappingUnmapped,
		MappingVersion: "pricebook-category-v1",
		UpdatedBy:      123,
	})
	require.NoError(t, err)

	ids, err := repo.ListAllCategoryIDs(ctx)
	require.NoError(t, err)
	require.Contains(t, ids, "cat-listall")
}

func TestCategoryMappingRepo_DBCheckConstraint_RejectsResolvedWithoutGroupCode(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	_, err := pool.Exec(ctx, `
		INSERT INTO category_crop_mapping (category_id, status, mapping_version, updated_by)
		VALUES ('cat-bad', 'resolved', 'pricebook-category-v1', 1)
	`)
	require.Error(t, err, "resolved 狀態下 group_code 為空必須違反 CHECK constraint")
}

func TestCategoryMappingRepo_DBCheckConstraint_RejectsUnmappedWithGroupCode(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	_, err := pool.Exec(ctx, `
		INSERT INTO category_crop_mapping (category_id, status, group_code, mapping_version, updated_by)
		VALUES ('cat-bad-2', 'unmapped', 'some-code', 'pricebook-category-v1', 1)
	`)
	require.Error(t, err, "unmapped 狀態下不該允許帶 group_code")
}

func TestCategoryMappingRepo_ListResolvedCategoryCropVarietyMappings_FoldsAndFiltersInactive(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	groupID := mustUpsertCropGroupWithCategory(ctx, t, q, "cmr-var-group", "測試大類-variety", "")
	mustUpsertCropInGroup(ctx, t, q, groupID, "cmr-var-dom", "國產品種")
	mustUpsertCropInGroup(ctx, t, q, groupID, "cmr-var-imp", "進口品種")
	inactiveID := mustUpsertCropInGroup(ctx, t, q, groupID, "cmr-var-dead", "已停用品種")
	_, err := pool.Exec(ctx, `UPDATE crop SET active = false WHERE id = $1`, inactiveID)
	require.NoError(t, err)

	repo := pbpostgres.NewCategoryMappingRepository(pool, zap.NewNop())
	groupCode := "cmr-var-group"
	for _, categoryID := range []string{"cat-var-narrowed", "cat-var-whole"} {
		_, err := repo.Upsert(ctx, domain.CategoryMapping{
			CategoryID: categoryID, Status: domain.CategoryMappingResolved,
			GroupCode: &groupCode, MappingVersion: "pricebook-category-v1", UpdatedBy: 1,
		})
		require.NoError(t, err)
	}
	for _, cropCode := range []string{"cmr-var-dom", "cmr-var-imp", "cmr-var-dead"} {
		require.NoError(t, q.InsertCategoryCropMappingCrop(ctx, gen.InsertCategoryCropMappingCropParams{
			CategoryID: "cat-var-narrowed", CropCode: cropCode,
		}))
	}

	got, err := repo.ListResolvedCategoryCropVarietyMappings(ctx)
	require.NoError(t, err)

	var narrowed *domain.CategoryCropVarietyMapping
	for i := range got {
		require.NotEqual(t, "cat-var-whole", got[i].CategoryID,
			"沒有收窄列的 categoryId 不得出現在清單裡——出現就代表呼叫端會拿到空集合，"+
				"而空集合在 narrowToCropCodes 是「整個大類」，語意剛好相反但不會報錯")
		if got[i].CategoryID == "cat-var-narrowed" {
			narrowed = &got[i]
		}
	}
	require.NotNil(t, narrowed, "被收窄的 categoryId 必須出現")
	require.Equal(t, []string{"cmr-var-dom", "cmr-var-imp"}, narrowed.CropCodes,
		"三列摺成一筆，且已停用的品種要被濾掉")
}

func TestCategoryMappingRepo_ListCategoryCropVarieties_NoRowsReturnsEmptyNotError(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := pbpostgres.NewCategoryMappingRepository(pool, zap.NewNop())
	got, err := repo.ListCategoryCropVarieties(ctx, "cat-never-narrowed")
	require.NoError(t, err)
	require.Empty(t, got)
}
