package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/middleware"
)

// These exercise the stateless HMAC ws-ticket degradation (IM design D5): when
// Redis — the single-use ticket store — is unavailable, ChatWSAuth must fall
// back to verifying a signed HMAC ticket so web chat does not fail-closed.

// Redis client never injected (rdb == nil): a valid HMAC ticket still upgrades.
func TestChatWSAuth_HMACDegrade_NoRedis(t *testing.T) {
	middleware.SetRedisClient(nil)

	tok, err := middleware.SignChatWSTicket(55, 1, 1, time.Minute)
	if err != nil {
		t.Fatalf("SignChatWSTicket: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/ws?ticket="+tok, nil)
	w := httptest.NewRecorder()
	wsAuthRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"accountId":55`) || !strings.Contains(w.Body.String(), `"loginRole":1`) {
		t.Fatalf("identity not applied from HMAC ticket; body=%s", w.Body.String())
	}
}

// Redis errors mid-request (client present but server down): degrade to HMAC.
func TestChatWSAuth_HMACDegrade_RedisDown(t *testing.T) {
	_, mr := wsAuthTestRedisMR(t)
	mr.Close() // every command now errors -> ConsumeWSTicket returns err

	tok, _ := middleware.SignChatWSTicket(66, 2, 2, time.Minute)
	req, _ := http.NewRequest(http.MethodGet, "/ws?ticket="+tok, nil)
	w := httptest.NewRecorder()
	wsAuthRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"accountId":66`) || !strings.Contains(w.Body.String(), `"loginRole":2`) {
		t.Fatalf("identity not applied from HMAC ticket; body=%s", w.Body.String())
	}
}

// Recovery window: an HMAC ticket minted during an outage is redeemed AFTER
// Redis recovers. Redis is healthy but never stored this token, so the GETDEL
// misses and verification falls through to HMAC.
func TestChatWSAuth_HMACDegrade_RedisHealthyMiss(t *testing.T) {
	wsAuthTestRedis(t) // healthy Redis, but the HMAC token was never Put

	tok, _ := middleware.SignChatWSTicket(77, 1, 1, time.Minute)
	req, _ := http.NewRequest(http.MethodGet, "/ws?ticket="+tok, nil)
	w := httptest.NewRecorder()
	wsAuthRouter().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"accountId":77`) {
		t.Fatalf("identity not applied from HMAC ticket; body=%s", w.Body.String())
	}
}

// A bogus (non-HMAC) ticket with no Redis must 401, not silently pass.
func TestChatWSAuth_HMACDegrade_GarbageTicket_401(t *testing.T) {
	middleware.SetRedisClient(nil)

	req, _ := http.NewRequest(http.MethodGet, "/ws?ticket=not-a-real-ticket", nil)
	w := httptest.NewRecorder()
	wsAuthRouter().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
}

// A bogus ticket against a HEALTHY Redis must also 401 (the HMAC fallback must
// not turn a Redis miss into a free pass).
func TestChatWSAuth_HMACDegrade_GarbageTicket_RedisHealthy_401(t *testing.T) {
	wsAuthTestRedis(t)

	req, _ := http.NewRequest(http.MethodGet, "/ws?ticket=not-a-real-ticket", nil)
	w := httptest.NewRecorder()
	wsAuthRouter().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
}

// An expired HMAC ticket is rejected even on the degrade path.
func TestChatWSAuth_HMACDegrade_Expired_401(t *testing.T) {
	middleware.SetRedisClient(nil)

	tok, _ := middleware.SignChatWSTicket(88, 1, 1, -time.Second) // already expired
	req, _ := http.NewRequest(http.MethodGet, "/ws?ticket="+tok, nil)
	w := httptest.NewRecorder()
	wsAuthRouter().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
}

// Redis 掛掉時降級要快：一次受 wsTicketLookupTimeout（800ms）限制的 GETDEL，接著驗 HMAC（微秒級），不再有第二次
// Redis 往返。斷線重連的風暴裡，每次連線多卡一次逾時就會堆積起來。
func TestChatWSAuth_HMACDegrade_RedisDown_StaysFast(t *testing.T) {
	_, mr := wsAuthTestRedisMR(t)
	mr.Close() // Redis down: ConsumeWSTicket errors

	tok, _ := middleware.SignChatWSTicket(123, 1, 1, time.Minute)
	req, _ := http.NewRequest(http.MethodGet, "/ws?ticket="+tok, nil)
	w := httptest.NewRecorder()

	start := time.Now()
	wsAuthRouter().ServeHTTP(w, req)
	elapsed := time.Since(start)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("degrade took %v (> 1.5s): the ticket lookup is no longer bounded, or a second Redis round trip was added", elapsed)
	}
}
