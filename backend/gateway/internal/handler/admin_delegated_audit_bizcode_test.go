package handler

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/gin-gonic/gin"
	gogrpc "google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// 稽核與摘要三支端點的業務碼還原（A-4）。
//
// 這是同一個 bug class 的第二次發作：第一次修的是 admin_delegation.go（見
// TestDelegationGrpcFailure_RestoresBizCode 的註解），而 admin_delegated_audit.go
// 從未一起轉換，於是「sessionId 查無」在這三支上仍舊塌成 424／9280 —— 前端據此走
// 「代行服務暫時不可用，請稍後再試」，而稍後再試永遠不會成功，因為那個工作階段本來
// 就不存在。正解是 410／9277「找不到指定的代行工作階段」。
//
// ⚠️ **不能用 grpcclient.NewAccountDelegationClientForTest 注 mock 來測這件事**：
// 那條路沒有 *grpc.ClientConn，EnvelopeGuardInterceptor 根本不會跑，mock 回的
// {Code: 9277} 會原樣抵達 handler 並走進 `resp.GetCode() != 0` 那條分支 —— 修不修
// 都綠（#2050 就是這樣讓一個真實安全漏洞全綠出廠的）。所以這裡起一台 bufconn 上的
// 假 DelegationService，讓真的 interceptor 把「gRPC OK ＋ 非零 envelope code」轉成
// FailedPrecondition ＋ "[9277] " 前綴，handler 收到的形狀與 production 逐位元相同。
//
// 每個端點都配一筆 codes.Unavailable：少了它，「把所有 err 一律翻成 410」也會全綠，
// 而那是另一個方向的同一個 bug（真的上游故障被講成「工作階段不存在」，操作者不會重試）。

type delegationAuditFakeServer struct {
	accountpb.UnimplementedDelegationServiceServer
	// envelopeCode 非零時回 gRPC OK ＋ 該 envelope code，鏡射 account-service 的
	// bizOf(err) 分支（internal/grpc/delegation.go）：業務拒絕從來不是 gRPC error。
	envelopeCode int32
	envelopeMsg  string
	// transportErr 非 nil 時直接回它，鏡射真正問不到答案的那一側。
	transportErr error
	summaryData  *accountpb.SessionSummaryData
}

func (s *delegationAuditFakeServer) ListAuditLogs(_ context.Context, _ *accountpb.ListAuditLogsRequest) (*accountpb.ListAuditLogsResponse, error) {
	if s.transportErr != nil {
		return nil, s.transportErr
	}
	return &accountpb.ListAuditLogsResponse{Code: s.envelopeCode, Message: s.envelopeMsg}, nil
}

func (s *delegationAuditFakeServer) GetSessionSummary(_ context.Context, _ *accountpb.GetSessionSummaryRequest) (*accountpb.GetSessionSummaryResponse, error) {
	if s.transportErr != nil {
		return nil, s.transportErr
	}
	return &accountpb.GetSessionSummaryResponse{Code: s.envelopeCode, Message: s.envelopeMsg, Data: s.summaryData}, nil
}

func TestDelegatedAdminSummaryReturnsFullSessionWithoutCredential(t *testing.T) {
	session := sampleSessionSummary()
	h := newDelegationAuditHolder(t, &delegationAuditFakeServer{summaryData: &accountpb.SessionSummaryData{
		SessionId: session.GetSessionId(), Session: session,
	}})
	w, c := newDelegationTestContext(t, http.MethodGet, "/api/v1/admin/delegated-operations/2701445580000001/summary", "")
	c.Params = gin.Params{{Key: "session_id", Value: session.GetSessionId()}}
	h.GetDelegatedSessionSummary(c)
	if w.Code != http.StatusOK {
		t.Fatalf("HTTP = %d, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data struct {
			Session DelegationSessionData `json:"session"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Session.SessionID != session.GetSessionId() || body.Data.Session.Reason.Code != session.GetReason().GetCode() ||
		len(body.Data.Session.Capabilities) != len(session.GetCapabilities()) {
		t.Fatalf("session = %+v", body.Data.Session)
	}
	if strings.Contains(w.Body.String(), "operationCredential") {
		t.Fatalf("admin summary 不得含 credential：%s", w.Body.String())
	}
}

func (s *delegationAuditFakeServer) GetCurrentSession(_ context.Context, _ *accountpb.GetCurrentSessionRequest) (*accountpb.GetCurrentSessionResponse, error) {
	if s.transportErr != nil {
		return nil, s.transportErr
	}
	return &accountpb.GetCurrentSessionResponse{Code: s.envelopeCode, Message: s.envelopeMsg}, nil
}

// newDelegationAuditHolder 起 bufconn 假 server，並以掛真 EnvelopeGuardInterceptor 的
// 連線建 AccountClient —— 那正是 production DefaultDialOpts 中 envelope guard 的位置。
// 其餘 auth／breaker interceptor 與本主張無關故不掛（breaker 的 registry 是以
// cc.Target() 為鍵的全域 sync.Map，掛上去會讓幾個 subtest 共用同一顆計數器）。
func newDelegationAuditHolder(t *testing.T, srv accountpb.DelegationServiceServer) *Holder {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := gogrpc.NewServer()
	accountpb.RegisterDelegationServiceServer(grpcSrv, srv)
	go grpcSrv.Serve(lis) //nolint:errcheck

	conn, err := gogrpc.NewClient("passthrough:///bufnet",
		gogrpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		gogrpc.WithTransportCredentials(insecure.NewCredentials()),
		gogrpc.WithChainUnaryInterceptor(grpcclient.EnvelopeGuardInterceptor()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		grpcSrv.Stop()
	})
	return &Holder{Account: grpcclient.NewAccountDelegationClientForTest(accountpb.NewDelegationServiceClient(conn))}
}

// delegationAuditEndpoints 是本 class 命中的三支。invoke 只負責把 handler 需要的
// 路徑參數／header 補齊，其餘一律共用。
var delegationAuditEndpoints = []struct {
	name   string
	invoke func(h *Holder, c *gin.Context)
	prep   func(c *gin.Context)
}{
	{
		name:   "GET {sessionId}/audit-logs",
		invoke: (*Holder).ListDelegatedAuditLogs,
		prep: func(c *gin.Context) {
			c.Params = gin.Params{{Key: "session_id", Value: "2701445580000001"}}
		},
	},
	{
		name:   "GET {sessionId}/summary",
		invoke: (*Holder).GetDelegatedSessionSummary,
		prep: func(c *gin.Context) {
			c.Params = gin.Params{{Key: "session_id", Value: "2701445580000001"}}
		},
	},
	{
		name:   "GET current/summary",
		invoke: (*Holder).GetCurrentDelegatedSessionSummary,
		prep: func(c *gin.Context) {
			c.Request.Header.Set(headerDelegationOperationCredential, "dops_test_credential")
		},
	},
}

// 查無工作階段（account-service 回 9277）必須還原成 410／9277，不得塌成 424／9280。
func TestDelegatedAuditEndpoints_RestoreSessionNotFound(t *testing.T) {
	for _, ep := range delegationAuditEndpoints {
		t.Run(ep.name, func(t *testing.T) {
			h := newDelegationAuditHolder(t, &delegationAuditFakeServer{
				envelopeCode: bizcode.DelegationSessionNotFound,
				envelopeMsg:  "找不到指定的代行工作階段",
			})
			w, c := newDelegationTestContext(t, http.MethodGet, "/api/v1/admin/delegated-operations/x", "")
			ep.prep(c)
			ep.invoke(h, c)

			if w.Code != http.StatusGone {
				t.Fatalf("HTTP 應為 410，實際 %d（body=%s）", w.Code, w.Body.String())
			}
			code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
			if code != bizcode.DelegationSessionNotFound {
				t.Fatalf("業務碼應為 %d，實際 %d", bizcode.DelegationSessionNotFound, code)
			}
		})
	}
}

// current/summary 的第二個症狀：憑證解不開是 9274／401，同樣不得塌成 9280。
// 這一條與上面那條是同根因的兩個出口，原報告只測到 audit-logs 那一個。
func TestDelegatedCurrentSummary_RestoresCredentialInvalid(t *testing.T) {
	h := newDelegationAuditHolder(t, &delegationAuditFakeServer{
		envelopeCode: bizcode.DelegationCredentialInvalid,
		envelopeMsg:  "代行憑證無效，請重新建立工作階段",
	})
	w, c := newDelegationTestContext(t, http.MethodGet, "/api/v1/admin/delegated-operations/current/summary", "")
	c.Request.Header.Set(headerDelegationOperationCredential, "dops_test_credential")
	h.GetCurrentDelegatedSessionSummary(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("HTTP 應為 401，實際 %d（body=%s）", w.Code, w.Body.String())
	}
	code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
	if code != bizcode.DelegationCredentialInvalid {
		t.Fatalf("業務碼應為 %d，實際 %d", bizcode.DelegationCredentialInvalid, code)
	}
}

// 反向的鑑別力：真正的傳輸層失敗仍舊是 424／9280。少了這幾筆，「把所有 err 一律翻成
// 410」也會讓上面兩支測試全綠 —— 而那會讓一次真實的上游故障被講成「工作階段不存在」，
// 操作者不但不會重試，還會以為自己的代行已經被撤銷。
func TestDelegatedAuditEndpoints_TransportFailureStaysUpstreamUnavailable(t *testing.T) {
	for _, ep := range delegationAuditEndpoints {
		t.Run(ep.name, func(t *testing.T) {
			h := newDelegationAuditHolder(t, &delegationAuditFakeServer{
				transportErr: grpcstatus.Error(grpccodes.Unavailable, "connection refused"),
			})
			w, c := newDelegationTestContext(t, http.MethodGet, "/api/v1/admin/delegated-operations/x", "")
			ep.prep(c)
			ep.invoke(h, c)

			if w.Code != http.StatusFailedDependency {
				t.Fatalf("HTTP 應為 424，實際 %d（body=%s）", w.Code, w.Body.String())
			}
			code, _ := decodeDelegationEnvelope(t, w.Body.Bytes())
			if code != bizcode.DelegationUpstreamUnavailable {
				t.Fatalf("業務碼應為 %d，實際 %d", bizcode.DelegationUpstreamUnavailable, code)
			}
		})
	}
}
