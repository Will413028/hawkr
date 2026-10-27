package infrastructure

import (
	"context"
	"fmt"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	pkgdb "github.com/Will413028/hawkr/go-pkg/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

// ProvidePool 建立 ai-service 既有 AI 表的 pgx 連線池，供四個已遷移到 sqlc 的
// usecase 使用（StyleUseCase（Wave 2）、QuotaUseCase（Wave 3）、
// CandidateSearchUseCase（Wave 4）、TransformUseCase（Wave 5））。六波 gorm→sqlc/pgx
// 遷移於 Wave 6 收尾，gorm 版的 ProvideDB 已移除，這四個 usecase 從此都只走本函式
// 提供的池。DSN 組法跟 internal/procurement/module.go 的 newProcurementPool 相同，
// 但連線池本身完全獨立——procurement 那個池刻意跟既有 AI 表分開（避免耦合，見該
// 檔案註解），這裡的池不重用它，兩者都連同一個 hawkr_ai 資料庫、各自一份
// connection pool。
//
// fx 圖裡已經有 procurement.Module 提供的一個不具名 *pgxpool.Pool
// （newProcurementPool）。這裡若也用不具名方式 provide 同一個型別，dig 會在
// 建圖時報「型別重複 provide」。用 fx.ResultTags 具名為 "ai_pool" 避開衝突，
// 見 internal/usecase/di.go 對應的 fx.ParamTags（Wave 2 Task 2）。
func ProvidePool(lc fx.Lifecycle, cfg *config.Config) (*pgxpool.Pool, error) {
	dsn := pkgdb.BuildDSN(pkgdb.DBConfig{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		Name:     cfg.DBName,
		SSLMode:  cfg.DBSSLMode,
	})
	pool, err := pkgdb.NewPool(context.Background(), dsn)
	if err != nil {
		return nil, fmt.Errorf("開啟 pgx 連線池失敗: %w", err)
	}
	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			pool.Close()
			return nil
		},
	})
	return pool, nil
}
