package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/cache"
	"github.com/Will413028/hawkr/gateway/internal/middleware"
	pkgredis "github.com/Will413028/hawkr/go-pkg/redis"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func setupWSTicketRouter(h *Holder) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1/app/chat")
	v1.Use(fakeJWTAuth())
	v1.POST("/ws-ticket", h.MintChatWSTicket)
	return r
}

// setHandlerMiniRedis points cache.RedisClient at an in-process miniredis
// (always runs; restores the previous client on cleanup).
func setHandlerMiniRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	prev := cache.RedisClient
	cache.RedisClient = rdb
	t.Cleanup(func() { _ = rdb.Close(); cache.RedisClient = prev })
	return rdb
}

func TestMintChatWSTicket_PersistsConsumableTicket(t *testing.T) {
	rdb := setHandlerMiniRedis(t)

	h := &Holder{Store: grandfatherResolverForTest()}
	r := setupWSTicketRouter(h)
	req, _ := http.NewRequest("POST", "/api/v1/app/chat/ws-ticket", nil)
	req.Header.Set("X-Account-Id", "100")
	req.Header.Set("X-Account-Type", "3")
	req.Header.Set("X-Login-Role", "2") // active role = seller
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	ticket, _ := data["ticket"].(string)
	if ticket == "" {
		t.Fatal("response missing ticket")
	}
	if data["userId"] != "100" {
		t.Errorf("userId = %v, want 100", data["userId"])
	}
	if data["userType"].(float64) != 2 {
		t.Errorf("userType = %v, want 2", data["userType"])
	}

	got, err := pkgredis.ConsumeWSTicket(context.Background(), rdb, ticket)
	if err != nil || got == nil {
		t.Fatalf("ConsumeWSTicket(%q) = %+v, %v; want a ticket", ticket, got, err)
	}
	if got.AccountID != 100 || got.AccountType != 3 || got.ActiveRole != 2 {
		t.Errorf("ticket = %+v, want {100, 3, 2}", got)
	}
}

// type=3 (both) account minting in the BUYER view: same account/headers as the
// seller case above but X-Login-Role=1 → ticket bakes ActiveRole=1 and userType=1.
// The seller case bakes 2; together they pin both directions so a dual-role
// account's buyer-side and seller-side chat route to IM as different accountType
// and never mix.
func TestMintChatWSTicket_BuyerView(t *testing.T) {
	rdb := setHandlerMiniRedis(t)
	h := &Holder{BuyerID: buyerResolverForTest(9999)}
	r := setupWSTicketRouter(h)
	req, _ := http.NewRequest("POST", "/api/v1/app/chat/ws-ticket", nil)
	req.Header.Set("X-Account-Id", "100")
	req.Header.Set("X-Account-Type", "3")
	req.Header.Set("X-Login-Role", "1") // active role = buyer
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	ticket, _ := data["ticket"].(string)
	if ticket == "" {
		t.Fatal("response missing ticket")
	}
	if data["userId"] != "9999" {
		t.Errorf("userId = %v, want 9999 (resolved buyer_id, not raw account_id 100)", data["userId"])
	}
	if data["userType"].(float64) != 1 {
		t.Errorf("userType = %v, want 1 (buyer)", data["userType"])
	}

	got, err := pkgredis.ConsumeWSTicket(context.Background(), rdb, ticket)
	if err != nil || got == nil {
		t.Fatalf("ConsumeWSTicket(%q) = %+v, %v; want a ticket", ticket, got, err)
	}
	if got.AccountID != 100 || got.AccountType != 3 || got.ActiveRole != 1 {
		t.Errorf("ticket = %+v, want {100, 3, 1}", got)
	}
}

// Buyer resolver erroring must NOT fail the mint: MintChatWSTicket's buyer
// branch is deliberately fail-open (mirrors the pre-existing seller branch) —
// a buyer-service hiccup must not block issuing a chat ticket. This pins that
// when h.BuyerID.CurrentBuyerID errors, wireID falls back to the raw
// accountID (not a 403, not a 5xx) — guards against someone later "fixing"
// this into a fail-closed check.
func TestMintChatWSTicket_BuyerView_ResolverErrorFailsOpen(t *testing.T) {
	setHandlerMiniRedis(t)

	h := &Holder{BuyerID: buyerServiceDownResolverForTest()}
	r := setupWSTicketRouter(h)
	req, _ := http.NewRequest("POST", "/api/v1/app/chat/ws-ticket", nil)
	req.Header.Set("X-Account-Id", "100")
	req.Header.Set("X-Account-Type", "3")
	req.Header.Set("X-Login-Role", "1") // active role = buyer
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)

	data := body["data"].(map[string]interface{})
	ticket, _ := data["ticket"].(string)
	if ticket == "" {
		t.Fatal("response missing ticket")
	}
	if data["userId"] != "100" {
		t.Errorf("userId = %v, want 100 (raw account_id fallback when buyer resolver errors)", data["userId"])
	}
	if data["userType"].(float64) != 1 {
		t.Errorf("userType = %v, want 1 (buyer)", data["userType"])
	}
}

// Redis unavailable no longer fail-closes: mint degrades to a stateless HMAC
// ticket (IM design D5) with the same envelope, and that ticket authenticates
// the WS upgrade end-to-end (verify side also with Redis down) carrying the
// baked identity — pinning that mint passes accountId/accountType/activeRole
// through SignChatWSTicket in the right order.
func TestMintChatWSTicket_NoRedis_DegradesToHMAC(t *testing.T) {
	prev := cache.RedisClient
	cache.RedisClient = nil
	t.Cleanup(func() { cache.RedisClient = prev })

	h := &Holder{Store: grandfatherResolverForTest()}
	r := setupWSTicketRouter(h)
	req, _ := http.NewRequest("POST", "/api/v1/app/chat/ws-ticket", nil)
	req.Header.Set("X-Account-Id", "100")
	req.Header.Set("X-Account-Type", "3")
	req.Header.Set("X-Login-Role", "2") // active role = seller
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	assertCode(t, body, 0)
	data := body["data"].(map[string]interface{})
	ticket, _ := data["ticket"].(string)
	if ticket == "" {
		t.Fatal("degrade response missing ticket")
	}
	if data["userId"] != "100" || data["userType"].(float64) != 2 {
		t.Errorf("degrade envelope = %+v, want userId 100 / userType 2", data)
	}

	// End-to-end: the stateless ticket authenticates the WS upgrade with Redis
	// still down on the verify side, applying the baked identity.
	middleware.SetRedisClient(nil)
	t.Cleanup(func() { middleware.SetRedisClient(nil) })
	probe := gin.New()
	probe.GET("/ws", middleware.ChatWSAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"accountId": c.GetInt64("accountId"), "loginRole": c.GetInt("loginRole")})
	})
	vreq, _ := http.NewRequest(http.MethodGet, "/ws?ticket="+ticket, nil)
	vw := httptest.NewRecorder()
	probe.ServeHTTP(vw, vreq)
	if vw.Code != http.StatusOK {
		t.Fatalf("degrade ticket failed WS auth: status=%d body=%s", vw.Code, vw.Body.String())
	}
	if !strings.Contains(vw.Body.String(), `"accountId":100`) || !strings.Contains(vw.Body.String(), `"loginRole":2`) {
		t.Fatalf("degrade ticket baked wrong identity; body=%s", vw.Body.String())
	}
}

func TestMintChatWSTicket_NoAccountIdInContext_401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Holder{}
	r := gin.New()
	r.POST("/api/v1/app/chat/ws-ticket", h.MintChatWSTicket) // no fakeJWTAuth → no accountId in context
	req, _ := http.NewRequest("POST", "/api/v1/app/chat/ws-ticket", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assertHTTPStatus(t, w, http.StatusUnauthorized)
}
