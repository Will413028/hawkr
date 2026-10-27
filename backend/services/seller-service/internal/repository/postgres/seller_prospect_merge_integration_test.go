//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

func TestProspectMergedIntoRequiresArchived(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	nextID := testSellerProspectID()
	winnerID, loserID := nextID(), nextID()

	insert := `INSERT INTO seller_prospects (id, display_name, vendor_type, status, outreach_status)
	           VALUES ($1, $2, 'business', 'ACTIVE', 'NOT_CONTACTED')`
	_, err := pool.Exec(ctx, insert, winnerID, "贏家業者")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, insert, loserID, "輸家業者")
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`UPDATE seller_prospects SET merged_into_prospect_id = $1 WHERE id = $2`,
		winnerID, loserID)
	require.Error(t, err, "ACTIVE 列不得帶 merged_into_prospect_id")

	_, err = pool.Exec(ctx,
		`UPDATE seller_prospects SET status = 'ARCHIVED', merged_into_prospect_id = $1 WHERE id = $2`,
		winnerID, loserID)
	require.NoError(t, err)
}

func TestProspectMergedIntoRejectsSelfReference(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	id := testSellerProspectID()()
	_, err := pool.Exec(ctx,
		`INSERT INTO seller_prospects (id, display_name, vendor_type, status, outreach_status)
		 VALUES ($1, '自指業者', 'business', 'ARCHIVED', 'NOT_CONTACTED')`, id)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`UPDATE seller_prospects SET merged_into_prospect_id = $1 WHERE id = $1`, id)
	require.Error(t, err, "merged_into_prospect_id 不得指向自己")
}

func TestMarkConvertedSkipsMergedLoser(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	winner, err := repo.Create(ctx, newTestSellerProspect("贏家業者", "水果", "台南"))
	require.NoError(t, err)
	loser, err := repo.Create(ctx, newTestSellerProspect("輸家業者", "水果", "台南"))
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`UPDATE seller_prospects SET status = 'ARCHIVED', merged_into_prospect_id = $1 WHERE id = $2`,
		winner.ID, loser.ID)
	require.NoError(t, err)

	rows, err := repo.MarkConverted(ctx, loser.ID, 555002, nil)
	require.NoError(t, err, "status = 'ACTIVE' 述詞下，0 列受影響不該冒出資料庫錯誤")
	require.Equal(t, int64(0), rows, "被合併掉的 loser 不可被標記為 CONVERTED")
}

func TestMarkConvertedSkipsAlreadyConvertedProspect(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	prospect, err := repo.Create(ctx, newTestSellerProspect("已入駐業者", "水果", "台南"))
	require.NoError(t, err)

	rows, err := repo.MarkConverted(ctx, prospect.ID, 555003, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), rows, "第一次轉換應該成功")

	rows, err = repo.MarkConverted(ctx, prospect.ID, 555004, nil)
	require.NoError(t, err, "第二次轉換不該冒出資料庫錯誤")
	require.Equal(t, int64(0), rows, "已 CONVERTED 的列不得被二次覆寫")
}

func TestMergeArchivesLoserAndMovesInvites(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	winner, err := repo.Create(ctx, newTestSellerProspect("多利多食品", "食品", "台南"))
	require.NoError(t, err)
	loser, err := repo.Create(ctx, newTestSellerProspect("多利多國際食品", "食品", "台南"))
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO seller_prospect_invites (id, prospect_id, token, expires_at)
		 VALUES ($1, $2, 'tok-merge-test', now() + interval '30 days')`,
		int64(910000001), loser.ID)
	require.NoError(t, err)

	got, err := repo.Merge(ctx, winner.ID, loser.ID, nil)
	require.NoError(t, err)
	require.Equal(t, winner.ID, got.ID)

	var status string
	var mergedInto *int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, merged_into_prospect_id FROM seller_prospects WHERE id = $1`,
		loser.ID).Scan(&status, &mergedInto))
	require.Equal(t, "ARCHIVED", status)
	require.NotNil(t, mergedInto)
	require.Equal(t, winner.ID, *mergedInto)

	var inviteOwner int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT prospect_id FROM seller_prospect_invites WHERE token = 'tok-merge-test'`).Scan(&inviteOwner))
	require.Equal(t, winner.ID, inviteOwner, "loser 的邀請必須搬到 winner")
}

func TestMergeCompressesExistingChain(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	c, err := repo.Create(ctx, newTestSellerProspect("C", "食品", "台南"))
	require.NoError(t, err)
	b, err := repo.Create(ctx, newTestSellerProspect("B", "食品", "台南"))
	require.NoError(t, err)
	a, err := repo.Create(ctx, newTestSellerProspect("A", "食品", "台南"))
	require.NoError(t, err)

	_, err = repo.Merge(ctx, b.ID, a.ID, nil)
	require.NoError(t, err)
	_, err = repo.Merge(ctx, c.ID, b.ID, nil)
	require.NoError(t, err)

	var aMergedInto int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT merged_into_prospect_id FROM seller_prospects WHERE id = $1`, a.ID).Scan(&aMergedInto))
	require.Equal(t, c.ID, aMergedInto, "鏈必須壓縮成一層")
}

func TestMergeRejectsConvertedRow(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	winner, err := repo.Create(ctx, newTestSellerProspect("贏家", "食品", "台南"))
	require.NoError(t, err)
	loser, err := repo.Create(ctx, newTestSellerProspect("已入駐", "食品", "台南"))
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`UPDATE seller_prospects SET status = 'CONVERTED', converted_store_id = 999, converted_at = now() WHERE id = $1`,
		loser.ID)
	require.NoError(t, err)

	_, err = repo.Merge(ctx, winner.ID, loser.ID, nil)
	require.Error(t, err)
	require.True(t, errors.Is(err, usecase.ErrProspectMergeNotAllowed),
		"CONVERTED loser 必須回可辨識的 ErrProspectMergeNotAllowed，而不是普通錯誤——"+
			"呼叫端（Task 12 的 gRPC handler）要能把它跟真正的資料庫錯誤分開處理")
}

func TestMergeRejectsSameProspectID(t *testing.T) {
	repo := postgres.NewSellerProspectRepository(nil, nil, testSellerProspectID())

	_, err := repo.Merge(context.Background(), 42, 42, nil)

	require.Error(t, err)
	require.True(t, errors.Is(err, usecase.ErrProspectMergeNotAllowed),
		"winner 與 loser 是同一筆時必須回可辨識的 ErrProspectMergeNotAllowed")
}

func TestMergeKeepsWinnerTaxIDAndClearsLoserWhenBothHaveOne(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	winnerTaxID := "91100001"
	loserTaxID := "91100002"
	winner := newTestSellerProspect("贏家業者", "食品", "台南")
	winner.TaxID = &winnerTaxID
	winner, err := repo.Create(ctx, winner)
	require.NoError(t, err)
	loser := newTestSellerProspect("輸家業者", "食品", "台南")
	loser.TaxID = &loserTaxID
	loser, err = repo.Create(ctx, loser)
	require.NoError(t, err)

	got, err := repo.Merge(ctx, winner.ID, loser.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, got.TaxID)
	require.Equal(t, winnerTaxID, *got.TaxID, "勝方已經有自己的統編，合併不能覆寫")

	archivedLoser, err := repo.GetByID(ctx, loser.ID)
	require.NoError(t, err)
	require.Equal(t, "ARCHIVED", archivedLoser.Status)
	require.Nil(t, archivedLoser.TaxID, "敗方的統編即使沒被搬走，合併之後也必須清空")
}

func TestMergeLeavesTaxIDNilWhenNeitherHasOne(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	winner, err := repo.Create(ctx, newTestSellerProspect("贏家業者", "食品", "台南"))
	require.NoError(t, err)
	loser, err := repo.Create(ctx, newTestSellerProspect("輸家業者", "食品", "台南"))
	require.NoError(t, err)

	got, err := repo.Merge(ctx, winner.ID, loser.ID, nil)
	require.NoError(t, err)
	require.Nil(t, got.TaxID)

	archivedLoser, err := repo.GetByID(ctx, loser.ID)
	require.NoError(t, err)
	require.Nil(t, archivedLoser.TaxID)
}

func TestMergeCarriedTaxIDIsVisibleToReconciliation(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID())

	taxID := "91100003"
	winner, err := repo.Create(ctx, newTestSellerProspect("贏家業者", "食品", "台南"))
	require.NoError(t, err)
	loser := newTestSellerProspect("輸家業者", "食品", "台南")
	loser.TaxID = &taxID
	loser, err = repo.Create(ctx, loser)
	require.NoError(t, err)

	const accountID, storeID, certID = int64(910000101), int64(910000102), int64(910000103)
	_, err = pool.Exec(ctx,
		`INSERT INTO sellers (store_id, store_code, account_id, is_current, store_name)
		 VALUES ($1, $2, $3, true, '對帳測試店')`, storeID, storeID, accountID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO certification_applications (id, account_id, application_no, status, seller_type, tax_id, company_name)
		 VALUES ($1, $2, 'APP-MERGE-RECON-1', 'forwarded', 'business', $3, '對帳測試公司')`,
		certID, accountID, taxID)
	require.NoError(t, err)

	operatorAccountID := int64(910000104)
	got, err := repo.Merge(ctx, winner.ID, loser.ID, &operatorAccountID)
	require.NoError(t, err)
	require.NotNil(t, got.TaxID)
	require.Equal(t, taxID, *got.TaxID)
	require.NotNil(t, got.UpdatedByAccountID, "勝方的 tax_id 是這次合併寫入的，updated_by_account_id 必須記下操作者")
	require.Equal(t, operatorAccountID, *got.UpdatedByAccountID)

	matches, err := repo.ListTaxIDMatches(ctx)
	require.NoError(t, err)
	require.Len(t, matches, 1, "合併後勝方持有統編，對帳清單必須找到它——這就是整個 tax_id 搬動要解決的問題")
	require.Equal(t, winner.ID, matches[0].Prospect.ID)
	require.Equal(t, storeID, matches[0].StoreID)
}
