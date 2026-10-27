package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/response"
	buyerpb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// noBuyerResolverForTest：模擬帳號目前無現任買家 stint（buyer-service 404，
// resolver 轉譯成 ErrNoCurrentBuyer）。mirrors noStoreResolverForTest in
// resolve_seller_test.go.
func noBuyerResolverForTest() *buyerid.Resolver {
	return buyerid.New(buyerid.GetterFunc(func(context.Context, int64) (*buyerpb.BuyerInfo, error) {
		return nil, status.Error(codes.NotFound, "buyer not found")
	}))
}

// ==================== resolveBuyerID ====================

func setupResolveBuyerProbe(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/probe", fakeJWTAuth(), func(c *gin.Context) {
		id, ok := h.resolveBuyerID(c)
		if !ok {
			return
		}
		response.OK(c, gin.H{"buyerId": strconv.FormatInt(int64(id), 10)})
	})
	return r
}

func TestResolveBuyerID_ReturnsSurrogate(t *testing.T) {
	h := &Holder{BuyerID: buyerResolverForTest(9001)}
	r := setupResolveBuyerProbe(h)

	req := httptest.NewRequest("GET", "/probe", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	if data["buyerId"] != "9001" {
		t.Errorf("buyerId = %v, want 9001 (resolved surrogate, not raw account_id 42)", data["buyerId"])
	}
}

func TestResolveBuyerID_NoCurrentBuyer403(t *testing.T) {
	h := &Holder{BuyerID: noBuyerResolverForTest()}
	r := setupResolveBuyerProbe(h)

	req := httptest.NewRequest("GET", "/probe", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 403)
	if body["message"] != "buyer access required" {
		t.Errorf("message = %v, want %q", body["message"], "buyer access required")
	}
}

func TestResolveBuyerID_MissingIdentity401(t *testing.T) {
	h := &Holder{BuyerID: buyerResolverForTest(9001)}
	r := gin.New()
	r.GET("/probe", func(c *gin.Context) {
		if _, ok := h.resolveBuyerID(c); !ok {
			return
		}
		t.Error("must not resolve without identity")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/probe", nil))
	assertHTTPStatus(t, w, http.StatusUnauthorized)
}

// ==================== resolveOptionalBuyerFilterQuery ====================

func setupResolveBuyerFilterProbe(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/probe", func(c *gin.Context) {
		id, ok := h.resolveOptionalBuyerFilterQuery(c, c.Query("buyer_id"))
		if !ok {
			return
		}
		response.OK(c, gin.H{"resolved": id})
	})
	return r
}

func TestResolveOptionalBuyerFilterQuery_Empty_NoFilter(t *testing.T) {
	h := &Holder{BuyerID: buyerResolverForTest(9001)}
	r := setupResolveBuyerFilterProbe(h)

	req := httptest.NewRequest("GET", "/probe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	if data["resolved"] != "" {
		t.Errorf("resolved = %v, want empty string (no filter)", data["resolved"])
	}
}

func TestResolveOptionalBuyerFilterQuery_ResolvesSurrogate(t *testing.T) {
	h := &Holder{BuyerID: buyerResolverForTest(9001)}
	r := setupResolveBuyerFilterProbe(h)

	req := httptest.NewRequest("GET", "/probe?buyer_id=42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	if data["resolved"] != "9001" {
		t.Errorf("resolved = %v, want 9001 (resolved surrogate, not raw 42)", data["resolved"])
	}
}

func TestResolveOptionalBuyerFilterQuery_NoCurrentBuyer_ReturnsSentinel(t *testing.T) {
	h := &Holder{BuyerID: noBuyerResolverForTest()}
	r := setupResolveBuyerFilterProbe(h)

	req := httptest.NewRequest("GET", "/probe?buyer_id=42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	if data["resolved"] != "-1" {
		t.Errorf("resolved = %v, want \"-1\" sentinel", data["resolved"])
	}
}

func TestResolveOptionalBuyerFilterQuery_InvalidID_BadRequest(t *testing.T) {
	h := &Holder{BuyerID: buyerResolverForTest(9001)}
	r := setupResolveBuyerFilterProbe(h)

	req := httptest.NewRequest("GET", "/probe?buyer_id=not-a-number", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}
