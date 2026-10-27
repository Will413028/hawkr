package app

import (
	"testing"

	"go.uber.org/fx"

	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/pricebook-service/internal/app/cron"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
	"github.com/Will413028/hawkr/pricebook-service/internal/domain/forecast"
	pricebookgrpc "github.com/Will413028/hawkr/pricebook-service/internal/grpc"
	"github.com/Will413028/hawkr/pricebook-service/internal/infrastructure"
	pricebookpostgres "github.com/Will413028/hawkr/pricebook-service/internal/repository/postgres"
	"github.com/Will413028/hawkr/pricebook-service/internal/usecase"
	"github.com/Will413028/hawkr/pricebook-service/internal/worker"
)

// TestFxGraph_Valid 確認整個 DI 圖可解析——尤其是 usecase.Module 提供的
// *usecase.RestDayIngest、*usecase.PriceIngest、*usecase.Backfill：在 Task 8
// 把每日 cron 接上它們之前，沒有任何 fx.Invoke 會消費這幾個型別，ValidateApp
// 不會主動走到，所以這裡明確補 fx.Invoke 強迫 dig 解析各自的依賴（*moa.Client、
// *pgxpool.Pool、func() int64——三個 usecase 共用同一組 provider，這正是 di.go
// 註解要防的「重複 provide」的另一面：確認三者都解析得到同一份、只註冊一次的
// 依賴；*usecase.Backfill 額外依賴 *usecase.PriceIngest 本身，一併確認這條邊
// 解析得到）。
//
// fx.ValidateApp 只建圖、不呼叫任何建構子、不跑 lifecycle hook（不需要真 DB/網路，
// 不用 Docker），是本任務 review 要求的、比一次性手動 smoke test（跑完即刪）更耐久
// 的回歸防線——專門防 `tsid.New()`（不存在的建構函式）這一類「go build/go vet 抓
// 不到、只在 fx 執行期才炸」的 DI 錯誤。
//
// Task 8：加入 cron.Module。它自己的 fx.Invoke(startDailyIngestJob) 已經會
// 強迫解析 *usecase.RestDayIngest / *usecase.PriceIngest / *pgxpool.Pool /
// func() int64 / *zap.Logger，下面針對 RestDayIngest/PriceIngest/Backfill 的
// 手動 fx.Invoke 保留（Task 5/6/7 加入時就在，*usecase.Backfill 目前沒有任何
// 呼叫端消費，仍需要它單獨撐住）——三者疊加確認同一組 provider 在多個消費端
// 下都解析得到同一份、只註冊一次的依賴。
//
// Task 9：加入 *usecase.Exporter。跟 Backfill 同理，唯一呼叫端是
// cmd/pricebook-export（不透過 fx），這裡的手動 fx.Invoke 是它在整個 fx 圖
// 唯一被解析到的路徑。
//
// Task 10：加入 pricebookpostgres.Module，provide domain.ForecastRepository。
// 跟 Backfill/Exporter 同理——Task 11（推論 usecase）尚未消費它，這裡的手動
// fx.Invoke(func(domain.ForecastRepository) {}) 是它在整個 fx 圖唯一被解析到
// 的路徑，Task 11 接上真正的消費端後可以留著（同型別重複 fx.Invoke 不是
// "already provided" 那種 panic，純粹多一次驗證）。
//
// Task 11：加入 *usecase.Forecast。跟 Backfill/Exporter 同理，唯一呼叫端是
// cmd/pricebook-forecast（不透過 fx），這裡的手動 fx.Invoke 是它在整個 fx 圖
// 唯一被解析到的路徑；同時確認它依賴的 domain.ForecastRepository / *config.Config /
// *zap.Logger 三者都解析得到同一份、只註冊一次的依賴。
//
// Task 4（GetObservedPrice）：加入 *usecase.ObservedPrice，依賴新的
// mapping.Seed（di.go 的 provideMappingSeed 提供，本 provider 讀
// reference/product_crop_seed.csv + .manifest.json）。跟 Backfill/Exporter/
// Forecast 同理，唯一呼叫端會是 Task 5 的 gRPC handler（尚未接上），這裡的
// 手動 fx.Invoke 是它在整個 fx 圖唯一被解析到的路徑——沒有這行，
// provideMappingSeed/NewObservedPrice 不會被 dig 實際建構過（ValidateApp 對
// 沒有消費端的 provider 是惰性的，見本函式最上方大段註解），驗證會是假陽性。
//
// Task 6：加入 pricebookgrpc.Module（Task 5 的 gRPC handler + Task 6 的
// server bootstrap）。跟 ObservedPrice usecase 同理，但這次不需要額外手動
// fx.Invoke：pricebookgrpc.Module 自己就帶了
// fx.Invoke(func(*grpclib.Server, *PricebookServer) {...})（把 handler 註冊
// 進 server），這個 invoke 本身會強迫 dig 解析整條鏈——infrastructure.Module
// 提供的 pkgserver.ServerConfig（struct，取代舊的 named "grpc_port" string
// provider）與 name:"service_name" string provider、
// pkgserver.NewGRPCServer（github.com/Will413028/hawkr/go-pkg/server，依賴
// fx.Lifecycle/ServerConfig/*zap.Logger）、以及 grpc.Module 內部把
// *usecase.ObservedPrice 橋接成 observedPriceUC 介面的 adapter provider。少了
// pricebookgrpc.Module，NewGRPCServer 雖然被 provide 但沒有消費端，跟本函式
// 最上方大段註解描述的「沒有消費端的 provider 是惰性的」問題一樣，這裡的驗證
// 會是假陽性。
//
// Task (model artifact loading): 加入 forecast.ModelArtifact，依賴新的
// cfg.ModelArtifactPath（di.go 的 provideModelArtifact 提供，讀 Docker image
// build 時 bake 進去的 model/model.json）。跟 Forecast/Backfill/Exporter
// 同理，目前沒有任何 RPC 消費它（預測 gRPC handler 是後續工作），這裡的手動
// fx.Invoke 是它在整個 fx 圖唯一被解析到的路徑——沒有這行，
// provideModelArtifact 不會被 dig 實際建構過，驗證會是假陽性。
//
// Task 8（issue #2249 Slice B GetTrendSummary）：加入 *usecase.TrendSummary，
// 依賴 domain.ForecastRepository / *usecase.Forecast / *config.Config /
// *zap.Logger——四者皆已有 provider（Forecast 由 Task 11 的
// fx.Provide(NewForecast) 提供），不需要新增任何一個。跟 ObservedPrice 同理，
// 唯一呼叫端會是 Task 9 的 gRPC handler（尚未接上），這裡的手動 fx.Invoke 是
// 它在整個 fx 圖唯一被解析到的路徑——沒有這行，NewTrendSummary 不會被 dig
// 實際建構過，驗證會是假陽性。
// Task 9（issue #2248 PR B GetSpatialSurface）：加入 *usecase.SpatialSurface，
// 依賴 forecast.ModelArtifact / geography.Registry / map[string]spatial.PriceScale
// / *usecase.Forecast——四者皆已有 provider（前兩者由 Task 8 加入，第三者同）。
// 這行同時是 geography.Registry 與 price scale 兩個 provider 第一次真的被
// 建構到的路徑：Task 8 註冊它們時還沒有任何消費端，惰性 provider 不會被
// dig 走到。gRPC handler（Task 10）接上後這行仍可留著（同型別重複 fx.Invoke
// 不是錯誤）。
func TestFxGraph_Valid(t *testing.T) {
	err := fx.ValidateApp(
		config.Module,
		usecase.Module,
		infrastructure.Module,
		cron.Module,
		pricebookpostgres.Module,
		pricebookgrpc.Module,
		worker.Module,
		fx.Invoke(func(*usecase.RestDayIngest) {}),
		fx.Invoke(func(*usecase.PriceIngest) {}),
		fx.Invoke(func(*usecase.Backfill) {}),
		fx.Invoke(func(*usecase.Exporter) {}),
		fx.Invoke(func(domain.ForecastRepository) {}),
		fx.Invoke(func(*usecase.Forecast) {}),
		fx.Invoke(func(*usecase.ObservedPrice) {}),
		fx.Invoke(func(forecast.ModelArtifact) {}),
		fx.Invoke(func(*usecase.TrendSummary) {}),
		fx.Invoke(func(*usecase.SpatialSurface) {}),
		fx.Invoke(func(*pkgserver.HealthController) {}),
		fx.Invoke(func(pkgserver.GRPCServerParams) {}),
	)
	if err != nil {
		t.Fatalf("fx dependency graph invalid: %v", err)
	}
}
