package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	sellerconnect "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
)

// IPv6 以 /64 聚合：同一個子網換位址不會多出額度。
func TestVisitorSubjectAggregatesIPv6(t *testing.T) {
	for in, want := range map[string]string{
		"192.0.2.1":             "v:192.0.2.1",
		"::ffff:192.0.2.1":      "v:192.0.2.1",
		"2001:db8:1:2:aaaa::1":  "v:2001:db8:1:2::/64",
		"2001:db8:1:2:bbbb::99": "v:2001:db8:1:2::/64",
		"2001:db8:1:3::1":       "v:2001:db8:1:3::/64",
		"not-an-ip":             "v:invalid",
	} {
		if got := visitorSubject(in); got != want {
			t.Errorf("visitorSubject(%q) = %q，預期 %q", in, got, want)
		}
	}
}

// 計數表滿了：先清過期的；還是滿的時候新的身分不計數，既有的計數不被歸零（大量換位址不能替別人解除限流）。
func TestRateLimiterKeepsCountsWhenFull(t *testing.T) {
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	l := newRateLimiter(func() time.Time { return now })
	for range 3 {
		l.allow("victim", 3)
	}
	if ok, _ := l.allow("victim", 3); ok {
		t.Fatal("前提：victim 已經超過額度")
	}
	for i := range maxRateKeys - 1 {
		l.allow(fmt.Sprintf("flood-%d", i), 3)
	}
	if ok, _ := l.allow("newcomer", 3); !ok {
		t.Fatal("滿了之後新的身分應該放行")
	}
	if ok, _ := l.allow("victim", 3); ok {
		t.Fatal("滿了之後既有的計數被歸零")
	}
	// 時間窗過了：過期的列清掉，新的身分重新計數。
	now = now.Add(rateWindow)
	if ok, _ := l.allow("newcomer", 1); !ok {
		t.Fatal("新的時間窗應該放行")
	}
	if n := len(l.m); n != 1 {
		t.Fatalf("過期的列沒有清掉：還有 %d 列", n)
	}
}

// 帳號的額度各自指定時間窗：一小時的額度不會在入口的一分鐘窗過後歸零，不同的 key（帳號、方法）互不影響，窗過了才重新計數。
func TestAccountLimiterUsesTheWindowOfEachCall(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	l := newAccountLimiter(func() time.Time { return now })
	for range 5 {
		if ok, _ := l.Allow(httpseller.Limit{Key: "submit|7", Quota: 5, Window: time.Hour}); !ok {
			t.Fatal("額度內的請求被擋下")
		}
	}
	ok, wait := l.Allow(httpseller.Limit{Key: "submit|7", Quota: 5, Window: time.Hour})
	if ok || wait != time.Hour {
		t.Fatalf("ok=%v wait=%v", ok, wait)
	}
	if ok, _ := l.Allow(httpseller.Limit{Key: "submit|8", Quota: 5, Window: time.Hour}); !ok {
		t.Fatal("別的帳號被這個帳號的計數影響")
	}
	if ok, _ := l.Allow(httpseller.Limit{Key: "lookup|7", Quota: 5, Window: time.Minute}); !ok {
		t.Fatal("同一個帳號的另一個方法被影響")
	}
	now = now.Add(rateWindow + time.Second)
	if ok, wait := l.Allow(httpseller.Limit{Key: "submit|7", Quota: 5, Window: time.Hour}); ok || wait <= 0 || wait > time.Hour {
		t.Fatalf("入口的一分鐘窗過後，一小時的額度不該歸零：ok=%v wait=%v", ok, wait)
	}
	now = now.Add(time.Hour)
	if ok, _ := l.Allow(httpseller.Limit{Key: "submit|7", Quota: 5, Window: time.Hour}); !ok {
		t.Fatal("時間窗過了應該重新計數")
	}
}

// 帳號的計數表滿了：新的帳號被拒絕（fail closed），既有的帳號照常計數；過了時間窗清掉過期的列之後新的帳號可以計數。
func TestAccountLimiterFailsClosedWhenTheTableIsFull(t *testing.T) {
	now := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	l := newAccountLimiter(func() time.Time { return now })
	for i := range maxRateKeys {
		if ok, _ := l.Allow(httpseller.Limit{Key: fmt.Sprintf("flood-%d", i), Quota: 5, Window: time.Hour}); !ok {
			t.Fatal("表還沒滿就被拒絕")
		}
	}
	if ok, wait := l.Allow(httpseller.Limit{Key: "newcomer", Quota: 5, Window: time.Hour}); ok || wait <= 0 {
		t.Fatalf("表滿了，新的帳號應該被拒絕：ok=%v wait=%v", ok, wait)
	}
	if ok, _ := l.Allow(httpseller.Limit{Key: "flood-0", Quota: 5, Window: time.Hour}); !ok {
		t.Fatal("既有的帳號不該被拒絕")
	}
	now = now.Add(time.Hour)
	if ok, _ := l.Allow(httpseller.Limit{Key: "newcomer", Quota: 5, Window: time.Hour}); !ok {
		t.Fatal("時間窗過了，過期的列清掉之後新的帳號應該可以計數")
	}
	// IP 層維持放行。
	ip := newRateLimiter(func() time.Time { return now })
	for i := range maxRateKeys {
		ip.allow(fmt.Sprintf("flood-%d", i), 3)
	}
	if ok, _ := ip.allow("newcomer", 3); !ok {
		t.Fatal("IP 層滿了之後新的身分應該放行")
	}
}

// 匿名的食安通報有兩個時間窗（10 分鐘 5 次、1 小時 10 次），以來源 IP 計數：全部在額度內才一起計，被拒絕的請求都不計（不扣
// 另一個時間窗）；RetryInfo 是最久的等待時間。1 小時用完之後，新的 10 分鐘也送不出去。登入的通報不在入口計（帳號額度在
// handler），其他方法不受影響。
func TestReportSubmissionIsLimitedByTwoWindows(t *testing.T) {
	now := entryNow
	path := sellerconnect.SellerReportPublicServiceSubmitFoodSafetyReportProcedure
	svcPath, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	h, err := assemble(Deps{
		Now: func() time.Time { return now }, Origins: []string{testOrigin},
		OriginAuth: OriginAuth{Main: []byte("main-secret"), CI: []byte("ci-secret")}, Logger: slog.New(slog.DiscardHandler),
	}, []service{{"/" + svcPath + "/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })}})
	if err != nil {
		t.Fatal(err)
	}
	post := func(ip string) *httptest.ResponseRecorder {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader("{}"))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = ip + ":4000"
		return do(h, r)
	}
	window := func(name, retry string) {
		t.Helper()
		for i := range 5 {
			if rec := post("192.0.2.1"); rec.Code != http.StatusOK {
				t.Fatalf("%s 第 %d 次 = %d", name, i+1, rec.Code)
			}
		}
		rec := post("192.0.2.1")
		if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), `"retryDelay":"`+retry+`"`) {
			t.Fatalf("%s 第 6 次 = %d %q", name, rec.Code, rec.Body)
		}
		// 文字是共用文案目錄的 RESOURCE_EXHAUSTED，不是入口自己寫的一句。
		if text := rpc.CommonText(t.Context(), connect.CodeResourceExhausted).Text; text != "操作過於頻繁，請稍後再試" || !strings.Contains(rec.Body.String(), text) {
			t.Fatalf("%s 第 6 次的文字 = %q，預期目錄的 %q", name, rec.Body, text)
		}
	}
	window("第一個 10 分鐘", "600s")
	if rec := post("192.0.2.2"); rec.Code != http.StatusOK {
		t.Fatalf("另一個 IP 不該被擋：%d", rec.Code)
	}
	now = now.Add(10 * time.Minute)
	// 被拒絕的第 6 次沒有扣 1 小時的額度：這 10 分鐘還能送 5 次；第 6 次兩個時間窗都超過，RetryInfo 是較久的 1 小時那個。
	window("第二個 10 分鐘", "3000s")
	now = now.Add(10 * time.Minute)
	if rec := post("192.0.2.1"); rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), `"retryDelay":"2400s"`) {
		t.Fatalf("1 小時的額度用完，要等到這個小時結束：%d %q", rec.Code, rec.Body)
	}
	if !slices.Equal(tiersOf(sellerconnect.SellerReportAppServiceCreateReportProcedure), []rateTier{tierDefault}) {
		t.Fatal("登入的通報在入口照一般級別，帳號額度在 handler")
	}
}

// 多個時間窗一起計：全部在額度內才都記一次；任何一個超過就都不記，回傳最久的等待時間。被長窗拒絕的請求不佔短窗的額度。
func TestAllowAllCountsEveryWindowOrNone(t *testing.T) {
	now := entryNow
	l := newRateLimiter(func() time.Time { return now })
	short := rateRequest{key: "s", limit: 3, window: 10 * time.Minute}
	long := rateRequest{key: "l", limit: 4, window: time.Hour}
	for i := range 3 {
		if ok, _ := l.allowAll(short, long); !ok {
			t.Fatalf("第 %d 次", i+1)
		}
	}
	if ok, wait := l.allowAll(short, long); ok || wait != 10*time.Minute {
		t.Fatalf("短窗超過：ok=%v wait=%v", ok, wait)
	}
	now = now.Add(10 * time.Minute)
	if ok, _ := l.allowAll(short, long); !ok {
		t.Fatal("被短窗拒絕的那次沒有佔長窗：長窗還剩 1 次")
	}
	if ok, wait := l.allowAll(short, long); ok || wait != 50*time.Minute {
		t.Fatalf("長窗用完：ok=%v wait=%v", ok, wait)
	}
	if ok, _ := l.allowAll(short); !ok {
		t.Fatal("被長窗拒絕的請求不佔短窗：短窗只用了 1 次")
	}
	if ok, _ := l.allowAll(short); !ok {
		t.Fatal("短窗第 3 次")
	}
	if ok, _ := l.allowAll(short); ok {
		t.Fatal("短窗第 4 次要被拒絕")
	}
}
