package app

import (
	"testing"

	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"go.uber.org/fx"
)

// TestFxGraph_Valid 確認整個 DI 圖可解析——尤其是 procurement.Module 這次新加的
// provider 鏈（tsid.NewGenerator → repository.New → usecase.NewGeminiSlotExtractor →
// usecase.NewProcurementGraphRunner → handler.New；usecase.NewTurnUseCase 已隨
// procurement graph cutover（2026-08-31）刪除，見 usecase/di.go 開頭註解）。
// fx.ValidateApp 只建圖、不呼叫任何
// 建構子、不跑 lifecycle hook（不需要真 DB/網路/Gemini API key），是比「編譯過」更強的
// 檢查：它會抓出建構函式簽名對不上、型別沒 provide、或重複 provide 這類只在執行期
// fx.New(...).Run() 才會 panic 的錯誤。
//
// modules() 同時包含 final gRPC bootstrap 的 named health controller 與 readiness
// invoke；下面額外消費 HealthController／GRPCServerParams，確保 shared server、
// 三個 generated service registration 及 telemetry 依賴都真的被 dig 解析。
func TestFxGraph_Valid(t *testing.T) {
	options := append(modules(),
		fx.Invoke(func(*pkgserver.HealthController) {}),
		fx.Invoke(func(pkgserver.GRPCServerParams) {}),
	)
	err := fx.ValidateApp(options...)
	if err != nil {
		t.Fatalf("fx dependency graph invalid: %v", err)
	}
}
