// Package grpc 是 Layer 5：gRPC handler 註冊與型別邊界。
//
// 對外 ID／金額一律 string（§4）；string id → int64 的轉換在 handler，
// 時間走 timeutil.FormatISO8601。不含業務邏輯。
package grpc

import (
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"go.uber.org/fx"
	grpclib "google.golang.org/grpc"
)

// Module 註冊 InvoiceService 的 handler。
//
// ⚠️ 註冊是 fx.Invoke 不是 fx.Provide：Provide 只有在有人要那個型別時才會被呼叫，
// 而**沒有人依賴「已註冊」這件事**——服務照樣開得起來，只是每一支 RPC 都回
// Unimplemented，而 gRPC health check 仍然是 SERVING。整圖測試（internal/app/app_test.go）
// 驗得到 handler 在圖上，驗不到它有沒有被註冊進 server；後者的攔截點是本檔的 Invoke 本身。
var Module = fx.Module("grpc",
	fx.Provide(NewInvoiceServer),
	fx.Invoke(func(server *grpclib.Server, handler *InvoiceServer) {
		pb.RegisterInvoiceServiceServer(server, handler)
	}),
)
