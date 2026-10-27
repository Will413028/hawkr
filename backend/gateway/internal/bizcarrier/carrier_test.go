package bizcarrier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

const (
	methodA = "/hawkr.invoice.InvoiceService/HealthCheckProviderAccountForAdmin"
	methodB = "/hawkr.seller.SellerService/GetSellerSimple"
)

// runWithCarrier 起一個掛了 Middleware 的最小 engine，把 fn 當成 handler 跑一次。
func runWithCarrier(t *testing.T, fn gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())
	r.GET("/probe", fn)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))
	return w
}

// Arm → Emit 是本機制的主幹：碼掛得上、取得下、取完就空。
func TestArmThenEmit(t *testing.T) {
	runWithCarrier(t, func(c *gin.Context) {
		if !Arm(c.Request.Context(), methodA, 9331, "憑證驗證失敗") {
			t.Fatal("Arm 在有 carrier 的請求上必須成功")
		}
		code, msg, ok := Emit(c, 0)
		if !ok || code != 9331 || msg != "憑證驗證失敗" {
			t.Fatalf("Emit(0) = (%d, %q, %v), want (9331, 憑證驗證失敗, true)", code, msg, ok)
		}
		if _, _, ok := Emit(c, 0); ok {
			t.Error("Emit 必須是一次性的：第二次不得再回報 armed")
		}
		c.Status(http.StatusOK)
	})
}

// 🔴 A2 的核心：述詞是「實際寫出去的 code」，不是「呼叫了哪個函式」。
//
// 非零 code 的出口不消化別的碼、也不改寫；零 code 的出口一律要消化。第一版把這件事
// 綁在函式身分上（四支查 carrier、十八支 MarkEmitted），於是 Biz(c, 0, …) 這個
// **已存在且正在被使用**的 helper 就是第十種繞道。
func TestEmitPredicateIsTheCodeNotTheCaller(t *testing.T) {
	runWithCarrier(t, func(c *gin.Context) {
		Arm(c.Request.Context(), methodA, 9331, "x")

		// 非零：不改寫，且不得消化掉不同碼的 arm。
		if _, _, ok := Emit(c, 400); ok {
			t.Error("emittedCode=400 不該觸發改寫")
		}
		if got := PendingCodes(c); len(got) != 1 || got[0] != 9331 {
			t.Fatalf("非零出口不得吃掉別碼的 arm，pending=%v", got)
		}

		// 零：必須改寫。
		code, _, ok := Emit(c, 0)
		if !ok || code != 9331 {
			t.Fatalf("emittedCode=0 必須改寫成 9331，得到 (%d,%v)", code, ok)
		}
		c.Status(http.StatusOK)
	})
}

// 非零出口若送的正是那個 pending 的碼 ⇒ handler 自己把它送出去了，arm 應被消化。
func TestEmitSameCodeConsumesTheArm(t *testing.T) {
	runWithCarrier(t, func(c *gin.Context) {
		Arm(c.Request.Context(), methodA, 9331, "x")
		if _, _, ok := Emit(c, 9331); ok {
			t.Error("送出同一個碼不是改寫")
		}
		if got := PendingCodes(c); len(got) != 0 {
			t.Errorf("handler 已送出 9331，arm 必須被消化，pending=%v", got)
		}
		c.Status(http.StatusOK)
	})
}

// 🔴 fail-closed 契約：沒有 carrier 的 context 上 Arm 必須回 false。
//
// 呼叫端（EnvelopeGuardInterceptor）據此撤銷 preserveData 特權、改走一般錯誤映射。
// 這一條是本設計與舊 AST gate 最關鍵的差別：舊 gate 把「我看不到成功出口」當成安全
// 的證明（per-site fail-OPEN），9 種繞道全部從那個洞進來。
func TestArmWithoutCarrierFailsClosed(t *testing.T) {
	if Arm(context.Background(), methodA, 9331, "x") {
		t.Fatal("沒有 carrier 時 Arm 必須回 false（撤銷特權），不得靜默成功")
	}
	//nolint:staticcheck // 明確斷言 nil context 不 panic：interceptor 收到什麼都不能炸
	if Arm(nil, methodA, 9331, "x") {
		t.Fatal("nil context 上 Arm 必須回 false")
	}
}

// code==0 不是業務失敗，不得掛上 arm（否則 Emit 的述詞會自我矛盾）。
func TestArmRejectsZeroCode(t *testing.T) {
	runWithCarrier(t, func(c *gin.Context) {
		if Arm(c.Request.Context(), methodA, 0, "success") {
			t.Error("code==0 不得 arm")
		}
		if got := PendingCodes(c); len(got) != 0 {
			t.Errorf("pending=%v，want 空", got)
		}
		c.Status(http.StatusOK)
	})
}

// Emit 在沒有 Request 的 gin.Context（單元測試常見）上必須安靜回 false，不得 panic。
func TestEmitWithoutRequestIsSafe(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if _, _, ok := Emit(c, 0); ok {
		t.Error("沒有 Request 的 context 不該回報 armed")
	}
	EmitNonSuccess(c) // 不得 panic
	if Accept(c, 9331) {
		t.Error("沒有 Request 的 context 上 Accept 不該命中")
	}
	if got := PendingCodes(c); got != nil {
		t.Errorf("PendingCodes=%v，want nil", got)
	}
}

// 🔴 A1：Disarm 必須是逐 method 的。
//
// 第一版是無條件 Disarm(ctx)：一個請求裡先失敗的 RPC，會被後面任何一支成功的 RPC
// 抹掉 —— 拿「沒有觀察到危險」推論「沒有危險」，正是本 package 要消滅的那個根因
// 換了地方住。別支 RPC 的成功不是前一支失敗已被處理的證據。
func TestDisarmIsScopedToTheSameMethod(t *testing.T) {
	runWithCarrier(t, func(c *gin.Context) {
		Arm(c.Request.Context(), methodA, 9331, "憑證驗證失敗")

		Disarm(c.Request.Context(), methodB) // 別支 RPC 成功
		if code, armed := Armed(c); !armed || code != 9331 {
			t.Fatalf("別支 RPC 的成功不得抹掉 9331，得到 (%d, %v)", code, armed)
		}

		Disarm(c.Request.Context(), methodA) // 同一支 RPC 重試成功
		if _, armed := Armed(c); armed {
			t.Error("同一支 method 回報成功之後，它自己掛的 arm 必須解除")
		}
		c.Status(http.StatusOK)
	})
}

// 兩支不同 RPC 各自失敗：後者不得覆蓋前者，改寫時取最舊那筆（第一件出錯的才是因）。
func TestTwoMethodsArmIndependently(t *testing.T) {
	runWithCarrier(t, func(c *gin.Context) {
		Arm(c.Request.Context(), methodA, 9331, "A 失敗")
		Arm(c.Request.Context(), methodB, 9310, "B 失敗")
		if got := PendingCodes(c); len(got) != 2 || got[0] != 9331 || got[1] != 9310 {
			t.Fatalf("pending=%v，want [9331 9310]", got)
		}
		code, msg, ok := Emit(c, 0)
		if !ok || code != 9331 || msg != "A 失敗" {
			t.Fatalf("改寫必須取最舊那筆，得到 (%d,%q,%v)", code, msg, ok)
		}
		if got := PendingCodes(c); len(got) != 0 {
			t.Errorf("回應已寫出，其餘 arm 也必須清掉，pending=%v", got)
		}
		c.Status(http.StatusOK)
	})
}

// 同一支 method 重試：後者取代前者，不累積。
func TestSameMethodRearmReplaces(t *testing.T) {
	runWithCarrier(t, func(c *gin.Context) {
		Arm(c.Request.Context(), methodA, 9331, "第一次")
		Arm(c.Request.Context(), methodA, 9310, "第二次")
		if got := PendingCodes(c); len(got) != 1 || got[0] != 9310 {
			t.Fatalf("pending=%v，want [9310]", got)
		}
		c.Status(http.StatusOK)
	})
}

// Accept 是唯一的逃生口，且必須指名 code：碼不符時不得解除。
func TestAcceptIsCodeScoped(t *testing.T) {
	runWithCarrier(t, func(c *gin.Context) {
		Arm(c.Request.Context(), methodA, 9310, "duplicate")
		if Accept(c, 9331) {
			t.Error("Accept 指名 9331 不得解除 9310 —— 逃生口必須是逐碼的")
		}
		if _, armed := Armed(c); !armed {
			t.Error("不相符的 Accept 之後 carrier 必須仍是 armed")
		}
		if !Accept(c, 9310) {
			t.Error("Accept 指名 9310 必須命中")
		}
		if _, _, ok := Emit(c, 0); ok {
			t.Error("Accept 之後 Emit 不得再回報 armed")
		}
		c.Status(http.StatusOK)
	})
}

// 收尾偵測：有未消化的 arm 且沒有任何 response 出口留下痕跡 ⇒ 繞過了 internal/response。
//
// 這裡斷言兩件事：① 偵測層不改寫已送出的 body ② 逃逸計數器真的跳了一格
// （②才是這道偵測的鑑別力所在——沒有它，「偵測」只是一段從沒被驗證過的程式碼）。
// 之所以只偵測不補正：要補正已送出的 body 必須扣住 Write/WriteHeader 到 c.Next()
// 之後，那會破壞 Flush()、SSE 與 http.ResponseController 的 deadline 設定。
func TestEscapeDetectionCountsAndDoesNotRewriteBody(t *testing.T) {
	before := testutil.ToFloat64(bizcodeEscapedTotal.WithLabelValues("9331", "/probe"))
	w := runWithCarrier(t, func(c *gin.Context) {
		Arm(c.Request.Context(), methodA, 9331, "憑證驗證失敗")
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"}) // 繞過 internal/response
	})
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] != float64(0) {
		t.Errorf("偵測層不得改寫 body，得到 code=%v", body["code"])
	}
	if after := testutil.ToFloat64(bizcodeEscapedTotal.WithLabelValues("9331", "/probe")); after != before+1 {
		t.Errorf("逃逸計數器 %v → %v，want +1", before, after)
	}
}

// 走過 response 層（不論成功或錯誤形狀）之後，收尾偵測不該當成繞過 —— 否則每個
// 正確走 response.Biz 的請求都會誤報，而假告警會讓真告警被忽略。
func TestEmitSuppressesEscapeDetection(t *testing.T) {
	before := testutil.ToFloat64(bizcodeEscapedTotal.WithLabelValues("9331", "/probe"))
	runWithCarrier(t, func(c *gin.Context) {
		Arm(c.Request.Context(), methodA, 9331, "x")
		Emit(c, 4001) // 非零出口：留痕跡但不消化 9331
		c.Status(http.StatusOK)
	})
	if after := testutil.ToFloat64(bizcodeEscapedTotal.WithLabelValues("9331", "/probe")); after != before {
		t.Errorf("走過 response 層不該被記成逃逸，%v → %v", before, after)
	}
}

// EmitNonSuccess 同樣要抑制逃逸偵測（GrpcErr 走它），但不得消化任何 arm。
func TestEmitNonSuccessMarksButNeverConsumes(t *testing.T) {
	before := testutil.ToFloat64(bizcodeEscapedTotal.WithLabelValues("9331", "/probe"))
	runWithCarrier(t, func(c *gin.Context) {
		Arm(c.Request.Context(), methodA, 9331, "x")
		EmitNonSuccess(c)
		if got := PendingCodes(c); len(got) != 1 {
			t.Errorf("EmitNonSuccess 不得消化 arm，pending=%v", got)
		}
		c.Status(http.StatusOK)
	})
	if after := testutil.ToFloat64(bizcodeEscapedTotal.WithLabelValues("9331", "/probe")); after != before {
		t.Errorf("EmitNonSuccess 之後不該被記成逃逸，%v → %v", before, after)
	}
}

// 改寫時 swallowed 計數器要跳 —— 它是「有 handler 在依賴自動改正」的唯一訊號，
// 也是舊 AST gate 降級之後唯一能發現「守門被刪掉」的東西。
func TestSwallowedCounterIncrementsOnRewrite(t *testing.T) {
	before := testutil.ToFloat64(bizcodeSwallowedTotal.WithLabelValues("9331", "/probe"))
	runWithCarrier(t, func(c *gin.Context) {
		Arm(c.Request.Context(), methodA, 9331, "x")
		Emit(c, 0)
		c.Status(http.StatusOK)
	})
	if after := testutil.ToFloat64(bizcodeSwallowedTotal.WithLabelValues("9331", "/probe")); after != before+1 {
		t.Errorf("swallowed 計數器 %v → %v，want +1", before, after)
	}
}

// TestEscapeDetectionSurvivesPanic 守的是 Middleware 的收尾偵測必須用 defer。
//
// 🔴 這一支的存在理由是它抓到過真的：偵測原本寫在 c.Next() 之後，panic 時
// c.Next() 不會正常返回 ⇒ 整段被跳過 ⇒ 計數器 0→0、零 log。而 panic 正是
// 「回應不是走 response 層寫出去」最可能發生的時刻——訊號在最需要它的時候
// 消失（.claude/rules/06-no-small-tails.md §3.10）。
//
// 變異驗證：把 Middleware 的 defer 拆回「c.Next() 之後直接寫」，本測試轉紅。
func TestEscapeDetectionSurvivesPanic(t *testing.T) {
	before := testutil.ToFloat64(bizcodeEscapedTotal.WithLabelValues("9331", "/probe"))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery()) // 生產鏈上有它；沒有的話 panic 會炸穿 ServeHTTP
	r.Use(Middleware())
	r.GET("/probe", func(c *gin.Context) {
		if !Arm(c.Request.Context(), "/inv.Invoice/Probe", 9331, "credential invalid") {
			t.Fatal("Arm 失敗——carrier 沒掛上")
		}
		panic("handler exploded after arming")
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))

	if after := testutil.ToFloat64(bizcodeEscapedTotal.WithLabelValues("9331", "/probe")); after != before+1 {
		t.Fatalf("panic 之後逃逸計數器 %v → %v，want +1"+
			"\n—— 收尾偵測沒有用 defer，handler panic 時它整段不執行", before, after)
	}
}
