//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
	"github.com/stretchr/testify/require"
)

// 56 §5.4 的樂觀鎖 CAS，對真實 SQL 驗一次。
//
// 這一層不能只靠 mock：CAS 是 `AND (expected < 0 OR version = expected)` 這句述詞，
// 而它的兩個關鍵性質——「哨兵值 -1 停用比對」與「不符即 rows = 0」——都只在資料庫裡
// 成立。mock 可以宣稱參數有傳到，卻證明不了述詞本身寫對了。
func TestSellerVersionCAS(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerRepository(pool)
	now := time.Now()

	const accountID int64 = 2613400001

	seed := func(t *testing.T) *domain.Seller {
		t.Helper()
		require.NoError(t, repo.Create(ctx, &domain.Seller{
			AccountID:  accountID,
			StoreID:    accountID,
			StoreCode:  accountID,
			StoreName:  "梨山盛花園",
			Status:     domain.SellerStatusActive,
			SellerType: domain.SellerTypeIndividual,
			CreatedAt:  now,
			UpdatedAt:  now,
		}))
		s, err := repo.GetByAccountID(ctx, accountID)
		require.NoError(t, err)
		require.EqualValues(t, 0, s.Version, "a fresh row starts at version 0")
		return s
	}

	t.Run("matching version writes and bumps", func(t *testing.T) {
		s := seed(t)
		expected := s.Version
		s.StoreName = "改過的店名"
		require.NoError(t, repo.Update(ctx, s, &expected))

		got, err := repo.GetByAccountID(ctx, accountID)
		require.NoError(t, err)
		require.Equal(t, "改過的店名", got.StoreName)
		require.EqualValues(t, expected+1, got.Version, "a successful write must bump version, otherwise the next CAS compares against a stale value and never conflicts")
	})

	t.Run("stale version is refused and changes nothing", func(t *testing.T) {
		got, err := repo.GetByAccountID(ctx, accountID)
		require.NoError(t, err)
		nameBefore, versionBefore := got.StoreName, got.Version

		stale := versionBefore - 1
		got.StoreName = "不該被寫進去的店名"
		err = repo.Update(ctx, got, &stale)
		require.ErrorIs(t, err, domain.ErrVersionConflict)

		after, err := repo.GetByAccountID(ctx, accountID)
		require.NoError(t, err)
		require.Equal(t, nameBefore, after.StoreName, "a refused CAS must leave the row untouched — the whole point is not losing the concurrent edit")
		require.Equal(t, versionBefore, after.Version, "a refused CAS must not bump version either")
	})

	t.Run("nil expectedVersion writes without comparing", func(t *testing.T) {
		before, err := repo.GetByAccountID(ctx, accountID)
		require.NoError(t, err)

		before.StoreName = "賣家自己改的"
		require.NoError(t, repo.Update(ctx, before, nil), "the seller's own path has no expectedVersion and must not be blocked by the CAS")

		after, err := repo.GetByAccountID(ctx, accountID)
		require.NoError(t, err)
		require.Equal(t, "賣家自己改的", after.StoreName)
		require.EqualValues(t, before.Version+1, after.Version, "the seller's own write must still bump version, or a delegated reader's expectedVersion would go stale without ever noticing")
	})

	t.Run("logo and cover honour the same CAS", func(t *testing.T) {
		before, err := repo.GetByAccountID(ctx, accountID)
		require.NoError(t, err)

		stale := before.Version - 1
		require.ErrorIs(t, repo.UpdateLogo(ctx, accountID, "seller/x/logo.png", &stale), domain.ErrVersionConflict)
		require.ErrorIs(t, repo.UpdateCover(ctx, accountID, "seller/x/cover.png", &stale), domain.ErrVersionConflict)

		current := before.Version
		require.NoError(t, repo.UpdateLogo(ctx, accountID, "seller/x/logo.png", &current))
		afterLogo, err := repo.GetByAccountID(ctx, accountID)
		require.NoError(t, err)
		require.Equal(t, "seller/x/logo.png", afterLogo.Logo)
		require.EqualValues(t, current+1, afterLogo.Version)

		next := afterLogo.Version
		require.NoError(t, repo.UpdateCover(ctx, accountID, "seller/x/cover.png", &next))
		afterCover, err := repo.GetByAccountID(ctx, accountID)
		require.NoError(t, err)
		require.Equal(t, "seller/x/cover.png", afterCover.CoverKey)
		require.EqualValues(t, next+1, afterCover.Version)
	})
}

// 讀取路徑必須回真實 version，否則 CAS 有寫沒有讀、整套樂觀鎖只剩一半。
//
// 這一條是實打本機時抓到的：三支 profile 查詢用的是**明列欄位**而不是 SELECT *，
// 加了 version 欄位之後它們照樣編譯、照樣回 code 0，只是 version 恆為 0。代行編輯器
// 第一次存檔之後拿到的 expectedVersion 就永遠對不上，之後每一次存檔都 409，而錯誤訊息
// 指向「資料已被更新」——與真正的原因毫無關係。
func TestSellerProfileReadsReturnRealVersion(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupTestDB(ctx, t)
	defer cleanup()

	repo := postgres.NewSellerRepository(pool)
	now := time.Now()

	const accountID int64 = 2613400002

	require.NoError(t, repo.Create(ctx, &domain.Seller{
		AccountID:  accountID,
		StoreID:    accountID,
		StoreCode:  accountID,
		StoreName:  "版本讀取測試",
		Status:     domain.SellerStatusActive,
		SellerType: domain.SellerTypeIndividual,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))

	// Two successful writes ⇒ the row must be at version 2, and every read path
	// must say so. Two rather than one: a projection that drops the column reads
	// back as 0, which a single write from 0 → 1 would still catch, but a reader
	// that happens to echo the request's expectedVersion would not.
	for i := int64(0); i < 2; i++ {
		s, err := repo.GetByAccountID(ctx, accountID)
		require.NoError(t, err)
		expected := s.Version
		require.NoError(t, repo.Update(ctx, s, &expected))
	}

	byAccount, err := repo.GetByAccountID(ctx, accountID)
	require.NoError(t, err)
	require.EqualValues(t, 2, byAccount.Version, "GetByAccountID")

	profile, err := repo.GetProfileByAccountID(ctx, accountID)
	require.NoError(t, err)
	require.EqualValues(t, 2, profile.Seller.Version, "GetProfileByAccountID — 這支是 /app/sellers/me 與代行 storefront GET 的來源")

	public, err := repo.GetPublicProfileByStoreIDOrCode(ctx, accountID)
	require.NoError(t, err)
	require.EqualValues(t, 2, public.Seller.Version, "GetPublicProfileByStoreIDOrCode")

	byStore, err := repo.GetProfileByStoreID(ctx, accountID)
	require.NoError(t, err)
	require.EqualValues(t, 2, byStore.Seller.Version, "GetProfileByStoreID")
}
