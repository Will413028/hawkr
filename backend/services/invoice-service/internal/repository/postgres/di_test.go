package postgres

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/Will413028/hawkr/go-pkg/crypto"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
)

// TestModule_GraphResolves 驗 fx 圖能解開。
//
// 為什麼需要它：fx 的接線錯誤（少一個 provider、fx.As 綁到沒實作的介面、型別撞名）
// **只有開機時才會發現**，而那時已經在叢集裡了。編譯期的 `var _ domain.X = (*Y)(nil)`
// 只擋得住「沒實作介面」，擋不住「圖裡少一個型別」。
//
// 三個相依（Config / Cipher / Pool）由 config.Module 提供，這裡用 fx.Supply 頂替——
// 驗的是本 Module 自己的圖，不是 Layer 0。
func TestModule_GraphResolves(t *testing.T) {
	cfg := &config.Config{}
	cfg.Encryption.MasterKeyVersion = 1
	cfg.Encryption.LookupHMACKey = "lookup"

	cipher, err := crypto.New("master")
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}

	if err := fx.ValidateApp(
		fx.NopLogger,
		fx.Supply(cfg, cipher, (*pgxpool.Pool)(nil)),
		Module,
		// 把每個介面都要一次，否則 fx 只驗「有沒有人要」而不會實際解析它們。
		fx.Invoke(func(RepositorySet) {}),
	); err != nil {
		t.Fatalf("fx graph 解不開：%v", err)
	}
}
