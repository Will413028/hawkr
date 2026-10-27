package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const (
	testVendorSessionID = "6f1c3a52-0d7e-4c55-9d52-3a2f4f0f9a10"
	testVendorVerified  = "2026-10-04T08:30:00Z"
)

type vendorPortalRecordingClient struct {
	aipb.AIProcurementServiceClient

	rfqCalls      int
	submitCalls   []*aipb.SubmitVendorPortalResponseRequest
	withdrawCalls []*aipb.WithdrawVendorPortalResponseRequest
	declineCalls  []*aipb.DeclineVendorPortalRfqPublicationRequest
}

func (c *vendorPortalRecordingClient) GetRfqPublicationInviteSellerView(
	_ context.Context, _ *aipb.GetRfqPublicationInviteSellerViewRequest, _ ...grpc.CallOption,
) (*aipb.RfqPublicationInviteSellerViewResponse, error) {
	c.rfqCalls++
	return &aipb.RfqPublicationInviteSellerViewResponse{Item: "APPLE"}, nil
}

func (c *vendorPortalRecordingClient) SubmitVendorPortalResponse(
	_ context.Context, in *aipb.SubmitVendorPortalResponseRequest, _ ...grpc.CallOption,
) (*aipb.SellerResponseVersionResponse, error) {
	c.submitCalls = append(c.submitCalls, in)
	return &aipb.SellerResponseVersionResponse{}, nil
}

func (c *vendorPortalRecordingClient) WithdrawVendorPortalResponse(
	_ context.Context, in *aipb.WithdrawVendorPortalResponseRequest, _ ...grpc.CallOption,
) (*aipb.SellerResponseClosureResponse, error) {
	c.withdrawCalls = append(c.withdrawCalls, in)
	return &aipb.SellerResponseClosureResponse{}, nil
}

func (c *vendorPortalRecordingClient) DeclineVendorPortalRfqPublication(
	_ context.Context, in *aipb.DeclineVendorPortalRfqPublicationRequest, _ ...grpc.CallOption,
) (*aipb.SellerResponseClosureResponse, error) {
	c.declineCalls = append(c.declineCalls, in)
	return &aipb.SellerResponseClosureResponse{}, nil
}

// vendorPortalTestRouter 掛的路由與 router/routes_vendor_portal.go 相同的 middleware 順序。
func vendorPortalTestRouter(mock *vendorPortalRecordingClient) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &Holder{AIProcurement: grpcclient.NewAIProcurementClientForTest(mock)}
	r := gin.New()
	vp := r.Group("/vp")
	vp.GET("/rfqs/:rfq_recipient_id", middleware.VendorPortalSession(), h.VendorPortalRFQ)
	vp.POST("/rfqs/:rfq_recipient_id/response_submissions",
		middleware.VendorPortalSession(), middleware.RequireVendorPortalGoogleVerified(), h.VendorPortalSubmitResponse)
	vp.POST("/rfqs/:rfq_recipient_id/seller_responses/:seller_response_id/withdrawals",
		middleware.VendorPortalSession(), middleware.RequireVendorPortalGoogleVerified(), h.VendorPortalWithdrawResponse)
	vp.POST("/rfqs/:rfq_recipient_id/declines",
		middleware.VendorPortalSession(), middleware.RequireVendorPortalGoogleVerified(), h.VendorPortalDeclineRfq)
	return r
}

func vendorPortalRequest(method, path, body string, verified bool) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idem-1")
	req.Header.Set(middleware.VendorPortalRecipientIDHeader, "111")
	req.Header.Set(middleware.VendorPortalSessionIDHeader, testVendorSessionID)
	if verified {
		req.Header.Set(middleware.VendorPortalVerificationMethodHeader, "GOOGLE")
		req.Header.Set(middleware.VendorPortalVerifiedEmailHeader, "vendor@example.com")
		req.Header.Set(middleware.VendorPortalVerifiedAtHeader, testVendorVerified)
	}
	return req
}

func serve(r *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// session 綁收件人 A、請求 RFQ B ⇒ 404（⛔ 不是 403：403 會洩漏該 id 存在，
// doc 64 §9.3）。
func TestVendorPortalRFQ_WrongRecipientIs404(t *testing.T) {
	mock := &vendorPortalRecordingClient{}
	rec := serve(vendorPortalTestRouter(mock), vendorPortalRequest(http.MethodGet, "/vp/rfqs/222", "", false))

	require.Equal(t, http.StatusNotFound, rec.Code)
	body := rec.Body.String()
	require.NotContains(t, body, "forbidden")
	require.NotContains(t, body, "222", "回應不得回述那個不屬於你的 id")
	require.Zero(t, mock.rfqCalls)
}

func TestVendorPortalRFQ_BoundRecipientReadsTheView(t *testing.T) {
	mock := &vendorPortalRecordingClient{}
	rec := serve(vendorPortalTestRouter(mock), vendorPortalRequest(http.MethodGet, "/vp/rfqs/111", "", false))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, mock.rfqCalls)
}

func TestVendorPortalRoutes_WithoutRecipientHeaderAre401AndNeverReachAI(t *testing.T) {
	mock := &vendorPortalRecordingClient{}
	r := vendorPortalTestRouter(mock)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/vp/rfqs/111"},
		{http.MethodPost, "/vp/rfqs/111/response_submissions"},
		{http.MethodPost, "/vp/rfqs/111/seller_responses/9/withdrawals"},
		{http.MethodPost, "/vp/rfqs/111/declines"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		req.Header.Set("Idempotency-Key", "idem-1")
		rec := serve(r, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code, tc.path)
	}
	require.Zero(t, mock.rfqCalls+len(mock.submitCalls)+len(mock.withdrawCalls)+len(mock.declineCalls))
}

func TestVendorPortalWrites_UnverifiedSessionIs403AndNeverReachesAI(t *testing.T) {
	mock := &vendorPortalRecordingClient{}
	r := vendorPortalTestRouter(mock)
	for _, path := range []string{
		"/vp/rfqs/111/response_submissions",
		"/vp/rfqs/111/seller_responses/9/withdrawals",
		"/vp/rfqs/111/declines",
	} {
		rec := serve(r, vendorPortalRequest(http.MethodPost, path, `{}`, false))
		require.Equal(t, http.StatusForbidden, rec.Code, path)
	}
	require.Zero(t, len(mock.submitCalls)+len(mock.withdrawCalls)+len(mock.declineCalls))
}

func TestVendorPortalWrites_ForwardTheVerificationEvidenceFromHeaders(t *testing.T) {
	mock := &vendorPortalRecordingClient{}
	r := vendorPortalTestRouter(mock)

	rec := serve(r, vendorPortalRequest(http.MethodPost, "/vp/rfqs/111/response_submissions", `{"expectedSellerResponseVersion":"1"}`, true))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = serve(r, vendorPortalRequest(http.MethodPost, "/vp/rfqs/111/seller_responses/9/withdrawals", `{"expectedSellerResponseVersion":"1","reasonCode":"X"}`, true))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = serve(r, vendorPortalRequest(http.MethodPost, "/vp/rfqs/111/declines", `{"reasonCode":"Y"}`, true))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	require.Len(t, mock.submitCalls, 1)
	require.Len(t, mock.withdrawCalls, 1)
	require.Len(t, mock.declineCalls, 1)
	want := time.Date(2026, 10, 4, 8, 30, 0, 0, time.UTC)
	for name, v := range map[string]*aipb.VendorPortalVerification{
		"submit":   mock.submitCalls[0].GetVerification(),
		"withdraw": mock.withdrawCalls[0].GetVerification(),
		"decline":  mock.declineCalls[0].GetVerification(),
	} {
		require.NotNil(t, v, name)
		require.Equal(t, "GOOGLE", v.GetMethod(), name)
		require.Equal(t, "vendor@example.com", v.GetEmail(), name)
		require.Equal(t, testVendorSessionID, v.GetSessionId(), name)
		require.True(t, want.Equal(v.GetVerifiedAt().AsTime()), name)
	}
	require.Equal(t, "111", mock.submitCalls[0].GetRfqRecipientId())
	require.Equal(t, "idem-1", mock.submitCalls[0].GetClientRequestId())
}

// 白名單映射漏一欄在型別上完全合法，症狀只是瀏覽器上少一行。
func TestVendorPortalRFQDTOFromProto_MapsWhitelistedFields(t *testing.T) {
	dto := vendorPortalRFQDTOFromProto(&aipb.RfqPublicationInviteSellerViewResponse{
		QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
		Item:                  "APPLE",
		ItemLabel:             "台灣蘋果",
		Quantity:              "50",
		Unit:                  "BOX",
		Packaging:             "紙箱",
		DeliveryCity:          "桃園市",
		DeliveryDate:          "2026-09-16",
		ResponseDeadline:      "2026-09-14",
	})

	require.Equal(t, VendorPortalRFQDTO{
		QualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
		Item:                  "APPLE",
		ItemLabel:             "台灣蘋果",
		Quantity:              "50",
		Unit:                  "BOX",
		Packaging:             "紙箱",
		DeliveryCity:          "桃園市",
		DeliveryDate:          "2026-09-16",
		ResponseDeadline:      "2026-09-14",
	}, dto)
}

// 舊 approval 沒有這個 key，必須原樣回空字串。
func TestVendorPortalRFQDTOFromProto_KeepsLegacyEmptyQualityState(t *testing.T) {
	dto := vendorPortalRFQDTOFromProto(&aipb.RfqPublicationInviteSellerViewResponse{Item: "APPLE"})
	require.Empty(t, dto.QualityKnowledgeState)
}
