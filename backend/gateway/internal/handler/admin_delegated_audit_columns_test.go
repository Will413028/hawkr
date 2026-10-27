package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	accountpb "github.com/Will413028/hawkr/go-pkg/proto/account"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 代行資源端點的 request ID 與 correlation ID 應與 tracing 使用相同的值。
func delegatedSessionForAuditColumns() *accountpb.OperationSessionSummary {
	return &accountpb.OperationSessionSummary{
		SessionId: "2701445580000001",
		Actor:     &accountpb.DelegationActor{AdminId: "2701338845", DisplayName: "Cora Chen"},
		Target:    &accountpb.DelegationTarget{StoreId: "2685914682"},
	}
}

// 沒有任何 inbound trace header 的請求（＝代行工作區的每一個請求）仍必須拿到非空的
// requestId／correlationId，而且必須是 TracingMiddleware 決定的那一個 —— 用回應 header
// 對拍，因為那正是操作者回報問題時手上唯一的線索。
func TestResolveDelegatedRequest_TraceColumnsComeFromTracingNotInboundHeader(t *testing.T) {
	shutdown := middleware.InitTracer(zap.NewNop())
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.TracingMiddleware())

	var got *delegatedRequest
	r.POST("/t/:store_id", func(c *gin.Context) {
		middleware.SetDelegationSession(c, delegatedSessionForAuditColumns())
		req, ok := resolveDelegatedRequest(c, false)
		if !ok {
			t.Errorf("resolveDelegatedRequest 應成功，實際被拒（HTTP %d）", c.Writer.Status())
			return
		}
		got = req
	})

	req := httptest.NewRequest(http.MethodPost, "/t/2685914682", strings.NewReader("{}"))
	// 刻意不設 X-Request-Id／x-trace-id：這正是 production 的形狀。
	req.RemoteAddr = "203.0.113.9:51234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got == nil {
		t.Fatalf("handler 未取得 delegatedRequest（HTTP %d）", w.Code)
	}

	if got.RequestID == "" {
		t.Fatalf("requestId 為空 —— 讀的是 inbound header 而不是 TracingMiddleware 算好的值，" +
			"稽核列會全表空字串")
	}
	if echoed := w.Header().Get("x-request-id"); got.RequestID != echoed {
		t.Fatalf("requestId=%q 與回應 header x-request-id=%q 不一致 —— 稽核列接不回 log", got.RequestID, echoed)
	}
	if got.CorrelationID == "" {
		t.Fatalf("correlationId 為空")
	}
	if echoed := w.Header().Get("x-trace-id"); got.CorrelationID != echoed {
		t.Fatalf("correlationId=%q 與回應 header x-trace-id=%q 不一致", got.CorrelationID, echoed)
	}
}

// 相反方向：呼叫端真的送了 trace header（curl／腳本直打 gateway）時仍必須沿用它，
// 否則「同一次請求在 gateway 與稽核列上是同一個 id」這件事在該路徑上不成立。
func TestResolveDelegatedRequest_KeepsInboundTraceHeadersWhenPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/t/2685914682", strings.NewReader("{}"))
	c.Request.Header.Set("X-Request-Id", "req-from-caller")
	c.Request.Header.Set("x-trace-id", "trace-from-caller")
	c.Params = gin.Params{{Key: "store_id", Value: "2685914682"}}
	middleware.SetDelegationSession(c, delegatedSessionForAuditColumns())

	got, ok := resolveDelegatedRequest(c, false)
	if !ok {
		t.Fatalf("resolveDelegatedRequest 應成功（HTTP %d）", w.Code)
	}
	if got.RequestID != "req-from-caller" {
		t.Fatalf("requestId=%q，want req-from-caller（context 未設值時應退回 header）", got.RequestID)
	}
	if got.CorrelationID != "trace-from-caller" {
		t.Fatalf("correlationId=%q，want trace-from-caller", got.CorrelationID)
	}
}
