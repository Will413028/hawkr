package app

import (
	"testing"

	"go.uber.org/fx"
)

// TestNew_WholeGraphResolves 驗**整個服務**的 fx 圖解得開。
//
// 為什麼需要這一支（2026-08-30 實際踩到才補）：每一層各自有 di_test.go，但那些測試
// 都用 `fx.Provide(func() X { return nil })` 供跨層依賴 ⇒ **它們驗的是自己那一層的形狀，
// 不是整張圖**。實例：把 worker 的 SellerAccountResolver 從佔位換成真的 *seller.Client
// 之後，worker 那一層的圖測試因為自己供了 nil 而仍然綠，真正缺的
// `client.Module` 一直到把整張圖組起來才浮現——而在補這支之前，那個「浮現」的時機是
// **叢集上的開機**。
//
// ⚠️ `fx.ValidateApp` 不呼叫任何建構函式 ⇒ 它驗的是「每個依賴都有人提供、型別對得上」，
// 驗不到「連得上 DB／MQ」或「promauto 重複註冊」。那兩類要靠實跑與各層的 TestMain。
//
// 🔴 **它也驗不到「有沒有被註冊」**（2026-08-30 變異實測）：把 `internal/grpc` 的
// `fx.Invoke` 掏空到不再呼叫 `RegisterInvoiceServiceServer`，本測試**照樣全綠** ——
// 因為沒有任何型別依賴「已註冊」這件事。服務會正常開機、gRPC health 仍是 SERVING，
// 而每一支 RPC 回 `Unimplemented`。攔截點是 `internal/grpc/di_test.go` 的
// `TestModule_RegistersInvoiceService`（用 `server.GetServiceInfo()` 把「註冊」
// 變成可觀察的事實）。⇒ **本測試的保證是「圖解得開」，不要把它讀成「接線都對」。**
func TestNew_WholeGraphResolves(t *testing.T) {
	opts := append([]fx.Option{fx.NopLogger}, modules()...)
	if err := fx.ValidateApp(opts...); err != nil {
		t.Fatalf("整張 fx 圖解不開（這個錯誤在補這支測試之前，只有叢集上的開機會發現）：%v", err)
	}
}
