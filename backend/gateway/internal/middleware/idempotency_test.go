package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// 本檔守的是 issue #3015：`Idempotency-Key` 曾經是 36 條寫入路由的必填 header，
// 而它**不做任何事**。
//
// 🔴 **有鑑別力的那一組是「同鍵 ＋ 不同 body」**（TestIdempotency_SameKeyDifferentBody_*）。
// 只測「同鍵 ＋ 同 body」會看到第二次被下游的業務去重擋掉（發票那組是 9306
// 「已存在相同的常用發票資料」），**看起來像冪等生效，實際上是巧合** —— 原始
// 回報就差點這樣判成 PASS。所以本檔的 handler 一律**計次**：斷言的是
// 「handler 有沒有被再跑一次」，不是「回應長什麼樣」。

const testAccountID = int64(2623900520)

// countingHandler 回一個 2xx，並記錄自己被叫了幾次、每次收到什麼 body。
type countingHandler struct {
	calls   atomic.Int64
	bodies  []string
	mu      sync.Mutex
	status  int
	release chan struct{} // 非 nil 時 handler 會卡住，用來製造「進行中」
}

func (h *countingHandler) handle(c *gin.Context) {
	n := h.calls.Add(1)
	buf := make([]byte, 4096)
	read, _ := c.Request.Body.Read(buf)
	h.mu.Lock()
	h.bodies = append(h.bodies, string(buf[:read]))
	h.mu.Unlock()
	if h.release != nil {
		<-h.release
	}
	st := h.status
	if st == 0 {
		st = http.StatusOK
	}
	c.JSON(st, gin.H{"code": 0, "message": "success", "data": gin.H{"profileId": fmt.Sprintf("%d", 7500000000000000000+n)}})
}

// newIdempotencyEngine 組一條與 routes_invoice.go 相同形狀的鏈：
// 先注入 accountId（正式鏈上由 JWTAuth 做），再掛 requireIdempotencyKey。
func newIdempotencyEngine(store *IdempotencyStore, h *countingHandler, accountID *int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/app/buyers/me/invoice_profiles")
	g.Use(func(c *gin.Context) {
		if accountID != nil {
			c.Set("accountId", *accountID)
		}
		c.Next()
	}, requireIdempotencyKey(store))
	g.POST("", h.handle)
	g.DELETE("/:profile_id", h.handle)
	g.GET("", h.handle)
	return r
}

func post(t *testing.T, r *gin.Engine, path, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set(IdempotencyKeyHeader, key)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func bizCode(t *testing.T, w *httptest.ResponseRecorder) int32 {
	t.Helper()
	var env struct {
		Code int32 `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("回應不是 envelope JSON：%s", w.Body.String())
	}
	return env.Code
}

// 🔴 判定依據：同一把鍵送**不同**內容，handler 不得被跑第二次。
// dev 實測（issue #3015 步驟 B）修復前是：200 ＋ 新的 profileId ＋ 有效列 2→3。
func TestIdempotency_SameKeyDifferentBody_DoesNotExecuteTwice(t *testing.T) {
	h := &countingHandler{}
	acc := testAccountID
	r := newIdempotencyEngine(NewIdempotencyStore(0, 0), h, &acc)

	w1 := post(t, r, "/app/buyers/me/invoice_profiles", "K2-aaaaaaaa", `{"label":"A"}`)
	if w1.Code != http.StatusOK {
		t.Fatalf("第一次應 200，實際 %d：%s", w1.Code, w1.Body.String())
	}

	w2 := post(t, r, "/app/buyers/me/invoice_profiles", "K2-aaaaaaaa", `{"label":"B"}`)
	if got := h.calls.Load(); got != 1 {
		t.Fatalf("handler 被執行 %d 次，同鍵不同內容不得再執行（這就是 #3015 的病）", got)
	}
	if w2.Code != http.StatusConflict {
		t.Fatalf("同鍵不同內容應回 409，實際 %d：%s", w2.Code, w2.Body.String())
	}
	if code := bizCode(t, w2); code != 9350 {
		t.Fatalf("同鍵不同內容應回業務碼 9350，實際 %d：%s", code, w2.Body.String())
	}
}

// 同鍵同 body ⇒ 回放上一次的回應本體，且 handler 不再執行。
// 斷言 body **完全相同**：靠下游業務去重的話第二次會是另一個碼（例如 9306），
// 那正是「看起來像冪等」的假象。
func TestIdempotency_SameKeySameBody_ReplaysWithoutReexecuting(t *testing.T) {
	h := &countingHandler{}
	acc := testAccountID
	r := newIdempotencyEngine(NewIdempotencyStore(0, 0), h, &acc)

	w1 := post(t, r, "/app/buyers/me/invoice_profiles", "K1-aaaaaaaa", `{"label":"A"}`)
	w2 := post(t, r, "/app/buyers/me/invoice_profiles", "K1-aaaaaaaa", `{"label":"A"}`)

	if got := h.calls.Load(); got != 1 {
		t.Fatalf("handler 被執行 %d 次，同鍵同內容必須回放而不是重跑", got)
	}
	if w2.Code != w1.Code || w2.Body.String() != w1.Body.String() {
		t.Fatalf("回放不一致：第一次 %d %s；第二次 %d %s",
			w1.Code, w1.Body.String(), w2.Code, w2.Body.String())
	}
	if w2.Header().Get(IdempotencyReplayHeader) != "true" {
		t.Fatalf("回放必須帶 %s: true，實際 header = %v", IdempotencyReplayHeader, w2.Header())
	}
	if w1.Header().Get(IdempotencyReplayHeader) != "" {
		t.Fatal("第一次不是回放，不得帶回放標頭")
	}
}

func TestIdempotency_DifferentKeys_AreIndependent(t *testing.T) {
	h := &countingHandler{}
	acc := testAccountID
	r := newIdempotencyEngine(NewIdempotencyStore(0, 0), h, &acc)

	post(t, r, "/app/buyers/me/invoice_profiles", "K3-aaaaaaaa", `{"label":"A"}`)
	w2 := post(t, r, "/app/buyers/me/invoice_profiles", "K4-bbbbbbbb", `{"label":"A"}`)

	if got := h.calls.Load(); got != 2 {
		t.Fatalf("不同鍵必須各自獨立執行，實際只執行 %d 次", got)
	}
	if w2.Code != http.StatusOK {
		t.Fatalf("第二把鍵應正常執行，實際 %d", w2.Code)
	}
}

// 同一把鍵在上一次還沒跑完時再送 ⇒ 9351，且不得並行執行。
func TestIdempotency_ConcurrentSameKey_ReturnsInProgress(t *testing.T) {
	h := &countingHandler{release: make(chan struct{})}
	acc := testAccountID
	r := newIdempotencyEngine(NewIdempotencyStore(0, 0), h, &acc)
	// Cleanup 而不是函式尾端 close：本測試在「冪等被拿掉」的變異下第二個請求會真的
	// 進到 handler 並卡在 <-release，若只在尾端 close，t.Fatalf 就會讓它永遠卡住 ——
	// 變異驗證會變成 40 秒逾時 panic 而不是一則清楚的 FAIL。
	var once sync.Once
	closeRelease := func() { once.Do(func() { close(h.release) }) }
	t.Cleanup(closeRelease)
	// 逃生閥：在「冪等被拿掉」的變異下，第二個請求會真的進到 handler 並卡在
	// <-release，而主 goroutine 正卡在 ServeHTTP 裡 ⇒ 沒有這個計時器，變異驗證
	// 會退化成整包測試逾時 panic，而不是一則指得出哪裡壞掉的 FAIL。
	time.AfterFunc(2*time.Second, closeRelease)

	started := make(chan struct{})
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/app/buyers/me/invoice_profiles",
			strings.NewReader(`{"label":"A"}`))
		req.Header.Set(IdempotencyKeyHeader, "K5-aaaaaaaa")
		close(started)
		r.ServeHTTP(httptest.NewRecorder(), req)
	}()
	<-started
	for h.calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}

	w := post(t, r, "/app/buyers/me/invoice_profiles", "K5-aaaaaaaa", `{"label":"A"}`)
	if w.Code != http.StatusConflict || bizCode(t, w) != 9351 {
		t.Fatalf("進行中的同鍵應回 409/9351，實際 %d %s", w.Code, w.Body.String())
	}
	if got := h.calls.Load(); got != 1 {
		t.Fatalf("進行中不得並行執行，handler 執行了 %d 次", got)
	}
	closeRelease()
}

// 非 2xx 不留紀錄：使用者修好問題後用同一把鍵重送，必須真的重跑。
func TestIdempotency_NonSuccessIsNotCached(t *testing.T) {
	h := &countingHandler{status: http.StatusUnprocessableEntity}
	acc := testAccountID
	r := newIdempotencyEngine(NewIdempotencyStore(0, 0), h, &acc)

	post(t, r, "/app/buyers/me/invoice_profiles", "K6-aaaaaaaa", `{"label":"A"}`)
	h.status = http.StatusOK
	w2 := post(t, r, "/app/buyers/me/invoice_profiles", "K6-aaaaaaaa", `{"label":"A"}`)

	if got := h.calls.Load(); got != 2 {
		t.Fatalf("上一次失敗的鍵必須可以重跑，實際只執行 %d 次", got)
	}
	if w2.Code != http.StatusOK {
		t.Fatalf("重送應正常執行，實際 %d：%s", w2.Code, w2.Body.String())
	}
}

// 作用域帶 accountId：不同帳號用同一把鍵不得互相回放（否則是跨帳號資料外洩）。
func TestIdempotency_ScopeIncludesAccount(t *testing.T) {
	h := &countingHandler{}
	acc := testAccountID
	r := newIdempotencyEngine(NewIdempotencyStore(0, 0), h, &acc)

	w1 := post(t, r, "/app/buyers/me/invoice_profiles", "K7-aaaaaaaa", `{"label":"A"}`)
	acc = int64(2688637900)
	w2 := post(t, r, "/app/buyers/me/invoice_profiles", "K7-aaaaaaaa", `{"label":"A"}`)

	if got := h.calls.Load(); got != 2 {
		t.Fatalf("不同帳號必須各自執行，實際只執行 %d 次", got)
	}
	if w1.Body.String() == w2.Body.String() {
		t.Fatal("不同帳號拿到同一份回應 = 跨帳號回放")
	}
}

// 路徑參數進指紋：同一把鍵刪兩個不同 profile_id 是兩份不同請求 ⇒ 9350，
// 不得互相回放（回放會讓第二筆「看起來刪掉了」而其實沒有）。
func TestIdempotency_PathParamsAreInFingerprint(t *testing.T) {
	h := &countingHandler{}
	acc := testAccountID
	r := newIdempotencyEngine(NewIdempotencyStore(0, 0), h, &acc)

	do := func(id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodDelete, "/app/buyers/me/invoice_profiles/"+id, nil)
		req.Header.Set(IdempotencyKeyHeader, "K8-aaaaaaaa")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	do("111")
	w2 := do("222")
	if got := h.calls.Load(); got != 1 {
		t.Fatalf("同鍵不同路徑參數不得執行第二次，實際 %d 次", got)
	}
	if w2.Code != http.StatusConflict || bizCode(t, w2) != 9350 {
		t.Fatalf("同鍵不同路徑參數應回 409/9350，實際 %d %s", w2.Code, w2.Body.String())
	}
}

// 格式驗證維持原行為（本次改動不得放寬它）。
func TestIdempotency_FormatValidationUnchanged(t *testing.T) {
	h := &countingHandler{}
	acc := testAccountID
	r := newIdempotencyEngine(NewIdempotencyStore(0, 0), h, &acc)

	for _, key := range []string{"", "short", strings.Repeat("a", 129), "has space", "中文"} {
		w := post(t, r, "/app/buyers/me/invoice_profiles", key, `{}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("key=%q 應回 400，實際 %d", key, w.Code)
		}
	}
	if got := h.calls.Load(); got != 0 {
		t.Fatalf("格式不符不得執行 handler，實際 %d 次", got)
	}
}

// GET 不受管：middleware 掛在群組上，讀取路由不得因此被要求 header。
func TestIdempotency_ReadMethodsPassThrough(t *testing.T) {
	h := &countingHandler{}
	acc := testAccountID
	r := newIdempotencyEngine(NewIdempotencyStore(0, 0), h, &acc)

	req := httptest.NewRequest(http.MethodGet, "/app/buyers/me/invoice_profiles", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || h.calls.Load() != 1 {
		t.Fatalf("GET 應直接放行，實際 %d／執行 %d 次", w.Code, h.calls.Load())
	}
}

// 沒有可信 accountId ⇒ 放行（見 middleware doc「擋不住什麼」①）。
// 這一條是刻意的 fail-open，寫成測試是為了讓它「被改掉時有人知道」，
// 而不是留成沒人發現的預設行為。
func TestIdempotency_NoAccountFallsThrough(t *testing.T) {
	h := &countingHandler{}
	r := newIdempotencyEngine(NewIdempotencyStore(0, 0), h, nil)

	post(t, r, "/app/buyers/me/invoice_profiles", "K9-aaaaaaaa", `{"label":"A"}`)
	post(t, r, "/app/buyers/me/invoice_profiles", "K9-aaaaaaaa", `{"label":"B"}`)
	if got := h.calls.Load(); got != 2 {
		t.Fatalf("無 accountId 時應放行兩次（已知 fail-open），實際 %d 次", got)
	}
}

// handler 讀得到 body：middleware 讀完必須塞回去。
func TestIdempotency_HandlerStillSeesBody(t *testing.T) {
	h := &countingHandler{}
	acc := testAccountID
	r := newIdempotencyEngine(NewIdempotencyStore(0, 0), h, &acc)

	post(t, r, "/app/buyers/me/invoice_profiles", "KA-aaaaaaaa", `{"label":"A"}`)
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.bodies) != 1 || h.bodies[0] != `{"label":"A"}` {
		t.Fatalf("handler 收到的 body = %q，應為原始 body", h.bodies)
	}
}

func TestIdempotencyStore_ExpiredRecordIsReExecuted(t *testing.T) {
	store := NewIdempotencyStore(0, time.Minute)
	now := time.Now()
	store.now = func() time.Time { return now }

	if out, _ := store.Begin("s", "fp"); out != idemProceed {
		t.Fatal("第一次應取得執行權")
	}
	store.Commit("s", 200, "application/json", []byte(`{}`))
	if out, _ := store.Begin("s", "fp"); out != idemReplay {
		t.Fatal("TTL 內應回放")
	}
	now = now.Add(2 * time.Minute)
	if out, _ := store.Begin("s", "fp"); out != idemProceed {
		t.Fatal("TTL 過後應重新執行")
	}
}

func TestIdempotencyStore_EvictsOldestWhenFull(t *testing.T) {
	store := NewIdempotencyStore(2, time.Hour)
	for _, s := range []string{"a", "b", "c"} {
		store.Begin(s, "fp")
		store.Commit(s, 200, "application/json", []byte(`{}`))
	}
	if store.Len() != 2 {
		t.Fatalf("容量上限 2，實際留了 %d 筆", store.Len())
	}
	if out, _ := store.Begin("a", "fp"); out != idemProceed {
		t.Fatal("最舊的一筆應已被逐出（退化成沒有冪等，而不是回錯的結果）")
	}
}

// 進行中的紀錄不得被容量逐出：丟掉它等於讓第二個請求同時開跑。
func TestIdempotencyStore_InFlightNotEvicted(t *testing.T) {
	store := NewIdempotencyStore(1, time.Hour)
	store.Begin("inflight", "fp")
	store.Begin("other", "fp")
	store.Commit("other", 200, "application/json", []byte(`{}`))
	if out, _ := store.Begin("inflight", "fp"); out != idemInFlight {
		t.Fatalf("進行中的紀錄被逐出了，outcome=%v", out)
	}
}

// 回應太大就不留紀錄（回放半截 JSON 比不回放危險）。
func TestIdempotencyStore_OversizedBodyNotStored(t *testing.T) {
	store := NewIdempotencyStore(0, time.Hour)
	store.Begin("big", "fp")
	store.Commit("big", 200, "application/json", make([]byte, idempotencyMaxBodyBytes+1))
	if out, _ := store.Begin("big", "fp"); out != idemProceed {
		t.Fatal("超大回應不得留下可回放的紀錄")
	}
}
