package handler_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/bizcarrier"
	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/gin-gonic/gin"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// 繞道 ⑨ 的第二種形態：**把 handler 搬到別的 package**。
//
// 舊 AST gate 的判定材料是 `internal/handler/*.go` 的 AST —— 它只掃自己那一個目錄。
// 把 handler 搬到任何其他 package（新的 internal/xxxhandler、既有的 proxy、甚至只是
// 換成 external test package）之後，gate 連檔案都看不到，報零 finding。
//
// 本檔刻意宣告成 `package handler_test`：它是一個**與 handler 不同的 package**，
// 只能碰得到匯出 API，也因此完全複製了「handler 住在別處」的條件。而斷言仍然成立，
// 因為判定發生在 internal/response —— 誰呼叫它、從哪個 package 呼叫它，都不影響。
//
// 一併證明的第二件事：這條路徑上沒有任何 handler package 的內部知識參與，所以
// 「gate 依賴 Holder.Invoice 這個欄位名／依賴 <recv>.Invoice.<M> 這個呼叫語法」
// （舊 gate 的 invoiceRPCName 在原始碼註解裡自己承認的失明點）也一併失效。

type externalProbeServer struct {
	pb.UnimplementedInvoiceServiceServer
}

func (s *externalProbeServer) HealthCheckProviderAccountForAdmin(_ context.Context, _ *pb.HealthCheckProviderAccountForAdminRequest) (*pb.GetProviderAccountForAdminResponse, error) {
	return &pb.GetProviderAccountForAdminResponse{
		Code:    9331,
		Message: "憑證驗證失敗",
		Data: &pb.SellerProviderAccountData{
			Provider: "AMEGO", Status: "CREDENTIAL_INVALID", LastRequestId: "req-external-001",
		},
	}, nil
}

func TestBypass10_HandlerInAnotherPackage(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := gogrpc.NewServer()
	pb.RegisterInvoiceServiceServer(srv, &externalProbeServer{})
	go srv.Serve(lis) //nolint:errcheck

	conn, err := gogrpc.NewClient("passthrough:///bufnet",
		gogrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
		gogrpc.WithChainUnaryInterceptor(grpcclient.EnvelopeGuardInterceptor()))
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(); srv.Stop() })

	h := &handler.Holder{Invoice: grpcclient.NewInvoiceServiceClientForTest(pb.NewInvoiceServiceClient(conn))}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(bizcarrier.Middleware())
	// 完全沒有守門的 handler，住在 handler package 之外。
	r.GET("/probe", func(c *gin.Context) {
		resp, rpcErr := h.Invoice.HealthCheckProviderAccountForAdmin(c.Request.Context(),
			&pb.HealthCheckProviderAccountForAdminRequest{ActorAccountId: 1, StoreId: "2685914682"})
		if rpcErr != nil {
			response.GrpcErr(c, "invoice", rpcErr)
			return
		}
		response.OK(c, map[string]string{
			"provider":      resp.GetData().GetProvider(),
			"status":        resp.GetData().GetStatus(),
			"lastRequestId": resp.GetData().GetLastRequestId(),
		})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want 200；body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if code, _ := body["code"].(float64); int32(code) != 9331 {
		t.Fatalf("envelope code = %v, want 9331 —— 換個 package 就失明的話，這裡會是 0；body=%v",
			body["code"], body)
	}
	data, ok := body["data"].(map[string]any)
	if !ok || data["lastRequestId"] != "req-external-001" {
		t.Errorf("data 必須原樣透傳，得到 %#v", body["data"])
	}
}
