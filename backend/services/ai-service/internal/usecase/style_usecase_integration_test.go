//go:build integration

package usecase

import (
	"context"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/domain"
	"github.com/Will413028/hawkr/ai-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/ai-service/internal/usecase/dto"
	"github.com/stretchr/testify/require"
)

// TestUpdate_SettingDefaultInOneCategory_DoesNotClearAnother is the regression
// test for the bug fixed by this task: idx_ai_styles_default is a per-category
// unique index (category) WHERE is_default = true, but the pre-fix
// ClearAllDefaultStyles query cleared is_default globally (WHERE is_default =
// true, no category filter). Setting a new default in one category silently
// wiped out the other category's default.
//
// 🔴 A unit test with a fake store cannot catch this: the fake never reaches
// the driver, so it can't see the UPDATE's WHERE scope — only a real database
// distinguishes "cleared this category" from "cleared everything".
//
// The migration seed already provides one default row per category (see the
// NOTE atop style_usecase_category_test.go), so this test locates the
// existing seeded post-category default instead of trying to INSERT a second
// one under idx_ai_styles_default — same adaptation style_usecase_category_test.go
// already uses for TestStyle_Update_IsDefaultTrue_ClearsOtherDefaults.
func TestUpdate_SettingDefaultInOneCategory_DoesNotClearAnother(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	q := gen.New(pool)

	postRows, err := q.ListAiStyles(ctx, domain.CategoryPost)
	require.NoError(t, err)
	var postDefaultID int64
	for _, r := range postRows {
		if r.IsDefault {
			postDefaultID = r.ID
			break
		}
	}
	require.NotZero(t, postDefaultID, "seed fixture must already have a post-category default")

	_, err = q.CreateAiStyle(ctx, gen.CreateAiStyleParams{
		ID: 601, Name: "productA", Label: "A", Prompt: "a", Category: domain.CategoryProduct,
		SortOrder: 1, Enabled: true, IsDefault: false,
	})
	require.NoError(t, err)
	_, err = q.CreateAiStyle(ctx, gen.CreateAiStyleParams{
		ID: 602, Name: "productB", Label: "B", Prompt: "b", Category: domain.CategoryProduct,
		SortOrder: 2, Enabled: true, IsDefault: false,
	})
	require.NoError(t, err)

	uc := NewStyleUseCase(pool)

	// Make productA the product-category default first.
	require.NoError(t, uc.Update(ctx, 601, dto.UpdateStyleRequest{IsDefault: boolPtr(true)}))
	rowA, err := q.GetAiStyleByID(ctx, 601)
	require.NoError(t, err)
	require.True(t, rowA.IsDefault, "productA must be the product-category default after being set")

	// Promoting productB must clear productA (same category) but must NOT
	// touch the post-category default.
	require.NoError(t, uc.Update(ctx, 602, dto.UpdateStyleRequest{IsDefault: boolPtr(true)}))

	rowA, err = q.GetAiStyleByID(ctx, 601)
	require.NoError(t, err)
	require.False(t, rowA.IsDefault, "productA's IsDefault must be cleared: replaced within the same category")

	rowB, err := q.GetAiStyleByID(ctx, 602)
	require.NoError(t, err)
	require.True(t, rowB.IsDefault, "productB must now be the product-category default")

	postDefaultRow, err := q.GetAiStyleByID(ctx, postDefaultID)
	require.NoError(t, err)
	// 🔴 This is the assertion that catches the bug: the pre-fix global clear
	// would flip this to false even though this Update only ever touched
	// product-category rows.
	require.True(t, postDefaultRow.IsDefault, "post-category default must be untouched by a product-category Update")
}

// TestUpdate_NonexistentIDWithIsDefaultTrue_DoesNotClearAnyCategoryDefault is
// the more severe latent case of the same bug: the pre-fix code called
// ClearAllDefaultStyles (or, mid-fix, the naive single-statement
// SetDefaultAiStyleInCategory) unconditionally — INCLUDING when the target id
// does not exist at all — with no transaction wrapping it, so a request for a
// nonexistent id would silently wipe out every category's default and never
// roll back, only failing afterwards with "style not found" once the
// downstream UpdateAiStyle found 0 rows.
//
// 🔴 A unit test cannot catch this either: it needs both a real driver (to
// see the WHERE scope, same as above) and a real transaction (to prove the
// no-op statements against a nonexistent id genuinely commit nothing, rather
// than merely "returning early" in a fake). This is also what exercises the
// affected-check-inside-WithTx fix (style_usecase.go): the not-found error
// must originate from inside the transaction so nothing upstream of it is
// left committed.
func TestUpdate_NonexistentIDWithIsDefaultTrue_DoesNotClearAnyCategoryDefault(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupAiTestPool(ctx, t)
	defer cleanup()

	q := gen.New(pool)

	findDefault := func(category string) int64 {
		rows, err := q.ListAiStyles(ctx, category)
		require.NoError(t, err)
		for _, r := range rows {
			if r.IsDefault {
				return r.ID
			}
		}
		t.Fatalf("seed fixture must already have a %s-category default", category)
		return 0
	}

	productDefaultID := findDefault(domain.CategoryProduct)
	postDefaultID := findDefault(domain.CategoryPost)

	uc := NewStyleUseCase(pool)

	const nonexistentID int64 = 987654321
	err := uc.Update(ctx, nonexistentID, dto.UpdateStyleRequest{IsDefault: boolPtr(true)})
	require.EqualError(t, err, "style not found")

	// 🔴 This is the assertion that catches the bug: both pre-existing
	// category defaults must still be set. The pre-fix code would have
	// cleared both (and committed that clear) before ever discovering the id
	// doesn't exist.
	productRow, err := q.GetAiStyleByID(ctx, productDefaultID)
	require.NoError(t, err)
	require.True(t, productRow.IsDefault, "product-category default must survive an Update on a nonexistent id")

	postRow, err := q.GetAiStyleByID(ctx, postDefaultID)
	require.NoError(t, err)
	require.True(t, postRow.IsDefault, "post-category default must survive an Update on a nonexistent id")
}
