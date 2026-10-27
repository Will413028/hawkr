package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// setupAdminNotificationTemplateRouter wires admin notification template routes
// using fakeJWTAuth — admin role check is enforced in production at the /admin
// group level, so handler-level tests bypass it.
func setupAdminNotificationTemplateRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1/admin")
	v1.Use(fakeJWTAuth())
	tmpl := v1.Group("/notification-templates")
	{
		tmpl.POST("/reload", h.AdminReloadTemplates)
		tmpl.GET("/:id", h.AdminGetTemplate)
		tmpl.PATCH("/:id/enabled", h.AdminUpdateTemplateEnabled)
		tmpl.POST("/:id/preview", h.AdminPreviewTemplate)
		tmpl.POST("/:id/test-send", h.AdminTestSendTemplate)
	}
	v1.POST("/notifications/broadcast", h.AdminBroadcastAnnouncement)
	return r
}

func newAdminNotifHolder(mc *mockNotificationServiceClient) *Holder {
	return &Holder{Notification: grpcclient.NewNotificationClientForTest(mc)}
}

func doAdminReq(r *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	var buf *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewBuffer(b)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "1")
	req.Header.Set("X-Account-Type", "4")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// ==================== AdminReloadTemplates ====================

func TestAdminReloadTemplates_Happy(t *testing.T) {
	mc := &mockNotificationServiceClient{
		ReloadTemplatesFn: func(ctx context.Context, in *pb.ReloadTemplatesRequest, opts ...grpc.CallOption) (*pb.ReloadTemplatesResponse, error) {
			return &pb.ReloadTemplatesResponse{Code: 0, ReloadedCount: "12"}, nil
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "POST", "/api/v1/admin/notification-templates/reload", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	data, _ := body["data"].(map[string]interface{})
	if got, _ := data["reloadedCount"].(string); got != "12" {
		t.Errorf("data.reloadedCount = %v, want 12; body = %v", data["reloadedCount"], body)
	}
}

func TestAdminReloadTemplates_NonZeroCode(t *testing.T) {
	mc := &mockNotificationServiceClient{
		ReloadTemplatesFn: func(ctx context.Context, in *pb.ReloadTemplatesRequest, opts ...grpc.CallOption) (*pb.ReloadTemplatesResponse, error) {
			return &pb.ReloadTemplatesResponse{Code: 5001}, nil
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "POST", "/api/v1/admin/notification-templates/reload", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	assertCode(t, parseJSON(t, w.Body), 5001)
}

func TestAdminReloadTemplates_GrpcError(t *testing.T) {
	mc := &mockNotificationServiceClient{
		ReloadTemplatesFn: func(ctx context.Context, in *pb.ReloadTemplatesRequest, opts ...grpc.CallOption) (*pb.ReloadTemplatesResponse, error) {
			return nil, grpcstatus.Error(grpccodes.Unavailable, "downstream down")
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "POST", "/api/v1/admin/notification-templates/reload", nil)
	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

// ==================== AdminGetTemplate ====================

func TestAdminGetTemplate_Happy(t *testing.T) {
	mc := &mockNotificationServiceClient{
		GetTemplateFn: func(ctx context.Context, in *pb.GetTemplateRequest, opts ...grpc.CallOption) (*pb.GetTemplateResponse, error) {
			if in.Id != "tpl-1" {
				t.Errorf("Id = %q, want tpl-1", in.Id)
			}
			return &pb.GetTemplateResponse{
				Code: 0,
				Template: &pb.NotificationTemplateProto{
					Id:            "tpl-1",
					EventType:     "order.created",
					TitleTemplate: "訂單 {{.orderNo}} 已成立",
					BodyTemplate:  "請於 24h 內付款",
					Enabled:       true,
				},
			}, nil
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "GET", "/api/v1/admin/notification-templates/tpl-1", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data not an object: %v", body["data"])
	}
	if id, _ := data["id"].(string); id != "tpl-1" {
		t.Errorf("data.id = %q, want tpl-1", id)
	}
}

func TestAdminGetTemplate_NonZeroCode(t *testing.T) {
	mc := &mockNotificationServiceClient{
		GetTemplateFn: func(ctx context.Context, in *pb.GetTemplateRequest, opts ...grpc.CallOption) (*pb.GetTemplateResponse, error) {
			return &pb.GetTemplateResponse{Code: 4004}, nil
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "GET", "/api/v1/admin/notification-templates/missing", nil)
	assertHTTPStatus(t, w, http.StatusOK)
	assertCode(t, parseJSON(t, w.Body), 4004)
}

func TestAdminGetTemplate_GrpcNotFound(t *testing.T) {
	mc := &mockNotificationServiceClient{
		GetTemplateFn: func(ctx context.Context, in *pb.GetTemplateRequest, opts ...grpc.CallOption) (*pb.GetTemplateResponse, error) {
			return nil, grpcstatus.Error(grpccodes.NotFound, "template not found")
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "GET", "/api/v1/admin/notification-templates/missing", nil)
	assertHTTPStatus(t, w, http.StatusNotFound)
}

// ==================== AdminUpdateTemplateEnabled ====================

func TestAdminUpdateTemplateEnabled_Enable(t *testing.T) {
	var captured *pb.UpdateTemplateEnabledRequest
	mc := &mockNotificationServiceClient{
		UpdateTemplateEnabledFn: func(ctx context.Context, in *pb.UpdateTemplateEnabledRequest, opts ...grpc.CallOption) (*pb.UpdateTemplateEnabledResponse, error) {
			captured = in
			return &pb.UpdateTemplateEnabledResponse{Code: 0}, nil
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "PATCH", "/api/v1/admin/notification-templates/tpl-1/enabled", map[string]bool{"enabled": true})
	assertHTTPStatus(t, w, http.StatusOK)
	assertCode(t, parseJSON(t, w.Body), 0)
	if captured == nil || captured.Id != "tpl-1" || !captured.Enabled {
		t.Errorf("captured = %+v, want Id=tpl-1 Enabled=true", captured)
	}
}

func TestAdminUpdateTemplateEnabled_Disable(t *testing.T) {
	var capturedEnabled bool
	mc := &mockNotificationServiceClient{
		UpdateTemplateEnabledFn: func(ctx context.Context, in *pb.UpdateTemplateEnabledRequest, opts ...grpc.CallOption) (*pb.UpdateTemplateEnabledResponse, error) {
			capturedEnabled = in.Enabled
			return &pb.UpdateTemplateEnabledResponse{Code: 0}, nil
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "PATCH", "/api/v1/admin/notification-templates/tpl-1/enabled", map[string]bool{"enabled": false})
	assertHTTPStatus(t, w, http.StatusOK)
	if capturedEnabled {
		t.Error("Enabled should be false")
	}
}

func TestAdminUpdateTemplateEnabled_GrpcError(t *testing.T) {
	mc := &mockNotificationServiceClient{
		UpdateTemplateEnabledFn: func(ctx context.Context, in *pb.UpdateTemplateEnabledRequest, opts ...grpc.CallOption) (*pb.UpdateTemplateEnabledResponse, error) {
			return nil, grpcstatus.Error(grpccodes.Internal, "boom")
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "PATCH", "/api/v1/admin/notification-templates/tpl-1/enabled", map[string]bool{"enabled": true})
	assertHTTPStatus(t, w, http.StatusInternalServerError)
}

// ==================== AdminPreviewTemplate ====================

func TestAdminPreviewTemplate_Happy(t *testing.T) {
	mc := &mockNotificationServiceClient{
		PreviewTemplateFn: func(ctx context.Context, in *pb.PreviewTemplateRequest, opts ...grpc.CallOption) (*pb.PreviewTemplateResponse, error) {
			if in.Id != "tpl-1" {
				t.Errorf("Id = %q, want tpl-1", in.Id)
			}
			return &pb.PreviewTemplateResponse{
				Code:                0,
				RenderedTitle:       "訂單 A123 已成立",
				RenderedBody:        "請付款",
				RenderedReferenceId: "A123",
			}, nil
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "POST", "/api/v1/admin/notification-templates/tpl-1/preview", map[string]string{
		"samplePayload": `{"orderNo":"A123"}`,
	})
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	data, _ := body["data"].(map[string]interface{})
	if title, _ := data["renderedTitle"].(string); title != "訂單 A123 已成立" {
		t.Errorf("data.renderedTitle = %q; body = %v", title, body)
	}
}

func TestAdminPreviewTemplate_MissingPayload(t *testing.T) {
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(&mockNotificationServiceClient{}))
	w := doAdminReq(r, "POST", "/api/v1/admin/notification-templates/tpl-1/preview", map[string]string{})
	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestAdminPreviewTemplate_NonZeroCode(t *testing.T) {
	mc := &mockNotificationServiceClient{
		PreviewTemplateFn: func(ctx context.Context, in *pb.PreviewTemplateRequest, opts ...grpc.CallOption) (*pb.PreviewTemplateResponse, error) {
			return &pb.PreviewTemplateResponse{Code: 4040}, nil
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "POST", "/api/v1/admin/notification-templates/tpl-1/preview", map[string]string{"samplePayload": "{}"})
	assertHTTPStatus(t, w, http.StatusOK)
	assertCode(t, parseJSON(t, w.Body), 4040)
}

// ==================== AdminTestSendTemplate ====================

func TestAdminTestSendTemplate_Happy(t *testing.T) {
	var captured *pb.TestSendTemplateRequest
	mc := &mockNotificationServiceClient{
		TestSendTemplateFn: func(ctx context.Context, in *pb.TestSendTemplateRequest, opts ...grpc.CallOption) (*pb.TestSendTemplateResponse, error) {
			captured = in
			return &pb.TestSendTemplateResponse{Code: 0}, nil
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "POST", "/api/v1/admin/notification-templates/tpl-1/test-send", map[string]string{
		"samplePayload":   "{}",
		"targetAccountId": "9999",
		"targetUserType":  "buyer",
	})
	assertHTTPStatus(t, w, http.StatusOK)
	assertCode(t, parseJSON(t, w.Body), 0)
	if captured == nil || captured.TargetUserId != "9999" || captured.TargetUserType != "buyer" {
		t.Errorf("captured = %+v", captured)
	}
}

func TestAdminTestSendTemplate_MissingTarget(t *testing.T) {
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(&mockNotificationServiceClient{}))
	w := doAdminReq(r, "POST", "/api/v1/admin/notification-templates/tpl-1/test-send", map[string]string{
		"samplePayload": "{}",
	})
	assertHTTPStatus(t, w, http.StatusBadRequest)
}

func TestAdminTestSendTemplate_GrpcError(t *testing.T) {
	mc := &mockNotificationServiceClient{
		TestSendTemplateFn: func(ctx context.Context, in *pb.TestSendTemplateRequest, opts ...grpc.CallOption) (*pb.TestSendTemplateResponse, error) {
			return nil, grpcstatus.Error(grpccodes.Unavailable, "FCM down")
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "POST", "/api/v1/admin/notification-templates/tpl-1/test-send", map[string]string{
		"samplePayload":   "{}",
		"targetAccountId": "9999",
	})
	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
}

// ==================== AdminBroadcastAnnouncement ====================

func TestAdminBroadcastAnnouncement_Happy(t *testing.T) {
	var captured *pb.BroadcastAnnouncementRequest
	mc := &mockNotificationServiceClient{
		BroadcastAnnouncementFn: func(ctx context.Context, in *pb.BroadcastAnnouncementRequest, opts ...grpc.CallOption) (*pb.BroadcastAnnouncementResponse, error) {
			captured = in
			return &pb.BroadcastAnnouncementResponse{
				Code:           0,
				Message:        "已推播給 100 位使用者",
				BroadcastId:    "7495464541774413824",
				RecipientCount: "100",
			}, nil
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "POST", "/api/v1/admin/notifications/broadcast", map[string]string{
		"title":          "系統維護",
		"body":           "今晚 12 點維護",
		"targetUserType": "buyer",
	})
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.TargetUserType != "buyer" || captured.Title != "系統維護" {
		t.Errorf("captured = %+v", captured)
	}

	// 公告識別與收件人數必須是 data 底下的具名欄位。它們原本只存在於下游的中文
	// message（"已推播給 N 位使用者"），broadcastId 則根本傳不出來——管理員發完公告
	// 之後查不到自己剛發的是哪一則。message 則恆為 "success"：Gateway 不轉發下游的
	// 成功文案，前端要值就讀 data，不是剖析字串。
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	if got := body["message"]; got != "success" {
		t.Errorf("message = %v, want \"success\"（Gateway 不得轉發下游成功文案）", got)
	}
	data, ok := body["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data 不是 object；body=%v", body)
	}
	if got := data["broadcastId"]; got != "7495464541774413824" {
		t.Errorf("data.broadcastId = %v, want 7495464541774413824", got)
	}
	if got := data["recipientCount"]; got != "100" {
		t.Errorf("data.recipientCount = %v, want \"100\"", got)
	}
}

func TestAdminBroadcastAnnouncement_DefaultTargetAll(t *testing.T) {
	var captured *pb.BroadcastAnnouncementRequest
	mc := &mockNotificationServiceClient{
		BroadcastAnnouncementFn: func(ctx context.Context, in *pb.BroadcastAnnouncementRequest, opts ...grpc.CallOption) (*pb.BroadcastAnnouncementResponse, error) {
			captured = in
			return &pb.BroadcastAnnouncementResponse{Code: 0}, nil
		},
	}
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(mc))
	w := doAdminReq(r, "POST", "/api/v1/admin/notifications/broadcast", map[string]string{
		"title": "T",
		"body":  "B",
	})
	assertHTTPStatus(t, w, http.StatusOK)
	if captured == nil || captured.TargetUserType != "all" {
		t.Errorf("TargetUserType = %q, want all (default)", captured.TargetUserType)
	}
}

func TestAdminBroadcastAnnouncement_MissingTitle(t *testing.T) {
	r := setupAdminNotificationTemplateRouter(newAdminNotifHolder(&mockNotificationServiceClient{}))
	w := doAdminReq(r, "POST", "/api/v1/admin/notifications/broadcast", map[string]string{
		"body": "no title",
	})
	assertHTTPStatus(t, w, http.StatusBadRequest)
}

// TestAdminTemplateDTO_CamelKeys locks the admin template DTO wire shape to
// camelCase. protoToDTO is a pure function — no gRPC server needed.
func TestAdminTemplateDTO_CamelKeys(t *testing.T) {
	dto := protoToDTO(&pb.NotificationTemplateProto{
		Id:                  "1",
		Exchange:            "ex",
		RoutingKey:          "order.paid",
		EventType:           "order.paid",
		TargetUserType:      "buyer",
		TargetUserField:     "buyerAccountId",
		Category:            "order_update",
		TitleTemplate:       "t",
		BodyTemplate:        "b",
		BodyFallback:        "bf",
		DataType:            "order_paid",
		DataFields:          map[string]*pb.FieldSpec{"order_id": {Spec: &pb.FieldSpec_Field{Field: "orderNo"}}},
		SchemaUrl:           "u",
		ReferenceIdTemplate: "#{orderNo}",
		Enabled:             true,
		UpdatedBy:           "9",
		CreatedAt:           "2026-06-25T00:00:00Z",
		UpdatedAt:           "2026-06-25T00:00:00Z",
	})
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(b)

	for _, key := range []string{`"dataFields"`, `"routingKey"`, `"eventType"`,
		`"targetUserType"`, `"targetUserField"`, `"titleTemplate"`, `"bodyTemplate"`,
		`"bodyFallback"`, `"dataType"`, `"schemaUrl"`, `"referenceIdTemplate"`,
		`"createdAt"`, `"updatedAt"`, `"updatedBy"`} {
		if !strings.Contains(got, key) {
			t.Errorf("missing camel key %s; got %s", key, got)
		}
	}
	for _, snake := range []string{`"routing_key"`, `"queue_name"`, `"event_type"`, `"target_user_type"`,
		`"target_user_field"`, `"title_template"`, `"body_template"`, `"body_fallback"`, `"data_type"`,
		`"schema_url"`, `"reference_id_template"`, `"fcm_topic"`, `"created_at"`, `"updated_at"`,
		`"updated_by"`, `"data_fields"`} {
		if strings.Contains(got, snake) {
			t.Errorf("snake key %s leaked; got %s", snake, got)
		}
	}
	// data_fields inner map keys are MQ payload field refs — must NOT be camelized.
	if !strings.Contains(got, `"order_id"`) {
		t.Errorf("inner data_fields key order_id was wrongly altered; got %s", got)
	}
}
