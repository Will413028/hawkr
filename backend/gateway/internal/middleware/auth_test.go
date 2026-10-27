package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/gin-gonic/gin"
)

// identityEcho 回傳中介層交給下游的身分：往下游的 header、gin ctx 與 gRPC 用的 Go context。
type identityEcho struct {
	HeaderAccountID   string `json:"headerAccountId"`
	HeaderAccountType string `json:"headerAccountType"`
	HeaderLoginRole   string `json:"headerLoginRole"`
	CtxAccountID      *int64 `json:"ctxAccountId"`
	CtxAccountType    *int   `json:"ctxAccountType"`
	CtxLoginRole      *int   `json:"ctxLoginRole"`
	GRPCAccountID     int64  `json:"grpcAccountId"`
	GRPCAccountType   string `json:"grpcAccountType"`
}

func identityEchoRouter(mw gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", mw, func(c *gin.Context) {
		var e identityEcho
		e.HeaderAccountID = c.Request.Header.Get(headerAccountID)
		e.HeaderAccountType = c.Request.Header.Get(headerAccountType)
		e.HeaderLoginRole = c.Request.Header.Get(headerLoginRole)
		if v, ok := c.Get("accountId"); ok {
			id := v.(int64)
			e.CtxAccountID = &id
		}
		if v, ok := c.Get("accountType"); ok {
			t := v.(int)
			e.CtxAccountType = &t
		}
		if v, ok := c.Get("loginRole"); ok {
			r := v.(int)
			e.CtxLoginRole = &r
		}
		e.GRPCAccountID, _ = c.Request.Context().Value(grpcclient.CtxKeyAccountID).(int64)
		e.GRPCAccountType, _ = c.Request.Context().Value(grpcclient.CtxKeyAccountType).(string)
		c.JSON(http.StatusOK, e)
	})
	return r
}

func serveWithHeaders(r *gin.Engine, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeEcho(t *testing.T, w *httptest.ResponseRecorder) identityEcho {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var e identityEcho
	if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	return e
}

// 單體帶的身分 header 重建成與過去相同的 gin ctx、往下游的 header 與 gRPC 的 Go context。
func TestJWTAuth_TrustsMonolithIdentityHeaders(t *testing.T) {
	cases := []struct {
		name      string
		headers   map[string]string
		wantType  int
		wantRole  int
		wantRoleH string
	}{
		{"buyer", map[string]string{"X-Account-Id": "101", "X-Account-Type": "1", "X-Login-Role": "1"}, 1, 1, "1"},
		{"seller", map[string]string{"X-Account-Id": "101", "X-Account-Type": "2", "X-Login-Role": "2"}, 2, 2, "2"},
		{"dual in buyer view", map[string]string{"X-Account-Id": "101", "X-Account-Type": "3", "X-Login-Role": "1"}, 3, 1, "1"},
		{"dual in seller view", map[string]string{"X-Account-Id": "101", "X-Account-Type": "3", "X-Login-Role": "2"}, 3, 2, "2"},
		// 沒有角色的帳號：單體不設 X-Login-Role。
		{"no roles", map[string]string{"X-Account-Id": "101", "X-Account-Type": "0"}, 0, 0, "0"},
		// 管理員：單體送 4；管理員沒有買家或賣家的視角，往下游與過去一樣是 0。
		{"admin", map[string]string{"X-Account-Id": "101", "X-Account-Type": "4", "X-Login-Role": "4"}, 4, 0, "0"},
		// 帳號沒有的角色不往下游傳：單一角色的帳號取它唯一的角色，其餘落到 0。
		{"role not held, single role", map[string]string{"X-Account-Id": "101", "X-Account-Type": "1", "X-Login-Role": "2"}, 1, 1, "1"},
		{"role not a role", map[string]string{"X-Account-Id": "101", "X-Account-Type": "3", "X-Login-Role": "7"}, 3, 0, "0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := decodeEcho(t, serveWithHeaders(identityEchoRouter(JWTAuth()), tc.headers))
			if e.CtxAccountID == nil || *e.CtxAccountID != 101 || e.HeaderAccountID != "101" || e.GRPCAccountID != 101 {
				t.Errorf("accountId: ctx=%v header=%q grpc=%d, want 101 everywhere", e.CtxAccountID, e.HeaderAccountID, e.GRPCAccountID)
			}
			if e.CtxAccountType == nil || *e.CtxAccountType != tc.wantType {
				t.Errorf("ctx accountType = %v, want %d", e.CtxAccountType, tc.wantType)
			}
			if e.HeaderAccountType != e.GRPCAccountType || e.GRPCAccountType == "" {
				t.Errorf("accountType: header=%q grpc=%q, want the same non-empty value", e.HeaderAccountType, e.GRPCAccountType)
			}
			if e.CtxLoginRole == nil || *e.CtxLoginRole != tc.wantRole {
				t.Errorf("ctx loginRole = %v, want %d", e.CtxLoginRole, tc.wantRole)
			}
			if e.HeaderLoginRole != tc.wantRoleH {
				t.Errorf("downstream X-Login-Role = %q, want %q", e.HeaderLoginRole, tc.wantRoleH)
			}
		})
	}
}

// X-Active-Role 由單體解析成 X-Login-Role，不會傳到 gateway；即使出現也不影響視角。
func TestJWTAuth_IgnoresActiveRoleHint(t *testing.T) {
	e := decodeEcho(t, serveWithHeaders(identityEchoRouter(JWTAuth()), map[string]string{
		"X-Account-Id": "101", "X-Account-Type": "3", "X-Login-Role": "1", "X-Active-Role": "2",
	}))
	if e.CtxLoginRole == nil || *e.CtxLoginRole != 1 || e.HeaderLoginRole != "1" {
		t.Fatalf("loginRole ctx=%v header=%q, want 1 (X-Login-Role is authoritative)", e.CtxLoginRole, e.HeaderLoginRole)
	}
}

// 沒有身分 header，或不合契約時一律 401；gateway 不再讀 Bearer、token 或 cookie。
func TestJWTAuth_RejectsMissingOrMalformedIdentity(t *testing.T) {
	cases := map[string]map[string]string{
		"none":                    {},
		"legacy credentials only": {"Authorization": "Bearer x.y.z", "token": "x.y.z", "Cookie": "hk_session=abc"},
		"id not a number":         {"X-Account-Id": "abc", "X-Account-Type": "1"},
		"id zero":                 {"X-Account-Id": "0", "X-Account-Type": "1"},
		"id negative":             {"X-Account-Id": "-5", "X-Account-Type": "1"},
		"id missing":              {"X-Account-Type": "1", "X-Login-Role": "1"},
		"type missing":            {"X-Account-Id": "101"},
		"type out of range":       {"X-Account-Id": "101", "X-Account-Type": "5"},
		"type negative":           {"X-Account-Id": "101", "X-Account-Type": "-1"},
		"role not a number":       {"X-Account-Id": "101", "X-Account-Type": "1", "X-Login-Role": "buyer"},
	}
	for name, headers := range cases {
		t.Run(name, func(t *testing.T) {
			called := false
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.GET("/x", JWTAuth(), func(c *gin.Context) { called = true })
			w := serveWithHeaders(r, headers)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
			}
			if called {
				t.Fatal("downstream handler ran; abort failed")
			}
		})
	}
}

// OptionalJWTAuth：有身分時套用，沒有或不合契約時當成訪客，一律不中斷；不合契約的 header 不往下游傳。
func TestOptionalJWTAuth(t *testing.T) {
	r := identityEchoRouter(OptionalJWTAuth())

	e := decodeEcho(t, serveWithHeaders(r, map[string]string{"X-Account-Id": "101", "X-Account-Type": "1", "X-Login-Role": "1"}))
	if e.CtxAccountID == nil || *e.CtxAccountID != 101 || e.CtxLoginRole == nil || *e.CtxLoginRole != 1 {
		t.Fatalf("identity not applied: %+v", e)
	}

	e = decodeEcho(t, serveWithHeaders(r, nil))
	if e.CtxAccountID != nil || e.HeaderAccountID != "" {
		t.Fatalf("anonymous request got an identity: %+v", e)
	}

	e = decodeEcho(t, serveWithHeaders(r, map[string]string{"X-Account-Id": "101", "X-Account-Type": "9", "X-Login-Role": "1"}))
	if e.CtxAccountID != nil {
		t.Fatalf("malformed identity must be treated as anonymous: %+v", e)
	}
	if e.HeaderAccountID != "" || e.HeaderAccountType != "" || e.HeaderLoginRole != "" {
		t.Fatalf("malformed identity headers must not reach downstream: %+v", e)
	}
}
