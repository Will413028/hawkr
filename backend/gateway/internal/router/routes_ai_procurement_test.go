package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/handler"
	"github.com/gin-gonic/gin"
)

// TestProcurementRoutes_MountedWithHandlers asserts the two Hawkr 幫找 procurement
// endpoints are actually registered on the engine (not just present in code) and
// dispatch to the expected Holder methods — a route-table check, not a code read.
func TestProcurementRoutes_MountedWithHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.Holder{}
	r := gin.New()
	app := r.Group("/api/v1/app")
	admin := r.Group("/api/v1/admin")
	registerAIRoutes(app, admin, h)

	routes := r.Routes()

	want := map[string]string{ // "METHOD PATH" -> substring expected in the handler func name
		"POST /api/v1/app/buyers/me/procurement/turns":                                                                          "ProxyProcurementTurn",
		"POST /api/v1/app/buyers/me/procurement/turns/stream":                                                                   "ProxyProcurementTurnStream",
		"GET /api/v1/app/buyers/me/procurement/requests/:id/snapshot":                                                           "ProxyProcurementSnapshot",
		"POST /api/v1/app/buyers/me/procurement/requests/:id/candidate-searches":                                                "ProxyProcurementCandidateSearch",
		"GET /api/v1/app/buyers/me/procurement/requests/:id/candidate-searches/:runId":                                          "ProxyProcurementCandidateSearchGet",
		"POST /api/v1/app/buyers/me/procurement/requests/:id/selections":                                                        "ProxyProcurementSelection",
		"POST /api/v1/app/buyers/me/procurement/requests/:id/answers":                                                           "ProxyProcurementAnswer",
		"POST /api/v1/app/buyers/me/procurement/requests/:id/rewinds":                                                           "ProxyProcurementRewind",
		"POST /api/v1/app/buyers/me/procurement/requests/:id/rfq-price-previews":                                                "ProxyProcurementRfqPricePreview",
		"POST /api/v1/app/buyers/me/procurement/requests/:id/rfq-confirmations":                                                 "ProxyProcurementRfqConfirmation",
		"POST /api/v1/app/buyers/me/procurement/requests/:id/rfq-publications/preview":                                          "ProxyProcurementRfqPublicationPreview",
		"POST /api/v1/app/buyers/me/procurement/requests/:id/rfq-publications":                                                  "ProxyProcurementRfqPublicationExecute",
		"GET /api/v1/app/buyers/me/procurement/conversations":                                                                   "ProxyProcurementConversations",
		"GET /api/v1/app/buyers/me/procurement/conversations/:id/messages":                                                      "ProxyProcurementConversationMessages",
		"GET /api/v1/app/buyers/me/procurement/conversations/:id/runtime-snapshot":                                              "ProxyProcurementConversationRuntimeSnapshot",
		"GET /api/v1/app/buyers/me/procurement/requests/:id/seller-responses":                                                   "ProxyBuyerProcurementSellerResponses",
		"PUT /api/v1/app/buyers/me/procurement/requests/:id/shortlist/:seller_response_id/versions/:seller_response_version":    "ProxyBuyerProcurementShortlistPut",
		"DELETE /api/v1/app/buyers/me/procurement/requests/:id/shortlist/:seller_response_id/versions/:seller_response_version": "ProxyBuyerProcurementShortlistDelete",
		"POST /api/v1/app/buyers/me/procurement/requests/:id/buyer-quote-acceptances":                                           "ProxyBuyerProcurementQuoteAcceptance",
		"PATCH /api/v1/app/buyers/me/procurement/conversations/:id":                                                             "ProxyProcurementConversationRename",
		"DELETE /api/v1/app/buyers/me/procurement/conversations/:id":                                                            "ProxyProcurementConversationDelete",
		"GET /api/v1/app/buyers/me/procurement/runs/:runId/events":                                                              "ProxyProcurementRunEvents",
	}

	found := make(map[string]bool, len(want))
	for _, ri := range routes {
		key := ri.Method + " " + ri.Path
		if wantHandler, ok := want[key]; ok {
			found[key] = true
			if !strings.Contains(ri.Handler, wantHandler) {
				t.Errorf("route %s: handler = %q, want it to contain %q", key, ri.Handler, wantHandler)
			}
		}
	}
	for key := range want {
		if !found[key] {
			t.Errorf("route %s not found in engine.Routes(); got %d routes total", key, len(routes))
		}
	}
}

// TestProcurementRoutes_RequireJWTAuth proves both routes are behind JWTAuth
// (fail closed to 401) rather than accidentally left open — hitting the real
// mounted route with no Authorization header/cookie, not inspecting middleware
// chain code.
func TestProcurementRoutes_RequireJWTAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &handler.Holder{}
	r := gin.New()
	app := r.Group("/api/v1/app")
	admin := r.Group("/api/v1/admin")
	registerAIRoutes(app, admin, h)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/app/buyers/me/procurement/turns"},
		{http.MethodPost, "/api/v1/app/buyers/me/procurement/turns/stream"},
		{http.MethodGet, "/api/v1/app/buyers/me/procurement/requests/abc123/snapshot"},
		{http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/abc123/candidate-searches"},
		{http.MethodGet, "/api/v1/app/buyers/me/procurement/requests/abc123/candidate-searches/run1"},
		{http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/abc123/selections"},
		{http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/abc123/answers"},
		{http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/abc123/rewinds"},
		{http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/abc123/rfq-price-previews"},
		{http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/abc123/rfq-confirmations"},
		{http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/abc123/rfq-publications/preview"},
		{http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/abc123/rfq-publications"},
		{http.MethodGet, "/api/v1/app/buyers/me/procurement/conversations"},
		{http.MethodGet, "/api/v1/app/buyers/me/procurement/conversations/abc123/messages"},
		{http.MethodGet, "/api/v1/app/buyers/me/procurement/conversations/abc123/runtime-snapshot"},
		{http.MethodGet, "/api/v1/app/buyers/me/procurement/requests/abc123/seller-responses"},
		{http.MethodPut, "/api/v1/app/buyers/me/procurement/requests/abc123/shortlist/r1/versions/2"},
		{http.MethodDelete, "/api/v1/app/buyers/me/procurement/requests/abc123/shortlist/r1/versions/2"},
		{http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/abc123/buyer-quote-acceptances"},
		{http.MethodPatch, "/api/v1/app/buyers/me/procurement/conversations/abc123"},
		{http.MethodDelete, "/api/v1/app/buyers/me/procurement/conversations/abc123"},
		{http.MethodGet, "/api/v1/app/buyers/me/procurement/runs/run1/events"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s with no credentials: status = %d, want 401 (JWTAuth must reject)", tc.method, tc.path, w.Code)
		}
	}
}
