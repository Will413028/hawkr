package client

import (
	"testing"

	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/client/seller"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// TestModule_GraphResolves 驗 Layer 3 的 fx 圖解得開，**含還沒有 consumer 的
// SEL peer client**。
//
// ⚠️ ValidateApp 不呼叫任何建構函式 ⇒ 它不會真的撥號、也不會讀 config 的值；
// 它驗的是「圖的形狀」。`fx.Invoke` 明列 *seller.Client 是刻意的：沒有這一行，
// 一個沒有 consumer 的 fx.Provide 在圖上等同不存在，接線壞掉也不會有人發現。
func TestModule_GraphResolves(t *testing.T) {
	if err := fx.ValidateApp(
		fx.NopLogger,
		fx.Supply(&config.Config{}, zap.NewNop()),
		// AMEGO client 需要 AuditRepository 才能把每次往返落 invoice_provider_audit_logs
		// （postgres.Module 提供；本測試只驗 Layer 3 的圖，供 nil 讓型別對得上即可）。
		fx.Provide(func() domain.AuditRepository { return nil }),
		Module,
		fx.Invoke(func(amego.Client, *seller.Client) {}),
	); err != nil {
		t.Fatalf("fx graph 解不開：%v", err)
	}
}
