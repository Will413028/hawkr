package payuni

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// 測試用的金鑰：長度符合規格，不是任何真實的金鑰。
const (
	testKey = "0123456789abcdef0123456789abcdef" // gitleaks:allow（虛構值 0123…）
	testIV  = "0123456789abcdef"
)

func testClient(t *testing.T, h http.HandlerFunc, timeout time.Duration) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(Config{BaseURL: srv.URL, AgentID: "agent", HashKey: testKey, HashIV: testIV, Timeout: timeout})
	if err != nil {
		t.Fatal(err)
	}
	return c, srv
}

// received 解出伺服器收到的、解密後的欄位。
func received(t *testing.T, r *http.Request) url.Values {
	t.Helper()
	if err := r.ParseForm(); err != nil {
		t.Fatal(err)
	}
	enc := r.PostForm.Get("EncryptInfo")
	if r.PostForm.Get("AgentID") != "agent" || r.PostForm.Get("HashInfo") != hashInfo(enc, []byte(testKey), []byte(testIV)) {
		t.Fatalf("envelope=%v", r.PostForm)
	}
	plain, err := decrypt(enc, []byte(testKey), []byte(testIV))
	if err != nil {
		t.Fatal(err)
	}
	v, err := url.ParseQuery(plain)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestNewRequiresEveryField(t *testing.T) {
	ok := Config{BaseURL: "https://example.invalid", AgentID: "a", HashKey: testKey, HashIV: testIV}
	if _, err := New(ok); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Config){
		"no url":      func(c *Config) { c.BaseURL = "" },
		"no agent":    func(c *Config) { c.AgentID = "" },
		"no key":      func(c *Config) { c.HashKey = "" },
		"no iv":       func(c *Config) { c.HashIV = "" },
		"short key":   func(c *Config) { c.HashKey = "short" },
		"short iv":    func(c *Config) { c.HashIV = "short" },
		"no scheme":   func(c *Config) { c.BaseURL = "example.invalid" },
		"bad scheme":  func(c *Config) { c.BaseURL = "ftp://example.invalid" },
		"no hostname": func(c *Config) { c.BaseURL = "https://" },
	} {
		c := ok
		mutate(&c)
		if _, err := New(c); err == nil {
			t.Errorf("%s：應該失敗", name)
		}
	}
}

func TestEncryptDecryptRoundTripAndTamper(t *testing.T) {
	enc, err := encrypt("MerID=HAWK1&Name=x", []byte(testKey), []byte(testIV)) //nolint:gosec // PayUni 的規格要求固定的 IV（由設定給），這是測試用的值
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := decrypt(enc, []byte(testKey), []byte(testIV)); err != nil || plain != "MerID=HAWK1&Name=x" {
		t.Fatalf("plain=%q err=%v", plain, err)
	}
	// 換一把金鑰、或改一個字元都解不開（GCM 的 tag）。
	if _, err := decrypt(enc, []byte(strings.Repeat("x", keyLen)), []byte(testIV)); err == nil {
		t.Error("wrong key decrypted")
	}
	if _, err := decrypt(enc[:len(enc)-2]+"00", []byte(testKey), []byte(testIV)); err == nil {
		t.Error("tampered ciphertext decrypted")
	}
}

func TestBindMerchantSendsEncryptedFieldsAndOmitsEmptyOptionals(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/bind_merchant" {
			t.Errorf("path=%s", r.URL.Path)
		}
		v := received(t, r)
		if v.Get("MerID") != "HAWK123" || v.Get("MerType") != "2" || v.Get("MCID") != "" || v.Has("MBCName") || !v.Has("Name") {
			t.Errorf("fields=%v", v)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"Status": "SUCCESS"})
	}, 0)
	if err := c.BindMerchant(context.Background(), BindMerchantRequest{MerID: "HAWK123", MerType: 2, Name: "王小明"}); err != nil {
		t.Fatal(err)
	}
}

func TestBusinessFailureIsAnAPIErrorButTransportFailureIsNot(t *testing.T) {
	c, srv := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"Status": "AGENT04017", "Message": "商店代號已存在"})
	}, 0)
	err := c.BindMerchant(context.Background(), BindMerchantRequest{MerID: "HAWK1"})
	if apiErr, ok := AsAPIError(err); !ok || apiErr.Code != "AGENT04017" {
		t.Fatalf("err=%v", err)
	}
	// 連不上：是錯誤，而且不是業務失敗（D16：舊實作把它轉成 Success:false 而且沒有錯誤碼）。
	srv.Close()
	err = c.BindMerchant(context.Background(), BindMerchantRequest{MerID: "HAWK1"})
	if err == nil {
		t.Fatal("transport failure returned no error")
	}
	if _, ok := AsAPIError(err); ok {
		t.Fatalf("transport failure classified as an API error: %v", err)
	}
}

func TestNon200AndMalformedBodiesAreErrorsWithoutLeakingTheBody(t *testing.T) {
	for name, h := range map[string]http.HandlerFunc{
		"500":       func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "secret-in-body", 500) },
		"not json":  func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>secret-in-body")) },
		"no status": func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) },
	} {
		c, _ := testClient(t, h, 0)
		err := c.UpdateMerchant(context.Background(), UpdateMerchantRequest{MerID: "HAWK1"})
		if err == nil || strings.Contains(err.Error(), "secret-in-body") {
			t.Errorf("%s：err=%v", name, err)
		}
		if _, ok := AsAPIError(err); ok {
			t.Errorf("%s：被當成業務失敗", name)
		}
	}
}

func TestEveryCallHasADeadline(t *testing.T) {
	block := make(chan struct{})
	c, _ := testClient(t, func(http.ResponseWriter, *http.Request) { <-block }, 50*time.Millisecond)
	t.Cleanup(func() { close(block) })
	start := time.Now()
	if err := c.UpdateMerchant(context.Background(), UpdateMerchantRequest{MerID: "HAWK1"}); err == nil {
		t.Fatal("hanging server returned no error")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("call took %v", time.Since(start))
	}
}

func TestUpdateMerchantIsAPartialUpdate(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		v := received(t, r)
		if len(v) != 3 || v.Get("MerID") != "HAWK9" || v.Get("MerEmail") != "a@example.com" || v.Get("MerAddrPost") != "100" {
			t.Errorf("fields=%v", v)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"Status": "SUCCESS"})
	}, 0)
	if err := c.UpdateMerchant(context.Background(), UpdateMerchantRequest{MerID: "HAWK9", MerEmail: "a@example.com", MerAddrPost: "100"}); err != nil {
		t.Fatal(err)
	}
}

func TestMerchantStatusDecryptsAndUsesPayUniFieldNames(t *testing.T) {
	plain := "Status=SUCCESS&MerID=HAWK7&MerStatus=1&MemberStatus=5&Reason=%E8%A3%9C%E4%BB%B6&CREDIT=1&ApplePay=9&GooglePay=9&ATM=1"
	enc, err := encrypt(plain, []byte(testKey), []byte(testIV)) //nolint:gosec // 同上
	if err != nil {
		t.Fatal(err)
	}
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if received(t, r).Get("MerID") != "HAWK7" {
			t.Error("MerID not sent")
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"EncryptInfo": enc, "HashInfo": "ignored"})
	}, 0)
	got, err := c.MerchantStatus(context.Background(), "HAWK7")
	if err != nil {
		t.Fatal(err)
	}
	want := MerchantStatus{MerID: "HAWK7", MerStatus: "1", MemberStatus: "5", Reason: "補件", Credit: "1", ApplePay: "9", GooglePay: "9", ATM: "1"}
	if got != want {
		t.Fatalf("got=%+v", got)
	}
}

func TestMerchantStatusBusinessFailure(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"Status": "AGENT04000", "Message": "查無商店"})
	}, 0)
	if _, err := c.MerchantStatus(context.Background(), "HAWK1"); err == nil {
		t.Fatal("expected an error")
	} else if apiErr, ok := AsAPIError(err); !ok || apiErr.Code != "AGENT04000" {
		t.Fatalf("err=%v", err)
	}
}

// 限流：token bucket，burst 為 1，呼叫以固定間隔排隊（大量同時到達也不會一次衝出去）；排不進 deadline 之前的立刻失敗。
func TestLimiterSpacesCallsAndRespectsTheDeadline(t *testing.T) {
	l := newLimiter(20) // 每 50ms 一次
	start := time.Now()
	for range 4 {
		if err := l.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed < 140*time.Millisecond {
		t.Fatalf("4 次呼叫只花了 %v，間隔沒有生效", elapsed)
	}
	slow := newLimiter(1)
	if err := slow.Wait(context.Background()); err != nil { // 第一次在空的時段
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start = time.Now()
	if err := slow.Wait(ctx); err == nil || time.Since(start) > 40*time.Millisecond {
		t.Fatalf("排不進 deadline 應該立刻失敗：err=%v elapsed=%v", err, time.Since(start))
	}
	if got := newLimiter(0).Limit(); got != DefaultMaxQPS {
		t.Fatalf("預設上限=%v", got)
	}
}

// 每一種呼叫都經過同一個限流器：bucket 空了之後，bind、update、查詢都在排不進 deadline 時失敗、不會送出。
func TestEveryCallGoesThroughTheLimiter(t *testing.T) {
	var hits atomic.Int32
	c, _ := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]string{"Status": "SUCCESS"})
	}, 0)
	c.limit = rate.NewLimiter(rate.Every(time.Hour), 1)
	if err := c.BindMerchant(context.Background(), BindMerchantRequest{MerID: "HAWK1"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.UpdateMerchant(ctx, UpdateMerchantRequest{MerID: "HAWK1"}); err == nil {
		t.Error("update 沒有經過限流器")
	}
	if _, err := c.MerchantStatus(ctx, "HAWK1"); err == nil {
		t.Error("查詢沒有經過限流器")
	}
	if err := c.BindMerchant(ctx, BindMerchantRequest{MerID: "HAWK1"}); err == nil {
		t.Error("bind 沒有經過限流器")
	}
	if hits.Load() != 1 {
		t.Fatalf("送出 %d 次，預期只有第一次", hits.Load())
	}
}

// 代理商文件 §7.3.3：ContactTel、MerTel（與企業的 MBCTel）要送「區碼-電話號碼」，我們存的是只有數字的電話；bind 與 update 都一樣。
func TestPhonesAreSentInThePayUniAreaCodeFormat(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0229010572", "02-29010572"}, {"0372123456", "037-2123456"}, {"0826123456", "0826-123456"}, {"0298887777#123", "02-98887777#123"},
	}
	for _, tc := range cases {
		var bind, update url.Values
		c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "bind_merchant") {
				bind = received(t, r)
			} else {
				update = received(t, r)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"Status": "SUCCESS"})
		}, 0)
		if err := c.BindMerchant(context.Background(), BindMerchantRequest{MerID: "HAWK1", MerTel: tc.in, ContactTel: tc.in, MBCTel: tc.in}); err != nil {
			t.Fatal(err)
		}
		if err := c.UpdateMerchant(context.Background(), UpdateMerchantRequest{MerID: "HAWK1", MerTel: tc.in, ContactTel: tc.in}); err != nil {
			t.Fatal(err)
		}
		for name, v := range map[string]url.Values{"bind": bind, "update": update} {
			for _, field := range []string{"MerTel", "ContactTel"} {
				if v.Get(field) != tc.want {
					t.Errorf("%s 的 %s：%q，要 %q", name, field, v.Get(field), tc.want)
				}
			}
		}
		if bind.Get("MBCTel") != tc.want {
			t.Errorf("MBCTel：%q，要 %q", bind.Get("MBCTel"), tc.want)
		}
	}
	// 認不得的格式原樣送出，由 PayUni 拒絕；沒有值的欄位不送。
	var got url.Values
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = received(t, r)
		_ = json.NewEncoder(w).Encode(map[string]string{"Status": "SUCCESS"})
	}, 0)
	if err := c.UpdateMerchant(context.Background(), UpdateMerchantRequest{MerID: "HAWK1", MerTel: "abc"}); err != nil || got.Get("MerTel") != "abc" || got.Has("ContactTel") {
		t.Fatalf("got=%v err=%v", got, err)
	}
}
