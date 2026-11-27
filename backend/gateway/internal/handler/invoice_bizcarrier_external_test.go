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
