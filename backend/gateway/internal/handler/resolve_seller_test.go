package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/buyerid"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/gateway/internal/storeid"
	buyerpb "github.com/Will413028/hawkr/go-pkg/proto/buyer"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)


func storeResolverForTest(surrogate int64) *storeid.Resolver {
	return storeid.New(storeid.GetterFunc(func(_ context.Context, accountID int64) (int64, error) {
		return surrogate, nil
	}))
}

func grandfatherResolverForTest() *storeid.Resolver {
	return storeid.New(storeid.GetterFunc(func(_ context.Context, accountID int64) (int64, error) {
		return accountID, nil
	}))
}

func mustNotResolveStore(t *testing.T) *storeid.Resolver {
	t.Helper()
	return storeid.New(storeid.GetterFunc(func(context.Context, int64) (int64, error) {
		t.Fatal("seller resolver must not be called when loginRole is unresolved")
		return 0, nil
	}))
}

func noStoreResolverForTest() *storeid.Resolver {
	return storeid.New(storeid.GetterFunc(func(context.Context, int64) (int64, error) {
		return 0, status.Error(codes.NotFound, "no current store for account")
	}))
}

func buyerResolverForTest(surrogate int64) *buyerid.Resolver {
	return buyerid.New(buyerid.GetterFunc(func(_ context.Context, accountID int64) (*buyerpb.BuyerInfo, error) {
		return &buyerpb.BuyerInfo{
			Id:        strconv.FormatInt(surrogate, 10),
			AccountId: strconv.FormatInt(accountID, 10),
		}, nil
	}))
}

func buyerGrandfatherResolverForTest() *buyerid.Resolver {
	return buyerid.New(buyerid.GetterFunc(func(_ context.Context, accountID int64) (*buyerpb.BuyerInfo, error) {
		return &buyerpb.BuyerInfo{
			Id:        strconv.FormatInt(accountID, 10),
			AccountId: strconv.FormatInt(accountID, 10),
		}, nil
	}))
}


func setupResolveProbe(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/probe", fakeSellerAuth(), func(c *gin.Context) {
		id, ok := h.resolveStoreID(c)
		if !ok {
			return
		}
		response.OK(c, gin.H{"storeId": strconv.FormatInt(int64(id), 10)})
	})
	return r
}

func TestResolveStoreID_ReturnsSurrogate(t *testing.T) {
	h := &Holder{Store: storeResolverForTest(7001)}
	r := setupResolveProbe(h)

	req := httptest.NewRequest("GET", "/probe", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	if data["storeId"] != "7001" {
		t.Errorf("storeId = %v, want 7001", data["storeId"])
	}
}

func TestResolveStoreID_NoCurrentSeller403(t *testing.T) {
	h := &Holder{Store: noStoreResolverForTest()}
	r := setupResolveProbe(h)

	req := httptest.NewRequest("GET", "/probe", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusForbidden)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 403)
	if body["message"] != "seller access required" {
		t.Errorf("message = %v, want %q", body["message"], "seller access required")
	}
}

func TestResolveStoreID_MissingIdentity401(t *testing.T) {
	h := &Holder{Store: storeResolverForTest(7001)}
	r := gin.New()
	r.GET("/probe", func(c *gin.Context) {
		if _, ok := h.resolveStoreID(c); !ok {
			return
		}
		t.Error("must not resolve without identity")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/probe", nil))
	assertHTTPStatus(t, w, http.StatusUnauthorized)
}
