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

// ==================== 共用 test resolvers（R4–R8 測試沿用）====================

// storeResolverForTest 回傳固定 surrogate 的 resolver（無 Redis）。
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

// mustNotResolveStore returns a resolver that fails the test if consulted —
// proves a call whose loginRole isn't 1 or 2 (unresolved active role) never
// attempts seller resolution.
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

// buyerResolverForTest 回傳固定 surrogate 的 resolver（無 Redis）。
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

// buyerServiceDownResolverForTest：模擬 buyer-service 基礎設施錯誤（非
// ErrNoCurrentBuyer），驗證呼叫端走 GrpcErr（503）而非 Forbidden（403）分支。
// noBuyerResolverForTest（ErrNoCurrentBuyer 版）定義在 resolve_buyer_test.go，共用。
func buyerServiceDownResolverForTest() *buyerid.Resolver {
	return buyerid.New(buyerid.GetterFunc(func(context.Context, int64) (*buyerpb.BuyerInfo, error) {
		return nil, status.Error(codes.Unavailable, "buyer-service down")
	}))
}

// storeServiceDownResolverForTest：模擬 seller-service 基礎設施錯誤（非
// ErrNoCurrentStore），驗證呼叫端走 GrpcErr（503）而非 Forbidden（403）分支。
// noStoreResolverForTest（ErrNoCurrentStore 版）定義在上方，共用。
func storeServiceDownResolverForTest() *storeid.Resolver {
	return storeid.New(storeid.GetterFunc(func(context.Context, int64) (int64, error) {
		return 0, status.Error(codes.Unavailable, "seller-service down")
	}))
}

// ==================== resolveStoreID ====================

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
	// 無 auth middleware → context 無 accountId → helper 自己回 401
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

// ==================== chatWireIdentity ====================

func TestChatWireIdentity_BuyerRole_ResolvesViaBuyerID(t *testing.T) {
	h := &Holder{BuyerID: buyerResolverForTest(9999)}
	r := gin.New()
	r.GET("/probe", fakeBuyerAuth(), func(c *gin.Context) {
		id, ok := h.chatWireIdentity(c)
		if !ok {
			return
		}
		response.OK(c, gin.H{"wireId": strconv.FormatInt(id, 10)})
	})

	req := httptest.NewRequest("GET", "/probe", nil)
	req.Header.Set("X-Account-Id", "42")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	if data["wireId"] != "9999" {
		t.Errorf("wireId = %v, want 9999 (current buyer_id, not raw account_id 42)", data["wireId"])
	}
}

// ==================== resolveOptionalBuyerID（W2/W3 PR1 optional-auth
// personalization；賣家軸 2026-08-01 ADR 併入，讀寫同批換） ====================

func TestResolveOptionalBuyerID_Anonymous(t *testing.T) {
	h := &Holder{BuyerID: buyerid.New(buyerid.GetterFunc(func(context.Context, int64) (*buyerpb.BuyerInfo, error) {
		t.Fatal("buyer resolver must not be called for accountID=0 (anonymous)")
		return nil, nil
	}))}
	id, err := h.resolveOptionalBuyerID(context.Background(), 0, 0)
	if err != nil || id != 0 {
		t.Errorf("resolveOptionalBuyerID(0, 0) = (%d, %v), want (0, nil)", id, err)
	}
}

func TestResolveOptionalBuyerID_BuyerRoleResolves(t *testing.T) {
	h := &Holder{BuyerID: buyerResolverForTest(9999)}
	id, err := h.resolveOptionalBuyerID(context.Background(), 42, 1)
	if err != nil || id != 9999 {
		t.Errorf("resolveOptionalBuyerID(42, 1) = (%d, %v), want (9999, nil)", id, err)
	}
}

func TestResolveOptionalBuyerID_NoCurrentBuyerFallsBackToAnonymous(t *testing.T) {
	h := &Holder{BuyerID: noBuyerResolverForTest()}
	id, err := h.resolveOptionalBuyerID(context.Background(), 42, 1)
	if err != nil || id != 0 {
		t.Errorf("resolveOptionalBuyerID(42, 1) with ErrNoCurrentBuyer = (%d, %v), want (0, nil) — anonymous fallback, not an error", id, err)
	}
}

func TestResolveOptionalBuyerID_BuyerBackendErrorPropagates(t *testing.T) {
	h := &Holder{BuyerID: buyerServiceDownResolverForTest()}
	_, err := h.resolveOptionalBuyerID(context.Background(), 42, 1)
	if err == nil {
		t.Error("expected backend error to propagate, got nil")
	}
}

func TestResolveOptionalBuyerID_SellerRoleResolvesCurrentStoreID(t *testing.T) {
	h := &Holder{Store: storeResolverForTest(8001)}
	id, err := h.resolveOptionalBuyerID(context.Background(), 42, 2)
	if err != nil || id != 8001 {
		t.Errorf("resolveOptionalBuyerID(42, 2) = (%d, %v), want (8001, nil)", id, err)
	}
}

func TestResolveOptionalBuyerID_NoCurrentSellerDegradesToAnonymous(t *testing.T) {
	h := &Holder{Store: noStoreResolverForTest()}
	id, err := h.resolveOptionalBuyerID(context.Background(), 42, 2)
	if err != nil || id != 0 {
		t.Errorf("resolveOptionalBuyerID(42, 2) with ErrNoCurrentStore = (%d, %v), want (0, nil) — anonymous fallback, not an error", id, err)
	}
}

// ---- loginRole 未定（#2897 讀側鏡像）----------------------------------------
//
// loginRole 未定時，讀側先試買家軸、再試賣家軸；兩軸皆無回 0（匿名），不擋頁面。

func TestResolveOptionalBuyerID_UnresolvedRoleResolvesBuyerAxis(t *testing.T) {
	h := &Holder{BuyerID: buyerResolverForTest(9999), Store: storeResolverForTest(8001)}
	id, err := h.resolveOptionalBuyerID(context.Background(), 42, 0)
	if err != nil || id != 9999 {
		t.Errorf("resolveOptionalBuyerID(42, 0) = (%d, %v), want (9999, nil) — 未定角色必須跨軸解析，不可當匿名", id, err)
	}
}

// 偏好順序與寫側一致（先買家後賣家）：下游 resolveForumIdentity 的預設分支就是
// 買家，兩側偏好一旦分岔，同一個使用者在讀與寫會拿到兩個不同的 id。
func TestResolveOptionalBuyerID_UnresolvedRolePrefersBuyerAxis(t *testing.T) {
	h := &Holder{BuyerID: buyerResolverForTest(9999), Store: storeResolverForTest(8001)}
	id, _ := h.resolveOptionalBuyerID(context.Background(), 42, 0)
	if id != 9999 {
		t.Errorf("resolveOptionalBuyerID(42, 0) = %d, want 9999（買家軸優先；拿到 8001 代表順序被倒過來了）", id)
	}
}

func TestResolveOptionalBuyerID_UnresolvedRoleFallsBackToStoreAxis(t *testing.T) {
	h := &Holder{BuyerID: noBuyerResolverForTest(), Store: storeResolverForTest(8001)}
	id, err := h.resolveOptionalBuyerID(context.Background(), 42, 0)
	if err != nil || id != 8001 {
		t.Errorf("resolveOptionalBuyerID(42, 0) 無 buyer stint = (%d, %v), want (8001, nil)", id, err)
	}
}

func TestResolveOptionalBuyerID_UnresolvedRoleNoStintOnEitherAxisIsAnonymous(t *testing.T) {
	h := &Holder{BuyerID: noBuyerResolverForTest(), Store: noStoreResolverForTest()}
	id, err := h.resolveOptionalBuyerID(context.Background(), 42, 0)
	if err != nil || id != 0 {
		t.Errorf("resolveOptionalBuyerID(42, 0) 兩軸皆無 = (%d, %v), want (0, nil) — 讀路徑永遠不因解析失敗而擋住頁面", id, err)
	}
}

func TestResolveOptionalBuyerID_SellerBackendErrorPropagates(t *testing.T) {
	h := &Holder{Store: storeServiceDownResolverForTest()}
	_, err := h.resolveOptionalBuyerID(context.Background(), 42, 2)
	if err == nil {
		t.Error("expected backend error to propagate, got nil")
	}
}

// ⚠️ 語意已翻轉（#2897）：這支原本還斷言「兩個 resolver 都不准被呼叫」——那是
// 舊行為（未定角色直接回 0，完全不解析）。寫側改成跨軸解析之後，讀側不同步就會
// 讓使用者看不到自己按的讚（見上方 ADR），所以現在**會**去問兩軸。
//
// 但它原本守的另一半仍然成立、而且是這支存在的理由：**任何情況下都不得回傳 raw
// account_id**。兩軸皆無時回 0（匿名）而不是 42，那一條沒有變，改寫後改為明寫。
func TestResolveOptionalBuyerID_UnresolvedRoleNeverReturnsRawAccountID(t *testing.T) {
	h := &Holder{BuyerID: noBuyerResolverForTest(), Store: noStoreResolverForTest()}
	id, err := h.resolveOptionalBuyerID(context.Background(), 42, 0)
	if err != nil {
		t.Fatalf("resolveOptionalBuyerID(42, 0) 不得回錯（讀路徑不擋頁面）: %v", err)
	}
	if id == 42 {
		t.Error("回傳了 raw account_id 42 —— 未定角色時絕不可把登入 principal 當成營運鍵送出")
	}
	if id != 0 {
		t.Errorf("resolveOptionalBuyerID(42, 0) = %d, want 0（兩軸皆無 ⇒ 匿名）", id)
	}
}

// ==================== resolveOptionalSellerFilterQuery ====================
// Mirrors resolveOptionalBuyerFilterQuery's tests in resolve_buyer_test.go —
// same D7 sentinel design, seller-side mirror added alongside
// resolveOptionalSellerFilterQuery itself (section D).

func setupResolveSellerFilterProbe(h *Holder) *gin.Engine {
	r := gin.New()
	r.GET("/probe", func(c *gin.Context) {
		id, ok := h.resolveOptionalSellerFilterQuery(c, c.Query("store_id"))
		if !ok {
			return
		}
		response.OK(c, gin.H{"resolved": id})
	})
	return r
}

func TestResolveOptionalSellerFilterQuery_Empty_NoFilter(t *testing.T) {
	h := &Holder{Store: storeResolverForTest(7001)}
	r := setupResolveSellerFilterProbe(h)

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

func TestResolveOptionalSellerFilterQuery_ResolvesSurrogate(t *testing.T) {
	h := &Holder{Store: storeResolverForTest(7001)}
	r := setupResolveSellerFilterProbe(h)

	req := httptest.NewRequest("GET", "/probe?store_id=99", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	if data["resolved"] != "7001" {
		t.Errorf("resolved = %v, want 7001 (resolved surrogate, not raw 99)", data["resolved"])
	}
}

func TestResolveOptionalSellerFilterQuery_NoCurrentSeller_ReturnsSentinel(t *testing.T) {
	h := &Holder{Store: noStoreResolverForTest()}
	r := setupResolveSellerFilterProbe(h)

	req := httptest.NewRequest("GET", "/probe?store_id=99", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 帳號無現任賣家：回哨兵值 "-1"（保證查無結果），不是略過篩選、也不是
	// 錯誤，鏡像 resolveOptionalBuyerFilterQuery 的行為。
	assertHTTPStatus(t, w, http.StatusOK)
	body := parseJSON(t, w.Body)
	data := body["data"].(map[string]interface{})
	if data["resolved"] != "-1" {
		t.Errorf("resolved = %v, want \"-1\" sentinel", data["resolved"])
	}
}

func TestResolveOptionalSellerFilterQuery_InvalidID_BadRequest(t *testing.T) {
	h := &Holder{Store: storeResolverForTest(7001)}
	r := setupResolveSellerFilterProbe(h)

	req := httptest.NewRequest("GET", "/probe?store_id=not-a-number", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
}
