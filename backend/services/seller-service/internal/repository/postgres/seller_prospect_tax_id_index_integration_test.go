//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

// Final whole-branch review, item 2: uq_seller_prospects_tax_id used to be
// `WHERE tax_id IS NOT NULL` with no status predicate, so an ARCHIVED row —
// whether archived directly (SetArchived) or as a merge's loser — could
// permanently occupy the index, and any later ACTIVE row wanting that same
// tax_id hit a real, unrecoverable 23505. The fix narrows the predicate to
// `tax_id IS NOT NULL AND status <> 'ARCHIVED'`. These tests prove both
// halves of that change against a real Postgres, not a mock:
//   - T1 (one tax id per *live* prospect) still holds between two ACTIVE
//     rows.
//   - An ARCHIVED row no longer blocks the same tax id from being reused by
//     an unrelated new prospect, both when the row was archived directly
//     (tax id kept on the row, freed by the narrowed predicate) and when it
//     was a merge's loser (tax id cleared outright by the merge itself).

func TestUqSellerProspectsTaxID_StillRejectsTwoActiveRows(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	taxID := "11171177"
	first := newTestSellerProspect("先建檔業者", "水果", "台南")
	first.TaxID = &taxID
	_, err := repo.Create(ctx, first)
	require.NoError(t, err)

	second := newTestSellerProspect("後建檔業者", "水果", "台南")
	second.TaxID = &taxID
	_, err = repo.Create(ctx, second)

	require.Error(t, err, "兩筆 ACTIVE 業者不可共用同一個統編——narrow 過的 index 沒有放寬這條")
	var dupErr *domain.ErrDuplicate
	require.True(t, errors.As(err, &dupErr),
		"error must be a *domain.ErrDuplicate (repository must translate the real 23505), got %T: %v", err, err)
	require.Equal(t, "uq_seller_prospects_tax_id", dupErr.ConstraintName)
}

// 這支測試原本示範的是舊規則：合併不搬 tax_id，敗方封存後仍握著統編，
// admin 得自己動手把它補到勝方身上，而 narrow 過的 index 不會擋住這個
// 手動步驟。新規則把這個手動步驟收進合併交易本身——勝方沒有統編、敗方
// 有時，合併當下就直接搬過去，敗方的則清空，不再需要 admin 事後再補一
// 次 Update。這支測試改成直接驗證新規則（也是 spec §5.2 tax_id 例外的
// 唯一理由：narrow 過的 index 仍然管兩個 ACTIVE 列，「先補值再合併」對
// tax_id 走不通，只有合併交易內部能安全搬動）。
func TestUqSellerProspectsTaxID_MergeCarriesLoserTaxIDToWinnerWhenWinnerHasNone(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	taxID := "22233344"
	winner, err := repo.Create(ctx, newTestSellerProspect("贏家業者", "水果", "台南"))
	require.NoError(t, err)
	loser := newTestSellerProspect("輸家業者", "水果", "台南")
	loser.TaxID = &taxID
	loser, err = repo.Create(ctx, loser)
	require.NoError(t, err)

	got, err := repo.Merge(ctx, winner.ID, loser.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, got.TaxID, "勝方原本沒有統編，合併必須把敗方的搬過去")
	require.Equal(t, taxID, *got.TaxID)

	archivedLoser, err := repo.GetByID(ctx, loser.ID)
	require.NoError(t, err)
	require.Equal(t, "ARCHIVED", archivedLoser.Status, "敗方合併後必須是 ARCHIVED")
	require.Nil(t, archivedLoser.TaxID, "統編搬到勝方之後，敗方自己的必須清空——不留下歷史記錄，否則會變成日後無法還原的死路")
}

// review 原本描述的情境（合併把敗方封存、敗方的統編仍留著、之後隨便一筆
// 全新業者都能沿用）已經被新規則取代：合併現在會直接清空敗方的統編，
// 不必等它被別人「補領」才算釋出（見上面那支測試）。這支測試改成驗證
// 新規則下的等價不變式：兩邊原本各自持有不同統編時，合併只清空敗方
// 的，不動勝方的，而敗方原本那個號碼合併後立刻對任何一筆全新、與這次
// 合併無關的第三方業者開放——不是只有合併的勝方才能受益於這次釋出。
func TestUqSellerProspectsTaxID_UnrelatedNewRowCanReuseLoserTaxIDFreedByMerge(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	winnerTaxID := "33344455"
	loserTaxID := "57555577"
	winner := newTestSellerProspect("贏家業者", "水果", "台南")
	winner.TaxID = &winnerTaxID
	winner, err := repo.Create(ctx, winner)
	require.NoError(t, err)
	loser := newTestSellerProspect("輸家業者", "水果", "台南")
	loser.TaxID = &loserTaxID
	loser, err = repo.Create(ctx, loser)
	require.NoError(t, err)

	_, err = repo.Merge(ctx, winner.ID, loser.ID, nil)
	require.NoError(t, err)

	fresh := newTestSellerProspect("全新第三方業者", "水果", "台南")
	fresh.TaxID = &loserTaxID
	_, err = repo.Create(ctx, fresh)
	require.NoError(t, err,
		"合併清空的是敗方自己的統編，不是勝方的——這個號碼合併後必須立刻對任何第三方開放")
}

// Closing-fix review item 1: narrow 過的 index 讓「封存」不再永久佔住
// 統編，這是好事，但反過來也代表「還原」現在可能撞回一個真正的、無法
// 復原的 23505——封存 A（持有統編 T）期間，B 合法拿到了同一個 T（上面
// 兩支測試已證明這是允許的），此時把 A 還原成 ACTIVE 必然違反
// uq_seller_prospects_tax_id（兩列都會是 ACTIVE 且共用 T）。這支測試證明
// repository.SetArchived 有把這個真實的資料庫層碰撞包成
// *domain.ErrDuplicate，而不是讓裸的 *pgconn.PgError 原樣往上傳
// （ErrorInterceptor 會把後者在 production 遮蔽成「內部錯誤」，在 dev
// 洩漏 constraint name——即這一輪要修的那個回歸）。
func TestUqSellerProspectsTaxID_RestoringArchivedRowCollidesWithNewOwner(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	taxID := "44455566"
	a := newTestSellerProspect("A業者", "水果", "台南")
	a.TaxID = &taxID
	a, err := repo.Create(ctx, a)
	require.NoError(t, err)

	_, err = repo.SetArchived(ctx, a.ID, true, nil)
	require.NoError(t, err, "A 先封存，讓統編從 index 的管轄範圍釋出")

	b := newTestSellerProspect("B業者", "水果", "台南")
	b.TaxID = &taxID
	_, err = repo.Create(ctx, b)
	require.NoError(t, err, "narrow 過的 index 允許 B 合法拿走 A 封存期間釋出的統編")

	_, err = repo.SetArchived(ctx, a.ID, false, nil)
	require.Error(t, err, "還原 A 會讓兩列同時 ACTIVE 且共用同一個統編，真實違反 uq_seller_prospects_tax_id")
	var dupErr *domain.ErrDuplicate
	require.True(t, errors.As(err, &dupErr),
		"error must be a *domain.ErrDuplicate (SetArchived must translate the real 23505), got %T: %v", err, err)
	require.Equal(t, "uq_seller_prospects_tax_id", dupErr.ConstraintName)
}

// Closing-fix review item 2: 上面的測試已證明一個統編可以同時被一筆
// ARCHIVED 與一筆 ACTIVE 列持有。FindByTaxID 是 usecase.Match 的 T1，T1
// 命中即短路——若 FindByTaxID 撈到的是已封存的舊列，真正存活的 ACTIVE
// 列就永遠不會被看到（也不會落回 T2）。這支測試直接對真實 Postgres 驗證
// FindByTaxID 回的是 ACTIVE 那筆，不是 ARCHIVED 的舊列。
func TestFindByTaxID_ArchivedRowDoesNotShadowActiveRow(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()
	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	taxID := "88188818"
	archived := newTestSellerProspect("已封存業者", "水果", "台南")
	archived.TaxID = &taxID
	archived, err := repo.Create(ctx, archived)
	require.NoError(t, err)
	_, err = repo.SetArchived(ctx, archived.ID, true, nil)
	require.NoError(t, err)

	active := newTestSellerProspect("在營運業者", "水果", "台南")
	active.TaxID = &taxID
	active, err = repo.Create(ctx, active)
	require.NoError(t, err, "narrow 過的 index 允許 ACTIVE 列拿走已封存列的統編")

	found, err := repo.FindByTaxID(ctx, taxID)
	require.NoError(t, err)
	require.Equal(t, active.ID, found.ID, "FindByTaxID 必須回活著的 ACTIVE 列，不是被封存的舊列")
	require.Equal(t, "ACTIVE", found.Status)
}
