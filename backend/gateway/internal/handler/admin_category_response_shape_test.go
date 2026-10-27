package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// assertMessageNoMsg verifies an emitter has converged to the canonical envelope:
// it carries the "message" field and no legacy "msg" field.
func assertMessageNoMsg(t *testing.T, body map[string]interface{}) {
	t.Helper()
	if _, ok := body["message"]; !ok {
		t.Errorf("missing 'message'; body=%v", body)
	}
	if _, ok := body["msg"]; ok {
		t.Errorf("legacy 'msg' still present; body=%v", body)
	}
}

// TestCategoryEmitter_MessageShape exercises a converted category emitter via an
// early-return validation path (no Category gRPC client needed). It must emit
// "message" (not "msg") at the original HTTP status.
func TestCategoryEmitter_MessageShape(t *testing.T) {
	r := gin.New()
	v1 := r.Group("/api/v1")
	// GetCategoryByID with empty id → response.BadRequest (400).
	v1.GET("/public/categories/:id", (&Holder{}).GetCategoryByID)
	// CreateCategory without auth → response.Err(401) with verbatim message.
	v1.POST("/app/categories", (&Holder{}).CreateCategory)

	t.Run("BadRequest message shape", func(t *testing.T) {
		// Route param ":id" can't be empty via path, so hit the param-empty branch
		// by registering a no-param variant inline.
		rr := gin.New()
		rr.GET("/c", func(c *gin.Context) { (&Holder{}).GetCategoryByID(c) })
		req := httptest.NewRequest("GET", "/c", nil)
		w := httptest.NewRecorder()
		rr.ServeHTTP(w, req)
		assertHTTPStatus(t, w, http.StatusBadRequest)
		body := parseJSON(t, w.Body)
		assertCode(t, body, 400)
		assertMessageNoMsg(t, body)
	})

	t.Run("Unauthorized preserves verbatim message at 401", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/app/categories", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assertHTTPStatus(t, w, http.StatusUnauthorized)
		body := parseJSON(t, w.Body)
		assertCode(t, body, 401)
		assertMessageNoMsg(t, body)
		if msg, _ := body["message"].(string); msg != "未授權：缺少用戶身份" {
			t.Errorf("message = %q, want verbatim 未授權：缺少用戶身份", msg)
		}
	})
}

// TestAdminNotificationEmitter_MessageShape covers a converted notification-template
// emitter: invalid-id → BadRequest carries "message", not "msg".
func TestAdminNotificationEmitter_MessageShape(t *testing.T) {
	r := gin.New()
	r.GET("/t", func(c *gin.Context) { (&Holder{}).AdminGetTemplate(c) }) // empty :id param
	req := httptest.NewRequest("GET", "/t", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusBadRequest)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 400)
	assertMessageNoMsg(t, body)
}

// TestAdminNotificationEmitter_BizCodePreserved verifies a downstream passthrough:
// when the notification service returns a non-zero business code, the gateway
// forwards that exact code at HTTP 200 (GrpcResult), not a clobbered 0 / 4xx.
func TestAdminNotificationEmitter_BizCodePreserved(t *testing.T) {
	mc := &shapeNotifMock{
		getTemplate: func(ctx context.Context, in *pb.GetTemplateRequest, opts ...grpc.CallOption) (*pb.GetTemplateResponse, error) {
			return &pb.GetTemplateResponse{Code: 4004}, nil
		},
	}
	h := &Holder{Notification: grpcclient.NewNotificationClientForTest(mc)}
	r := gin.New()
	r.GET("/api/v1/admin/notification-templates/:id", h.AdminGetTemplate)
	req := httptest.NewRequest("GET", "/api/v1/admin/notification-templates/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Biz-code preservation: non-zero code MUST stay at HTTP 200, not become 4xx.
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 4004)
	if _, ok := body["msg"]; ok {
		t.Errorf("legacy 'msg' still present; body=%v", body)
	}
}

// shapeNotifMock is a locally-named NotificationServiceClient mock to avoid
// colliding with the shared mockNotificationServiceClient in this package.
type shapeNotifMock struct {
	pb.NotificationServiceClient
	getTemplate func(ctx context.Context, in *pb.GetTemplateRequest, opts ...grpc.CallOption) (*pb.GetTemplateResponse, error)
}

func (m *shapeNotifMock) GetTemplate(ctx context.Context, in *pb.GetTemplateRequest, opts ...grpc.CallOption) (*pb.GetTemplateResponse, error) {
	return m.getTemplate(ctx, in, opts...)
}
