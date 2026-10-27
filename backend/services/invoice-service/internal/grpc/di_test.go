package grpc

import (
	"testing"

	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
	grpclib "google.golang.org/grpc"
)

// serviceFullName 是 proto 契約上的完整服務名。寫死字面值是刻意的：
// 它就是註冊到 gRPC server 上的那把鍵，改了 proto package 就該讓這支測試轉紅。
const serviceFullName = "hawkr.invoice.InvoiceService"

// TestModule_RegistersInvoiceService 驗 handler **真的被註冊進 server**。
//
// 🔴 為什麼 internal/app 的整圖測試不夠：fx.ValidateApp 只驗「每個依賴都有人提供」。
// 把本 package 的 Module 整個掏空之後，**沒有任何東西依賴「已註冊」這件事** ⇒ 圖照樣解得開、
// app_test 照樣綠，而服務起來之後每一支 RPC 都回 Unimplemented，gRPC health check
// 仍然是 SERVING。實測確認過這個盲點：拿掉整個 Module 內容，app_test 不會轉紅。
//
// 這支測試把「註冊」變成可觀察的事實（GetServiceInfo），所以它抓得到那個形狀。
func TestModule_RegistersInvoiceService(t *testing.T) {
	server := grpclib.NewServer()

	app := fx.New(
		fx.NopLogger,
		fx.Supply(server, zap.NewNop()),
		fx.Provide(func() *usecase.Checkout { return nil }),
		fx.Provide(func() *usecase.Query { return nil }),
		fx.Provide(func() *usecase.Preference { return nil }),
		fx.Provide(func() *usecase.Complaint { return nil }),
		fx.Provide(func() *usecase.TaxProfile { return nil }),
		fx.Provide(func() *usecase.AdminJob { return nil }),
		fx.Provide(func() *usecase.AdminIssuance { return nil }),
		fx.Provide(func() *usecase.AdminComplaint { return nil }),
		fx.Provide(func() *usecase.ProviderAccount { return nil }),
		fx.Provide(func() *usecase.SellerComplaint { return nil }),
		fx.Provide(func() *usecase.SellerJob { return nil }),
		fx.Provide(func() *usecase.CorrectionIntake { return nil }),
		fx.Provide(func() *usecase.NotificationResend { return nil }),
		fx.Provide(func() *usecase.DocumentFile { return nil }),
		fx.Provide(func() SellerNameResolver { return nil }),
		fx.Provide(func() ShipmentStatusResolver { return nil }),
		fx.Provide(func() BuyerNameResolver { return nil }),
		fx.Provide(func() EvidenceMetadataResolver { return nil }),
		Module,
	)
	if err := app.Err(); err != nil {
		t.Fatalf("grpc Module 的 fx 圖解不開：%v", err)
	}

	if _, ok := server.GetServiceInfo()[serviceFullName]; !ok {
		t.Fatalf("%s 沒有被註冊進 gRPC server —— 服務會正常開機，而每一支 RPC 都回 Unimplemented",
			serviceFullName)
	}
}
