package usecase

import (
	"testing"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/client/media"
	"github.com/Will413028/hawkr/invoice-service/internal/config"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// TestModule_GraphResolves 驗 fx 圖能解開（形狀照 repository/postgres/di_test.go）。
//
// fx 的接線錯誤只有**開機時**才會發現，而那時已經在叢集裡了。編譯期的
// `var _ domain.X = (*Y)(nil)` 擋得住「沒實作介面」，擋不住「圖裡少一個型別」。
//
// Layer 0–3 的產物用 fx.Supply 頂替：驗的是本 Module 自己的圖。
func TestModule_GraphResolves(t *testing.T) {
	cfg := &config.Config{}
	cfg.Schedule.IssueMaxRetry = 13
	cfg.Schedule.BackoffBaseSeconds = 30
	cfg.Schedule.BackoffMaxSeconds = 1800
	cfg.Schedule.IssueLeaseSeconds = 120

	if err := fx.ValidateApp(
		fx.NopLogger,
		fx.Supply(cfg, zap.NewNop()),
		fx.Provide(
			func() database.TxManager { return directTx{} },
			func() amego.Client { return nil },
			func() domain.IssuerRepository { return nil },
			func() domain.SellerTaxProfileRepository { return nil },
			func() domain.ProviderAccountRepository { return nil },
			func() domain.DocumentRepository { return nil },
			func() domain.OperationRepository { return nil },
			func() domain.CorrectionRequestRepository { return nil },
			func() domain.OutboxRepository { return nil },
			func() domain.AuditRepository { return nil },
			func() domain.BuyerPreferenceRepository { return nil },
			func() *media.Client { return nil },
		),
		Module,
		fx.Invoke(func(*Issuance) {}),
		fx.Invoke(func(*Checkout) {}),
		fx.Invoke(func(*TaxProfile) {}),
	); err != nil {
		t.Fatalf("fx graph 解不開：%v", err)
	}
}
