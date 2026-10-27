//go:build integration

package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/ai-service/internal/usecase/dto"
	"github.com/stretchr/testify/require"
)

// NOTE on fixture reality vs. the plan's literal test code: setupAiTestPool replays
// every migration in db/migrations/, including the original 7-style + 1-default
// 'product' seed (20260406200000) and Task 1's 4-style + 1-default 'post' seed
// (20260717053531_add_ai_styles_category.sql). So every test DB here already has
// 8 product rows and 5 post rows before a test inserts anything — the table is
// never empty. The plan's brief asserted exact counts (Len==1, Len==2, Empty)
// assuming a blank table, which cannot hold against this fixture data. The
// assertions below check the same underlying property (category isolation, cache
// isolation) via containment/delta instead of absolute counts.

func styleNames(items []dto.StyleItem) []string {
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Name
	}
	return names
}

func adminStyleNames(items []dto.AdminStyleItem) []string {
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Name
	}
	return names
}

func TestStyle_ListForApp_FiltersByCategory(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	_, err := q.CreateAiStyle(ctx, gen.CreateAiStyleParams{
		ID: 101, Name: "p1", Label: "商品款", Prompt: "product prompt",
		Category: domain.CategoryProduct, Enabled: true, SortOrder: 1,
	})
	require.NoError(t, err)
	_, err = q.CreateAiStyle(ctx, gen.CreateAiStyleParams{
		ID: 102, Name: "post1", Label: "貼文款", Prompt: "post prompt",
		Category: domain.CategoryPost, Enabled: true, SortOrder: 1,
	})
	require.NoError(t, err)

	uc := NewStyleUseCase(pool)

	productResp, err := uc.ListForApp(ctx, 1, domain.CategoryProduct)
	require.NoError(t, err)
	require.Contains(t, styleNames(productResp.Styles), "p1", "product list must contain the newly created product style")
	require.NotContains(t, styleNames(productResp.Styles), "post1", "product list must not leak the post-category style")

	postResp, err := uc.ListForApp(ctx, 1, domain.CategoryPost)
	require.NoError(t, err)
	require.Contains(t, styleNames(postResp.Styles), "post1", "post list must contain the newly created post style")
	require.NotContains(t, styleNames(postResp.Styles), "p1", "post list must not leak the product-category style")
}

func TestStyle_ListForAdmin_OptionalCategoryFilter(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	_, err := q.CreateAiStyle(ctx, gen.CreateAiStyleParams{
		ID: 201, Name: "p1", Label: "商品款", Prompt: "x",
		Category: domain.CategoryProduct, Enabled: true, SortOrder: 1,
	})
	require.NoError(t, err)
	_, err = q.CreateAiStyle(ctx, gen.CreateAiStyleParams{
		ID: 202, Name: "post1", Label: "貼文款", Prompt: "x",
		Category: domain.CategoryPost, Enabled: true, SortOrder: 1,
	})
	require.NoError(t, err)

	uc := NewStyleUseCase(pool)

	all, err := uc.ListForAdmin(ctx, "")
	require.NoError(t, err)
	require.Contains(t, adminStyleNames(all), "p1", "unfiltered admin list must contain the product-category row")
	require.Contains(t, adminStyleNames(all), "post1", "unfiltered admin list must contain the post-category row")

	postOnly, err := uc.ListForAdmin(ctx, domain.CategoryPost)
	require.NoError(t, err)
	require.Contains(t, adminStyleNames(postOnly), "post1")
	require.NotContains(t, adminStyleNames(postOnly), "p1", "category filter must exclude product rows")
	for _, item := range postOnly {
		require.Equal(t, domain.CategoryPost, item.Category, "every row returned for category=post must actually be post")
	}
}

func TestStyle_Create_CachePerCategoryIsolated(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	uc := NewStyleUseCase(pool)

	// Warm both category caches first (seed fixtures mean neither starts empty).
	productRespBefore, err := uc.ListForApp(ctx, 1, domain.CategoryProduct)
	require.NoError(t, err)
	postRespBefore, err := uc.ListForApp(ctx, 1, domain.CategoryPost)
	require.NoError(t, err)

	// Create a POST style; the already-warmed PRODUCT cache must not mask it,
	// and the POST list must reflect it without a stale read.
	require.NoError(t, uc.Create(ctx, 301, dto.CreateStyleRequest{
		Name: "post1", Label: "貼文款", Prompt: "x",
		Category: domain.CategoryPost, SortOrder: "1", Enabled: true,
	}))

	postResp, err := uc.ListForApp(ctx, 1, domain.CategoryPost)
	require.NoError(t, err)
	require.Len(t, postResp.Styles, len(postRespBefore.Styles)+1, "post cache must reflect the new row, not share the already-warmed product cache")
	require.Contains(t, styleNames(postResp.Styles), "post1")

	productRespAfter, err := uc.ListForApp(ctx, 1, domain.CategoryProduct)
	require.NoError(t, err)
	require.Len(t, productRespAfter.Styles, len(productRespBefore.Styles), "product cache must be unaffected by a post-category create")
}

// TestStyle_Create_EmptySortOrder_DefaultsToZero is the actually-reachable
// regression case a first pass at closing the SortOrder-overflow gap (parsing
// via strconv.ParseInt and erroring on ANY parse failure) got wrong: gRPC's
// CreateAiStyleRequest.SortOrder is a plain proto3 string with no has-flag
// (see internal/grpc/handler.go:392, which passes req.SortOrder straight into
// dto.CreateStyleRequest with no validation), so any create-style call that
// doesn't explicitly set sort_order arrives here as "". That must still
// succeed and default to 0, exactly like the pre-migration
// strconv.Atoi(s); _ behavior — it must NOT be treated as a parse error.
func TestStyle_Create_EmptySortOrder_DefaultsToZero(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	uc := NewStyleUseCase(pool)
	require.NoError(t, uc.Create(ctx, 601, dto.CreateStyleRequest{
		Name: "no_sort_order", Label: "L", Prompt: "p",
		Category: domain.CategoryProduct, SortOrder: "", Enabled: true,
	}))

	q := gen.New(pool)
	row, err := q.GetAiStyleByID(ctx, 601)
	require.NoError(t, err)
	require.Equal(t, int32(0), row.SortOrder, "an unset (empty string) sort order must default to 0, not fail the request")
}

// TestStyle_Create_SortOrderOutOfInt32Range_ReturnsErrorAndNoRowCreated is the
// actual overflow gap Finding 3 was about: before this migration, a
// SortOrder that parsed fine as a number but didn't fit in the DB's INTEGER
// column reached Postgres and was rejected there. After the migration, the
// int32(...) conversion happens in Go before the query runs, so this proves
// it now surfaces as an error instead of silently wrapping around.
func TestStyle_Create_SortOrderOutOfInt32Range_ReturnsErrorAndNoRowCreated(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	uc := NewStyleUseCase(pool)
	err := uc.Create(ctx, 602, dto.CreateStyleRequest{
		Name: "overflow", Label: "L", Prompt: "p",
		Category: domain.CategoryProduct, SortOrder: "99999999999", Enabled: true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "sort order out of range")

	q := gen.New(pool)
	_, err = q.GetAiStyleByID(ctx, 602)
	require.Error(t, err, "no row must be created when SortOrder parsing rejects the request")
}

// TestStyle_Update_PartialUpdate_OnlyChangesSetFields exercises the actual
// UPDATE ... SET x = COALESCE(sqlc.narg(x), x) SQL in db/queries/style.sql
// against a real database. This is the query the wave's later tables will
// copy as a template, and it has a quiet failure mode: swapping
// a direct *string parameter (where nil -> NULL and an empty pointed-to string
// remains a valid empty string) for an empty-string-to-NULL helper would silently
// turn "clear this label" into "keep the old
// label" with nothing failing — only a real read-back after a real UPDATE
// catches that.
func TestStyle_Update_PartialUpdate_OnlyChangesSetFields(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	created, err := q.CreateAiStyle(ctx, gen.CreateAiStyleParams{
		ID: 501, Name: "orig", Label: "Original Label", Prompt: "orig prompt",
		ThumbnailKey: "orig-thumb", Category: domain.CategoryProduct,
		SortOrder: 5, Enabled: true, IsDefault: false,
	})
	require.NoError(t, err)

	uc := NewStyleUseCase(pool)
	newLabel := "New Label"
	// A short real gap between the INSERT and UPDATE round trips is insurance
	// against both landing on the same Postgres NOW() for the
	// UpdatedAt-advanced-past-CreatedAt assertion below. NOW() is a DB-side
	// value, not an application clock, so there is nothing to inject here.
	time.Sleep(5 * time.Millisecond)
	require.NoError(t, uc.Update(ctx, 501, dto.UpdateStyleRequest{Label: &newLabel}))

	row, err := q.GetAiStyleByID(ctx, 501)
	require.NoError(t, err)
	require.Equal(t, newLabel, row.Label, "the one field set on the request must be updated")
	require.Equal(t, created.Prompt, row.Prompt, "Prompt must be unchanged")
	require.Equal(t, created.ThumbnailKey, row.ThumbnailKey, "ThumbnailKey must be unchanged")
	require.Equal(t, created.SortOrder, row.SortOrder, "SortOrder must be unchanged")
	require.Equal(t, created.Enabled, row.Enabled, "Enabled must be unchanged")
	require.Equal(t, created.IsDefault, row.IsDefault, "IsDefault must be unchanged")
	require.Equal(t, created.Category, row.Category, "Category must be unchanged")
	require.True(t, row.UpdatedAt.Time.After(row.CreatedAt.Time), "UpdatedAt must advance past CreatedAt after a real UPDATE")
}

// TestStyle_Update_IsDefaultTrue_ClearsOtherDefaults proves
// ClearCategoryDefaultStyle + MarkAiStyleDefault actually run against a real
// database and that their effect is persisted, not just that Update()'s
// early-return lets an IsDefault=true request through (that part is covered
// by the reachability tests in style_usecase_test.go).
func TestStyle_Update_IsDefaultTrue_ClearsOtherDefaults(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	_, err := q.CreateAiStyle(ctx, gen.CreateAiStyleParams{
		ID: 502, Name: "styleA", Label: "A", Prompt: "a", Category: domain.CategoryProduct,
		SortOrder: 1, Enabled: true, IsDefault: false,
	})
	require.NoError(t, err)
	_, err = q.CreateAiStyle(ctx, gen.CreateAiStyleParams{
		ID: 503, Name: "styleB", Label: "B", Prompt: "b", Category: domain.CategoryProduct,
		SortOrder: 2, Enabled: true, IsDefault: false,
	})
	require.NoError(t, err)

	uc := NewStyleUseCase(pool)
	require.NoError(t, uc.Update(ctx, 502, dto.UpdateStyleRequest{IsDefault: boolPtr(true)}))

	rowA, err := q.GetAiStyleByID(ctx, 502)
	require.NoError(t, err)
	require.True(t, rowA.IsDefault, "styleA must be the default after being set")

	// Setting styleB as default must flip styleA back to false in the DB —
	// the real, persisted effect of ClearCategoryDefaultStyle, not just the
	// early-return guard covered elsewhere.
	require.NoError(t, uc.Update(ctx, 503, dto.UpdateStyleRequest{IsDefault: boolPtr(true)}))

	rowA, err = q.GetAiStyleByID(ctx, 502)
	require.NoError(t, err)
	require.False(t, rowA.IsDefault, "styleA's IsDefault must have been cleared by ClearCategoryDefaultStyle")

	rowB, err := q.GetAiStyleByID(ctx, 503)
	require.NoError(t, err)
	require.True(t, rowB.IsDefault, "styleB must now be the default")

	productRows, err := q.ListAiStyles(ctx, domain.CategoryProduct)
	require.NoError(t, err)
	defaultCount := 0
	for _, r := range productRows {
		if r.IsDefault {
			defaultCount++
			require.Equal(t, int64(503), r.ID, "styleB must be the only default row in the product category")
		}
	}
	require.Equal(t, 1, defaultCount, "exactly one product-category row must be default after the second Update")
}

// TestStyle_Delete_RemovesRow proves DeleteAiStyle's SQL actually removes the
// row from a real database.
func TestStyle_Delete_RemovesRow(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	_, err := q.CreateAiStyle(ctx, gen.CreateAiStyleParams{
		ID: 504, Name: "deleteme", Label: "D", Prompt: "d", Category: domain.CategoryProduct,
		SortOrder: 1, Enabled: true, IsDefault: false,
	})
	require.NoError(t, err)

	uc := NewStyleUseCase(pool)
	require.NoError(t, uc.Delete(ctx, 504))

	_, err = q.GetAiStyleByID(ctx, 504)
	require.Error(t, err, "row must be gone after Delete")
}

// TestStyle_Delete_CannotDeleteDefaultStyle proves the "cannot delete default
// style" guard in Delete() and that the row is left untouched when it fires.
func TestStyle_Delete_CannotDeleteDefaultStyle(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	q := gen.New(pool)
	// Created non-default, then promoted via Update — creating with
	// IsDefault: true directly would collide with the seed fixture's existing
	// default row for this category under idx_ai_styles_default.
	_, err := q.CreateAiStyle(ctx, gen.CreateAiStyleParams{
		ID: 505, Name: "defaultstyle", Label: "D", Prompt: "d", Category: domain.CategoryPost,
		SortOrder: 1, Enabled: true, IsDefault: false,
	})
	require.NoError(t, err)

	uc := NewStyleUseCase(pool)
	require.NoError(t, uc.Update(ctx, 505, dto.UpdateStyleRequest{IsDefault: boolPtr(true)}))

	err = uc.Delete(ctx, 505)
	require.EqualError(t, err, "cannot delete default style")

	row, err := q.GetAiStyleByID(ctx, 505)
	require.NoError(t, err, "row must still exist after the guarded delete is rejected")
	require.True(t, row.IsDefault)
}

// TestStyle_Update_NonexistentID_ReturnsNotFound and
// TestStyle_Delete_NonexistentID_ReturnsNotFound check the 0-rows-affected /
// row-not-found paths against a real database, matching the "style not
// found" error-message convention used elsewhere in style_usecase.go.
func TestStyle_Update_NonexistentID_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	uc := NewStyleUseCase(pool)
	newLabel := "won't land anywhere"
	err := uc.Update(ctx, 999999, dto.UpdateStyleRequest{Label: &newLabel})
	require.EqualError(t, err, "style not found")
}

func TestStyle_Delete_NonexistentID_ReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	uc := NewStyleUseCase(pool)
	err := uc.Delete(ctx, 999999)
	require.EqualError(t, err, "style not found")
}
