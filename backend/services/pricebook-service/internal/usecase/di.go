package usecase

import (
	"time"

	"github.com/Will413028/hawkr/go-pkg/tsid"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/pricebook-service/internal/client/category"
	"github.com/Will413028/hawkr/pricebook-service/internal/client/moa"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/geography"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/mapping"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/porkmarket"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/ricecity"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/spatial"
	"github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
)

// provideMOAClient 從 cfg.MOABaseURL 建立農業部 API client。
//
// 這是 *moa.Client 在整個服務 fx 圖中唯一的一處註冊——見下方 Module 註解，
// 兩者是同一條規則（provide 恰好一次），寫在這裡是因為這是唯一的 provider 定義點。
func provideMOAClient(cfg *config.Config) *moa.Client {
	return moa.NewClient(cfg.MOABaseURL, cfg.MOAOpenDataBaseURL)
}

// provideMappingSeed 從 cfg 指定的路徑載入 P4 的商品↔作物種子表。LoadSeed
// 對 sha256/manifest 不符是硬錯誤（見 mapping.ErrDigestMismatch）——服務啟動
// 失敗比帶著損毀的種子表默默跑起來安全。
//
// 跟 provideMOAClient 同一條「provide 恰好一次」規則：mapping.Seed 只在這裡
// 註冊一次，新 usecase 需要它就直接依賴 mapping.Seed 參數，不要再加第二個
// fx.Provide(provideMappingSeed) 或 fx.Provide(mapping.LoadSeed) 之類的重複註冊。
func provideMappingSeed(cfg *config.Config) (mapping.Seed, error) {
	return mapping.LoadSeed(cfg.MappingSeedCSVPath, cfg.MappingSeedManifestPath)
}

// provideGeographyRegistry loads the category-aware market geography
// registry (internal/domain/geography, PR #2321) exactly once at startup.
// This is geography's first real consumer — the package existed with no
// DI wiring before issue #2248 PR B needed it. Same "provide exactly
// once" rule as provideMappingSeed: new usecases depend on
// geography.Registry directly, don't add a second fx.Provide(geography.Load).
func provideGeographyRegistry(cfg *config.Config) (geography.Registry, error) {
	return geography.Load(
		cfg.GeographyMarketRegistryCSVPath,
		cfg.GeographyMarketRegistryManifestPath,
		cfg.GeographyBoundaryGeoJSONPath,
		cfg.GeographyBoundaryManifestPath,
	)
}

// providePriceScales loads the versioned fixed price-color scale
// (internal/domain/spatial, issue #2248 PR B) exactly once at startup.
func providePriceScales(cfg *config.Config) (map[string]spatial.PriceScale, error) {
	return spatial.LoadPriceScales(cfg.PriceScaleCSVPath, cfg.PriceScaleManifestPath)
}

// provideSpatialSurface 建構 issue #2248 PR B 的空間曲面 usecase。
//
// 為什麼要一個 provider 函式而不是直接 fx.Provide(NewSpatialSurface)：
// NewSpatialSurface 的第四個參數型別是套件內未匯出的 seriesForecaster 介面
// （為了讓測試能塞 fake，不必扛整個 *Forecast 的 DB 依賴）。dig 是照具體型別
// 解析的，圖上沒有任何東西 provide 那個介面，直接註冊建構子會在啟動時噴
// "missing type: usecase.seriesForecaster"。這裡把同一個 *Forecast 同時當作
// 介面實作與大類列舉來源傳進去，是兩個參數指向同一個依賴、不是兩份依賴。
//
// 依賴皆已有 provider，不新增任何一個：forecast.ModelArtifact
// （provideModelArtifact）、geography.Registry（provideGeographyRegistry）、
// map[string]spatial.PriceScale（providePriceScales）、*Forecast（既有
// fx.Provide(NewForecast)）。呼叫端是 internal/grpc 的 PricebookServer
// （GetSpatialSurface handler，已於 Task 10 接上，經 internal/grpc/di.go 的
// spatialSurfaceUC bridge）。internal/app/app_validate_test.go 仍留著手動的
// fx.Invoke(func(*usecase.SpatialSurface) {})，讓 ValidateApp 不因日後
// handler 改動而失去這條解析路徑（同型別重複 fx.Invoke 不是錯誤）。
func provideSpatialSurface(artifact forecast.ModelArtifact, geo geography.Registry, scales map[string]spatial.PriceScale, forecastUC *Forecast) *SpatialSurface {
	return NewSpatialSurface(artifact, geo, scales, forecastUC, forecastUC)
}

// provideRegionalComparison builds issue #2249 Slice C's regional
// comparison usecase. Depends on forecast.ModelArtifact
// (provideModelArtifact), geography.Registry (provideGeographyRegistry),
// *Forecast (existing fx.Provide(NewForecast)) — all already have
// providers, none added here. Unlike provideSpatialSurface, no
// map[string]spatial.PriceScale dependency: this usecase never draws a
// map, so it has no legend to look up.
func provideRegionalComparison(artifact forecast.ModelArtifact, geo geography.Registry, forecastUC *Forecast) *RegionalComparison {
	return NewRegionalComparison(artifact, geo, forecastUC, forecastUC)
}

// provideUpsertCategoryMapping 建構 issue #2786 的 admin mapping 寫入
// usecase（Task 10）。
//
// 為什麼要一個 provider 函式而不是直接 fx.Provide(NewUpsertCategoryMapping)：
// NewUpsertCategoryMapping 的第二個參數型別是套件內未匯出的
// categoryTreeSource 介面（同 provideSpatialSurface 的 seriesForecaster 手法
// ——為了讓測試能塞 fake，不必扛整個 *category.Client 的網路依賴）。dig 是照
// 具體型別解析的，圖上沒有任何東西 provide 那個介面，直接註冊建構子會在啟動時
// 噴 "missing type: usecase.categoryTreeSource"。這裡把 *category.Client 同時
// 當作介面實作傳進去。
//
// 依賴皆已有 provider，不新增任何一個：domain.CategoryMappingRepository（本檔
// fx.Provide(postgres.NewCategoryMappingRepository)）、*category.Client（本檔
// fx.Provide(category.NewCategoryClient)）。呼叫端是 internal/grpc 的
// PricebookServer（UpsertCategoryMapping handler），透過既有
// fx.Invoke(RegisterPricebookServiceServer) 一併涵蓋，不需要在
// app_validate_test.go 額外補 fx.Invoke。
func provideUpsertCategoryMapping(repo domain.CategoryMappingRepository, client *category.Client) *UpsertCategoryMapping {
	return NewUpsertCategoryMapping(repo, client)
}

// provideListCategoryMappingWorklist 建構 issue #2786 的 admin 待處理清單
// usecase（Task 10）。跟 provideUpsertCategoryMapping 同理需要橋接
// categoryTreeSource，依賴同樣皆已有 provider，呼叫端同樣是
// PricebookServer（ListCategoryMappingWorklist handler），透過既有
// fx.Invoke(RegisterPricebookServiceServer) 一併涵蓋。
func provideListCategoryMappingWorklist(repo domain.CategoryMappingRepository, client *category.Client) *ListCategoryMappingWorklist {
	return NewListCategoryMappingWorklist(repo, client)
}

// provideRollupBatch 橋接 categoryTreeSource，同 provideUpsertCategoryMapping/
// provideListCategoryMappingWorklist 的既有理由：*category.Client 滿足介面，
// 只是型別不是介面本身，fx 需要一個具名 provider 做這層轉換。
// artifact forecast.ModelArtifact 走既有的 provideModelArtifact provider（見
// 下方定義與其註解「forecast.ModelArtifact 只在這裡註冊一次」）——同
// provideSpatialSurface/provideRegionalComparison/NewEstimatedItemPrice/
// NewForecastCurve 的既有消費模式，直接宣告一個 forecast.ModelArtifact
// 參數讓 fx 從既有 provider 接線，不重新載入一次 model.json。
func provideRollupBatch(
	forecastUC *Forecast,
	commodityForecastUC *CommodityForecast,
	artifact forecast.ModelArtifact,
	mappingRepo domain.CategoryMappingRepository,
	rollupRepo domain.CategoryPriceRollupRepository,
	client *category.Client,
	cfg *config.Config,
	logger *zap.Logger,
) *RollupBatch {
	return NewRollupBatch(forecastUC, commodityForecastUC, artifact, mappingRepo, rollupRepo, client, cfg, logger, time.Now)
}

// provideModelArtifact 從 cfg.ModelArtifactPath 載入正式 model.json（由
// Docker image build 時 bake 進去，見 model/registry.json +
// cmd/pricebook-model-fetch，docs/superpowers/specs/
// 2026-08-01-pricebook-model-artifact-loading-design.md）。LoadArtifact 對
// schema version / feature_schema 不符是硬錯誤——服務啟動失敗比帶著損毀或
// 版本不符的模型默默跑起來安全，同 provideMappingSeed 的 ErrDigestMismatch
// 哲學。
//
// 跟 provideMOAClient/provideMappingSeed 同一條「provide 恰好一次」規則：
// forecast.ModelArtifact 只在這裡註冊一次，新 usecase（例如未來的
// 預測 gRPC handler）需要它就直接依賴 forecast.ModelArtifact 參數，不要再加
// 第二個 fx.Provide(provideModelArtifact) 或 fx.Provide(forecast.LoadArtifact)。
func provideModelArtifact(cfg *config.Config) (forecast.ModelArtifact, error) {
	return forecast.LoadArtifact(cfg.ModelArtifactPath)
}

// Module 提供的 fx.Provide(tsid.NewGenerator) 與 fx.Provide(provideMOAClient)
// 都是整個服務 fx 圖中唯一的一處註冊——同型別被 provide 兩次不會是編譯期錯誤，
// 而是啟動時 dig 的 "already provided" runtime panic，`go build`/`go vet` 完全
// 抓不到（`internal/app/app_validate_test.go` 的 `TestFxGraph_Valid` 是這類錯誤
// 的回歸防線，但那條測試防的是「缺 provider」，防不了「重複 provider」——dig 對
// 重複註冊是直接 panic，不是回傳可比對的 error，ValidateApp 一樣會炸，但錯誤訊息
// 是「already provided」而不是這裡描述的「missing type」）。
//
// 所以：
//   - 新 usecase 需要 TSID → 走這裡注入的 func() int64，不要再加第二個
//     fx.Provide(tsid.NewGenerator)（brief/preflight 沒有明講這點時最容易犯，
//     Task 6/7/8 只讀各自的 task brief 很可能不會注意到這裡已經 provide 過）。
//   - 新 usecase 需要 *moa.Client（例如 Task 6 的行情 ingest）→ 一樣走這裡的
//     provideMOAClient，不要再加第二個 fx.Provide(func(*config.Config) *moa.Client)
//     或直接 fx.Provide(moa.NewClient) 之類的重複註冊。
//
// Task 9 補的 NewExporter 只依賴既有的 *pgxpool.Pool，不需要新 provider——
// 跟 RestDayIngest/PriceIngest/Backfill 一樣，即使目前唯一的呼叫端是
// cmd/pricebook-export（不透過 fx），仍在此註冊：本服務的慣例是每個 usecase
// 都進 fx 圖，由 internal/app/app_validate_test.go 的 TestFxGraph_Valid 統一
// 守住 DI 正確性，不為單一 usecase 開特例。
//
// Task 11 補的 NewForecast 依賴 domain.ForecastRepository（Task 10 的
// pricebookpostgres.Module 提供）、*config.Config、*zap.Logger——三者皆已有
// provider，不需要新增任何一個。目前唯一的呼叫端是 cmd/pricebook-forecast
// （不透過 fx），與 Exporter 同理仍在此註冊。
//
// Task 4（本任務）補的 provideMappingSeed / NewObservedPrice：ObservedPrice
// 依賴 domain.ForecastRepository（同上，pricebookpostgres.Module 提供）、
// mapping.Seed（新增，本檔 provideMappingSeed 提供）、*config.Config——皆不
// 重複註冊。目前唯一的呼叫端會是 Task 5 的 gRPC handler（尚未接上），跟
// Backfill/Exporter/Forecast 同理仍在此註冊，由
// internal/app/app_validate_test.go 的 TestFxGraph_Valid 統一守住 DI 正確性。
//
// provideModelArtifact 跟上面幾個 provider 不同：它在本檔多帶了一個
// fx.Invoke（緊接在 fx.Provide(provideModelArtifact) 後面），在真正跑起來的
// 服務裡也會強迫 dig 建構 forecast.ModelArtifact，並印一行 "model artifact
// loaded" log——不是只靠 app_validate_test.go 的 TestFxGraph_Valid 撐著（那條
// 測試只驗證圖可解析，從不真的呼叫任何建構子，見該檔案最上方註解）。這是本服務
// 目前唯一「real fx.Invoke + test-only fx.Invoke」兩者並存的 provider：模型
// 載入失敗要讓 pod 啟動失敗、且要在 pod log 留下成功載入的證據，兩者都要求
// 有一個在正式啟動路徑上真的執行的消費端，其他 provider（Backfill/Exporter/
// Forecast/ObservedPrice）目前的唯一消費端仍是各自的 cmd/ 或尚未接上的 handler。
//
// P5（本任務）補的 NewEstimatedItemPrice：依賴 *Forecast（本檔既有
// fx.Provide(NewForecast)）、domain.ForecastRepository、mapping.Seed
// （provideMappingSeed 提供）、forecast.ModelArtifact（provideModelArtifact
// 提供）——皆不重複註冊。這是本 package第一個依賴其他 usecase 的 usecase：
// 直接複用 SeriesForecast/GroupForecast 既有的日曆/品質閘門/狀態判斷邏輯，
// 避免重複那段複雜度（見 estimated_item_price.go 自己的說明）。呼叫端是
// internal/grpc 的 PricebookServer（grpc/di.go 接線），透過既有
// fx.Invoke(RegisterPricebookServiceServer) 一併涵蓋，不需要在
// app_validate_test.go 額外補 fx.Invoke。
//
// P6+P7（本任務）補的 NewForecastCurve：依賴 *Forecast（本檔既有
// fx.Provide(NewForecast)）、domain.ForecastRepository、mapping.Seed
// （provideMappingSeed 提供）、forecast.ModelArtifact（provideModelArtifact
// 提供）——皆不重複註冊。跟 NewEstimatedItemPrice 共用同一批依賴、也共用
// crop_resolution.go 的解析＋覆蓋率 pre-check，只是輸出形狀不同（完整 7 點
// 曲線 vs 單一換算價）。呼叫端是 internal/grpc 的 PricebookServer（grpc/di.go
// 接線），透過既有 fx.Invoke(RegisterPricebookServiceServer) 一併涵蓋，不需
// 要在 app_validate_test.go 額外補 fx.Invoke。
//
// Task 5（本任務，v2 seller market lookup）：NewObservedPrice 新增
// *Forecast 依賴（呼叫其 GroupObservedSeries 修正大類歷史成交價的鋸齒
// bug）。*Forecast 已由本檔既有 fx.Provide(NewForecast) 提供，不需要新增
// provider——這是本 package 第二個依賴其他 usecase 的 usecase
// （EstimatedItemPrice/ForecastCurve 已是第一批）。
//
// 本任務補的 NewSearchCropCatalog：依賴 domain.ForecastRepository（同上）、
// mapping.Seed（provideMappingSeed 提供）、forecast.ModelArtifact
// （provideModelArtifact 提供）——皆不重複註冊。呼叫端是 internal/grpc 的
// PricebookServer（grpc/di.go 接線，Task 9），透過既有
// fx.Invoke(RegisterPricebookServiceServer) 一併涵蓋。
//
// Task 8（本任務）補的 NewTrendSummary：依賴 domain.ForecastRepository（同
// 上）、*Forecast（本檔既有 fx.Provide(NewForecast)，用於群組路徑的
// ComputeGroupVolumes 空間權重）、*config.Config、*zap.Logger——四者皆已有
// provider，不需要新增任何一個。呼叫端是 internal/grpc 的 PricebookServer
// （Task 9 接線），透過既有 fx.Invoke(RegisterPricebookServiceServer) 一併
// 涵蓋；在 Task 9 接上之前，internal/app/app_validate_test.go 的
// TestFxGraph_Valid 額外補了一個手動 fx.Invoke(func(*usecase.TrendSummary)
// {})（同 ObservedPrice/Forecast 等既有 provider 的手法）——沒有消費端的
// provider 是惰性的，少了它 ValidateApp 不會真的建構出 TrendSummary，驗證
// 會是假陽性。
//
// Task 3（每日批次預測 cron，本任務）補的 NewForecastBatch：依賴 *Forecast
// （本檔既有 fx.Provide(NewForecast)）、*pgxpool.Pool（pricebookpostgres.Module
// 提供）、tsid func() int64（本檔既有 fx.Provide(tsid.NewGenerator)）、
// forecast.ModelArtifact（provideModelArtifact 提供）、*zap.Logger（final
// whole-branch review Finding M1 補的注入依賴，internal/config/di.go 既有
// fx.Provide 提供）——五者皆已有 provider，不需要新增任何一個。呼叫端是
// internal/app/cron 的 startDailyForecastJob（fx.Invoke(startDailyForecastJob)），
// 與 startDailyIngestJob 同理直接消費 *usecase.ForecastBatch 具體型別。
var Module = fx.Module("usecase",
	fx.Provide(tsid.NewGenerator),
	fx.Provide(provideMOAClient),
	fx.Provide(provideMappingSeed),
	fx.Provide(provideGeographyRegistry),
	fx.Provide(providePriceScales),
	fx.Provide(NewRestDayIngest),
	fx.Provide(NewPriceIngest),
	fx.Provide(providePorkMarketRegistry),
	fx.Provide(NewPorkIngest),
	fx.Provide(NewPoultryChickenEggIngest),
	fx.Provide(NewPoultryGooseDuckIngest),
	fx.Provide(provideRiceCityRegistry),
	fx.Provide(NewRiceIngest),
	fx.Provide(NewCommodityForecast),
	fx.Provide(NewBackfill),
	fx.Provide(NewExporter),
	fx.Provide(NewForecast),
	fx.Provide(NewObservedPrice),
	fx.Provide(postgres.NewMarketOverviewRepository),
	fx.Provide(postgres.NewMarketWatchRepository),
	fx.Provide(NewMarketOverview),
	fx.Provide(NewMarketWatch),
	fx.Provide(provideModelArtifact),
	fx.Provide(NewForecastBatch),
	fx.Provide(NewEstimatedItemPrice),
	fx.Provide(NewForecastCurve),
	fx.Provide(NewSearchCropCatalog),
	fx.Provide(NewSearchPriceableEntities),
	fx.Provide(NewTrendSummary),
	fx.Provide(provideSpatialSurface),
	fx.Provide(provideRegionalComparison),
	fx.Provide(postgres.NewCategoryMappingRepository),     // domain.CategoryMappingRepository 的具體實作
	fx.Provide(postgres.NewSurfaceFormRepository),         // domain.SurfaceFormRepository 的具體實作
	fx.Provide(postgres.NewCategoryPriceRollupRepository), // domain.CategoryPriceRollupRepository 的具體實作
	fx.Provide(category.NewCategoryClient),
	fx.Provide(NewResolveCategoryMapping),
	fx.Provide(provideUpsertCategoryMapping),
	fx.Provide(provideListCategoryMappingWorklist),
	fx.Provide(provideRollupBatch),
	fx.Provide(NewGetCategoryPriceRollup),
	fx.Invoke(func(a forecast.ModelArtifact, l *zap.Logger) {
		l.Info("model artifact loaded",
			zap.String("model_version", a.ModelVersion),
			zap.Time("trained_at", a.TrainedAt),
			zap.String("dataset_digest", a.DatasetDigest),
			zap.Int("series_count", len(a.Series)),
		)
	}),
)

func providePorkMarketRegistry(cfg *config.Config) (porkmarket.Registry, error) {
	return porkmarket.Load(cfg.PorkMarketRegistryCSVPath, cfg.PorkMarketRegistryManifestPath)
}

func provideRiceCityRegistry(cfg *config.Config) (ricecity.Registry, error) {
	return ricecity.Load(cfg.RiceCityRegistryCSVPath, cfg.RiceCityRegistryManifestPath)
}
