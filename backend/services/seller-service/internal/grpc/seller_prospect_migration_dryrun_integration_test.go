//go:build integration

package grpc

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/go-pkg/testutil/dbtest"
	"github.com/Will413028/hawkr/seller-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/seller-service/internal/usecase"
)

// prospectMigrationPG／TestMain 是這個檔案獨有的 dbtest 設定：這個套件（grpc）
// 目前沒有任何整合測試，因此沒有既有 TestMain 可以掛。只在 -tags integration
// 下編譯，跟套件其餘的一般單元測試互不影響。
var prospectMigrationPG = dbtest.New(dbtest.Config{
	TemplateDB:    "seller_prospect_migration_test",
	MigrationsDir: filepath.Join("..", "..", "db", "migrations"),
})

func TestMain(m *testing.M) {
	dbtest.RunTests(m, prospectMigrationPG)
}

func setupProspectMigrationDB(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	db, drop := prospectMigrationPG.Clone(ctx, t)
	pool, err := pgxpool.New(ctx, db.DSN)
	require.NoError(t, err)
	return pool, func() { pool.Close(); drop() }
}

// TestMigrationDryRunAlgorithm_SurfacesTheTwoKnownDuplicatePairs 是 C2 修復的
// server 端驗證：scripts/migrate_candidate_leads_to_prospects.go 活在獨立的
// scripts/ module（go.mod 空白、//go:build ignore、無 go.sum），這個 module 的
// 測試無法直接呼叫它——這裡改為對真正的 CreateSellerProspect handler（script
// 唯一的寫入路徑，且是 script 檔頭「dry-run 演算法」段落描述的同一個機制）
// 重播 script 修好後會送出的完全相同序列：dry_run=false 真寫入 → 收集每一步
// 回報的 matches（staging pass）→ 跑完後用 DELETE ... WHERE id >= 首筆 id 清除。
//
// 資料取自 spec §1.2 記載的兩對真實重複（漁夫鮮撈／梓官區漁會共用 email
// marketing@example.org；多利多食品／多利多國際食品共用電話 04-2581-0586），另外
// 混入兩筆互不相關的業者，證明這套機制不會把不相關輸入誤判成重複（fail closed
// on false positives，不只 fail open on true positives）。
//
// 這條測試對真實 Postgres 執行，不是 mock/stub；它是「dry-run 對真的資料庫」
// 這一類demonstration，只是省略了 gRPC wire 傳輸本身（那一層在 Task 8 已由
// 其他測試覆蓋，這裡不重複驗證序列化）與 ai-service 端讀 candidate_leads 的
// 部分（單純 SELECT，沒有邏輯可驗證）。
func TestMigrationDryRunAlgorithm_SurfacesTheTwoKnownDuplicatePairs(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProspectMigrationDB(ctx, t)
	defer cleanup()

	var nextID int64 = 800_000_000
	newID := func() int64 { nextID++; return nextID }

	repo := postgres.NewSellerProspectRepository(pool, database.NewTxManager(pool), newID)
	uc := usecase.NewSellerProspectUseCase(repo, nil)
	srv := &SellerServer{prospectCreator: uc}

	type migrationLead struct {
		name, email, phone string
	}
	// 對照 scripts/migrate_candidate_leads_to_prospects.go 的 CreateSellerProspectRequest
	// 建構：candidate_leads 沒有統編欄位，這支腳本從不送 tax_id，所以只會撞到 T2。
	leads := []migrationLead{
		{"漁夫鮮撈", "marketing@example.org", "07-610-7551"},
		{"梓官區漁會", "marketing@example.org", "07-6107551"},
		{"多利多食品", "", "04-2581-0586"},
		{"多利多國際食品", "", "04-2581-0586"},
		{"無關業者甲", "unrelated-a@example.com", "02-1111-1111"},
		{"無關業者乙", "unrelated-b@example.com", "02-2222-2222"},
	}

	var mintedIDs []int64
	var proposals []*sellerpb.ProspectMatchCandidate
	for _, l := range leads {
		resp, err := srv.CreateSellerProspect(ctx, &sellerpb.CreateSellerProspectRequest{
			DisplayName: l.name, VendorType: "individual",
			PublicEmail: l.email, PublicPhone: l.phone,
			DryRun: false, // staging pass：script 修好後這裡永遠是 false，靠真寫入偵測批次內重複
		})
		require.NoError(t, err)
		require.NotNil(t, resp.GetData(), "staging pass 必須真的建立列，後面的輸入才能撞見它")

		id, err := strconv.ParseInt(resp.GetData().GetId(), 10, 64)
		require.NoError(t, err)
		mintedIDs = append(mintedIDs, id)
		proposals = append(proposals, resp.GetMatches()...)
	}

	require.Len(t, proposals, 2, "634 筆的縮影：兩對真實重複各觸發一次提案，不多不少——"+
		"不相關的兩筆業者不該產生任何提案")

	matchedFields := make(map[string]bool)
	for _, p := range proposals {
		matchedFields[p.GetMatchedOn()] = true
	}
	require.True(t, matchedFields["email"], "漁夫鮮撈／梓官區漁會這對必須以 email 命中")
	require.True(t, matchedFields["phone"], "多利多食品／多利多國際食品這對必須以 phone 命中")

	// 清除 staging pass 寫入的每一列——demo script 收尾用的同一條 DELETE。
	require.NotEmpty(t, mintedIDs)
	first := mintedIDs[0]
	tag, err := pool.Exec(ctx, `DELETE FROM seller_prospects WHERE id >= $1`, first)
	require.NoError(t, err)
	require.Equal(t, int64(len(leads)), tag.RowsAffected(), "必須清掉這次 staging pass 建立的每一列，不多不少")

	var remaining int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM seller_prospects WHERE id >= $1`, first).Scan(&remaining))
	require.Zero(t, remaining, "清除後不得留下任何 staging 列——這是 dry-run 對外承諾「未寫入任何資料」的前提")
}
