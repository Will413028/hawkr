package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
	pkgredis "github.com/Will413028/hawkr/go-pkg/redis"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// wsAuthRouter mounts ChatWSAuth on a probe route that echoes the resolved
// identity.
func wsAuthRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", middleware.ChatWSAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"accountId": c.GetInt64("accountId"),
			"loginRole": c.GetInt("loginRole"),
		})
	})
	return r
}

// wsAuthTestRedis backs the middleware's Redis with an in-process miniredis
// (always runs; no external Redis, no skip).
func wsAuthTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	middleware.SetRedisClient(rdb)
	t.Cleanup(func() { _ = rdb.Close(); middleware.SetRedisClient(nil) })
	return rdb
}

// wsAuthTestRedisMR is like wsAuthTestRedis but also returns the *miniredis.Miniredis
// so callers can use FastForward for TTL-based expiry tests.
func wsAuthTestRedisMR(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	middleware.SetRedisClient(rdb)
	t.Cleanup(func() { _ = rdb.Close(); middleware.SetRedisClient(nil) })
	return rdb, mr
}

func TestChatWSAuth_ValidTicket(t *testing.T) {
	rdb := wsAuthTestRedis(t)
	const token = "ws-auth-ticket-valid"
	if err := pkgredis.PutWSTicket(context.Background(), rdb, token,
		&pkgredis.WSTicket{AccountID: 100, AccountType: 3, ActiveRole: 2}, time.Minute); err != nil {
		t.Fatalf("PutWSTicket: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/ws?ticket="+token, nil)
	w := httptest.NewRecorder()
	wsAuthRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"accountId":100`) || !strings.Contains(w.Body.String(), `"loginRole":2`) {
		t.Fatalf("identity not applied from ticket; body=%s", w.Body.String())
	}
}

// type=3 (both) account connecting in the BUYER view: same accountId as the
// seller-view ValidTicket test (100), but ActiveRole=1 must resolve to
// loginRole=1 (vs the seller-view test's loginRole=2). Since ChatWebSocket
// derives the IM `accountType` from loginRole, this keeps a dual-role account's
// buyer-side chat separate from its seller-side chat — they never mix.
func TestChatWSAuth_ValidTicket_BuyerView(t *testing.T) {
	rdb := wsAuthTestRedis(t)
	const token = "ws-auth-ticket-buyer-view"
	if err := pkgredis.PutWSTicket(context.Background(), rdb, token,
		&pkgredis.WSTicket{AccountID: 100, AccountType: 3, ActiveRole: 1}, time.Minute); err != nil {
		t.Fatalf("PutWSTicket: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/ws?ticket="+token, nil)
	w := httptest.NewRecorder()
	wsAuthRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"accountId":100`) || !strings.Contains(w.Body.String(), `"loginRole":1`) {
		t.Fatalf("buyer-view ticket should resolve loginRole:1; body=%s", w.Body.String())
	}
}

// A stored ticket's ActiveRole is mint-time state, not a fact about now: the
// ticket outlives a role removal and would carry that role forward into the
// chat service (which derives the IM accountType from loginRole). It must be
// held to the account's role set, same as the HMAC degrade path.
func TestChatWSAuth_StoredTicketRoleNotHeld_NotHandedToChat(t *testing.T) {
	cases := []struct {
		name        string
		token       string
		accountType int32
		activeRole  int32
		wantRole    string // what the chat service must see
	}{
		// 7 is not a role at all → "no active role".
		{name: "out_of_range", token: "ws-ticket-role-7", accountType: 3, activeRole: 7, wantRole: `"loginRole":0`},
		// Seller role on a buyer-only account → dropped, then derived from the
		// account's single role rather than honoured.
		{name: "not_in_set", token: "ws-ticket-role-not-held", accountType: 1, activeRole: 2, wantRole: `"loginRole":1`},
		// Control: a role the account holds is untouched.
		{name: "legal_role_preserved", token: "ws-ticket-role-legal", accountType: 3, activeRole: 2, wantRole: `"loginRole":2`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rdb := wsAuthTestRedis(t)
			if err := pkgredis.PutWSTicket(context.Background(), rdb, tc.token,
				&pkgredis.WSTicket{AccountID: 101, AccountType: tc.accountType, ActiveRole: tc.activeRole},
				time.Minute); err != nil {
				t.Fatalf("PutWSTicket: %v", err)
			}

			req, _ := http.NewRequest(http.MethodGet, "/ws?ticket="+tc.token, nil)
			w := httptest.NewRecorder()
			wsAuthRouter().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("a valid ticket must still connect (only the role is corrected), got %d; body=%s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.wantRole) {
				t.Fatalf("body %s, want %s", w.Body.String(), tc.wantRole)
			}
		})
	}
}

func TestChatWSAuth_TicketIsSingleUse(t *testing.T) {
	rdb := wsAuthTestRedis(t)
	const token = "ws-auth-ticket-reuse"
	_ = pkgredis.PutWSTicket(context.Background(), rdb, token,
		&pkgredis.WSTicket{AccountID: 1, AccountType: 1, ActiveRole: 1}, time.Minute)

	first := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/ws?ticket="+token, nil)
	wsAuthRouter().ServeHTTP(first, req1)
	if first.Code != http.StatusOK {
		t.Fatalf("first use status = %d, want 200", first.Code)
	}

	second := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/ws?ticket="+token, nil)
	wsAuthRouter().ServeHTTP(second, req2)
	if second.Code != http.StatusUnauthorized {
		t.Fatalf("reused ticket status = %d, want 401 (single-use)", second.Code)
	}
}

func TestChatWSAuth_MissingTicketNoCredential_401(t *testing.T) {
	middleware.SetRedisClient(nil)
	req, _ := http.NewRequest(http.MethodGet, "/ws", nil)
	w := httptest.NewRecorder()
	wsAuthRouter().ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

// 沒有 ticket 時信任單體的身分 header（App 以 Bearer 連線，單體認證後設上）。
func TestChatWSAuth_IdentityHeaderFallback(t *testing.T) {
	middleware.SetRedisClient(nil) // 沒有 ticket 的路徑不需要 Redis

	req, _ := http.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("X-Account-Id", "55")
	req.Header.Set("X-Account-Type", "1")
	req.Header.Set("X-Login-Role", "1")
	w := httptest.NewRecorder()
	wsAuthRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("identity header fallback status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"accountId":55`) || !strings.Contains(w.Body.String(), `"loginRole":1`) {
		t.Fatalf("identity not applied from headers; body=%s", w.Body.String())
	}
}

// gateway 不再讀憑證：只帶 Bearer、沒有單體設的身分 header 時是 401。
func TestChatWSAuth_CredentialWithoutIdentityHeaders_401(t *testing.T) {
	middleware.SetRedisClient(nil)

	req, _ := http.NewRequest(http.MethodGet, "/ws?token=legacy", nil)
	req.Header.Set("Authorization", "Bearer legacy-jwt")
	req.AddCookie(&http.Cookie{Name: "hk_session", Value: "legacy-session"})
	w := httptest.NewRecorder()
	wsAuthRouter().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
}

// a ticket whose TTL has elapsed is rejected (GETDEL returns nil after expiry).
func TestChatWSAuth_ExpiredTicket_401(t *testing.T) {
	ctx := context.Background()
	rdb, mr := wsAuthTestRedisMR(t)
	const token = "ws-auth-ticket-expired"

	if err := pkgredis.PutWSTicket(ctx, rdb, token,
		&pkgredis.WSTicket{AccountID: 300, AccountType: 1, ActiveRole: 1}, 5*time.Second); err != nil {
		t.Fatalf("PutWSTicket: %v", err)
	}

	// Advance miniredis clock past the ticket TTL.
	mr.FastForward(10 * time.Second)

	req, _ := http.NewRequest(http.MethodGet, "/ws?ticket="+token, nil)
	w := httptest.NewRecorder()
	wsAuthRouter().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expired ticket should 401, got %d; body=%s", w.Code, w.Body.String())
	}
}
