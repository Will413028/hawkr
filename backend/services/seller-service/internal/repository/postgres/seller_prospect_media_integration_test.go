//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/seller-service/internal/domain"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
)

const (
	testLogoKey   = "public/seller_prospect_logo/1/20260920/a.png"
	testCoverKey  = "public/seller_prospect_cover/1/20260920/b.png"
	testLogoPage  = "https://www.mahimahi.com.tw/"
	testCoverPage = "https://docmall.doctw.com/story_page.php?id=128"
)

func newProspectRepoForMedia(ctx context.Context, t *testing.T) (domain.SellerProspectRepository, func()) {
	t.Helper()
	pool, cleanup := setupTestDB(ctx, t)
	return postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), testSellerProspectID()), cleanup
}

// 遷移後的既有列必須拿到可用的預設值：空 key、空 source、版本 0。
// 版本若落成 NULL，第一次換圖的 CAS 會永遠對不上。
func TestProspectMediaDefaultsAfterMigration(t *testing.T) {
	ctx := context.Background()
	repo, cleanup := newProspectRepoForMedia(ctx, t)
	defer cleanup()

	created, err := repo.Create(ctx, newTestSellerProspect("尚未建圖業者", "水產", "高雄"))
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Empty(t, got.LogoKey)
	require.Empty(t, got.CoverKey)
	require.Empty(t, got.LogoSourceURL)
	require.Empty(t, got.CoverSourceURL)
	require.EqualValues(t, 0, got.MediaVersion)
}

// 換 logo 不可以動到 cover 的欄位，反之亦然——兩個角色各自獨立。
func TestUpdateMediaKeepsTheOtherRoleIntact(t *testing.T) {
	ctx := context.Background()
	repo, cleanup := newProspectRepoForMedia(ctx, t)
	defer cleanup()

	created, err := repo.Create(ctx, newTestSellerProspect("兩角色業者", "水產", "高雄"))
	require.NoError(t, err)

	afterLogo, err := repo.UpdateMedia(ctx, domain.ProspectMediaUpdate{
		ProspectID: created.ID, Kind: "logo",
		ObjectKey: testLogoKey, SourceURL: testLogoPage, ExpectedMediaVersion: 0,
	})
	require.NoError(t, err)
	require.Equal(t, testLogoKey, afterLogo.LogoKey)
	require.Equal(t, testLogoPage, afterLogo.LogoSourceURL)
	require.Empty(t, afterLogo.CoverKey)
	require.EqualValues(t, 1, afterLogo.MediaVersion)

	afterCover, err := repo.UpdateMedia(ctx, domain.ProspectMediaUpdate{
		ProspectID: created.ID, Kind: "cover",
		ObjectKey: testCoverKey, SourceURL: testCoverPage, ExpectedMediaVersion: 1,
	})
	require.NoError(t, err)
	require.Equal(t, testCoverKey, afterCover.CoverKey)
	require.Equal(t, testCoverPage, afterCover.CoverSourceURL)
	// 換封面沒有把 logo 洗掉。
	require.Equal(t, testLogoKey, afterCover.LogoKey)
	require.Equal(t, testLogoPage, afterCover.LogoSourceURL)
	require.EqualValues(t, 2, afterCover.MediaVersion)
}

// 樂觀鎖：拿舊版本再寫一次必須落空，而且不能把已經寫好的那張圖蓋掉。
func TestUpdateMediaRejectsStaleVersion(t *testing.T) {
	ctx := context.Background()
	repo, cleanup := newProspectRepoForMedia(ctx, t)
	defer cleanup()

	created, err := repo.Create(ctx, newTestSellerProspect("併發換圖業者", "水產", "高雄"))
	require.NoError(t, err)

	_, err = repo.UpdateMedia(ctx, domain.ProspectMediaUpdate{
		ProspectID: created.ID, Kind: "logo",
		ObjectKey: testLogoKey, SourceURL: testLogoPage, ExpectedMediaVersion: 0,
	})
	require.NoError(t, err)

	_, err = repo.UpdateMedia(ctx, domain.ProspectMediaUpdate{
		ProspectID: created.ID, Kind: "logo",
		ObjectKey: "public/seller_prospect_logo/1/20260920/stale.png",
		SourceURL: testLogoPage, ExpectedMediaVersion: 0,
	})
	require.ErrorIs(t, err, domain.ErrProspectMediaConflict)

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, testLogoKey, got.LogoKey, "落敗的寫入不得覆蓋先寫入者的圖")
	require.EqualValues(t, 1, got.MediaVersion)
}

// ARCHIVED 與被合併的列只供檢視。述詞擋下來時回的是衝突而不是 not found——
// 呼叫端要能分辨「不給改」與「不存在」。
func TestUpdateMediaRejectsNonActiveRow(t *testing.T) {
	ctx := context.Background()
	repo, cleanup := newProspectRepoForMedia(ctx, t)
	defer cleanup()

	archived := newTestSellerProspect("已封存業者", "水產", "高雄")
	archived.Status = "ARCHIVED"
	created, err := repo.Create(ctx, archived)
	require.NoError(t, err)

	_, err = repo.UpdateMedia(ctx, domain.ProspectMediaUpdate{
		ProspectID: created.ID, Kind: "logo",
		ObjectKey: testLogoKey, SourceURL: testLogoPage, ExpectedMediaVersion: 0,
	})
	require.ErrorIs(t, err, domain.ErrProspectMediaConflict)
}

func TestUpdateMediaOnMissingRowIsConflictNotPanic(t *testing.T) {
	ctx := context.Background()
	repo, cleanup := newProspectRepoForMedia(ctx, t)
	defer cleanup()

	_, err := repo.UpdateMedia(ctx, domain.ProspectMediaUpdate{
		ProspectID: 9999999999999999, Kind: "logo",
		ObjectKey: testLogoKey, SourceURL: testLogoPage, ExpectedMediaVersion: 0,
	})
	require.ErrorIs(t, err, domain.ErrProspectMediaConflict)
}
