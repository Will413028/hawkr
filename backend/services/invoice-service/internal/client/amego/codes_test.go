package amego

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	gobreaker "github.com/sony/gobreaker/v2"
)

// codeCase 是 §7.5.1／§4.5.6 逐碼對照表的一列。
//
// bucket 記的是 §7.5.1 的**落點**（operation 最後停在哪個狀態），class 記的是
// §4.5.6 的四分類。兩欄一起寫是刻意的：§7.5.1 是 §4.5.6 在執行時視圖上的投影，
// 對照關係為 RETRYABLE=R｜PERMANENT=P ＋ M 的「只有 ADM 能解」那一支｜UNKNOWN=Q，
// 唯一有語意差的一格是 M 的「賣家自己能解」那一支落 SELLER_ACTION_REQUIRED。
type codeCase struct {
	code     string
	msg      string
	endpoint string
	bucket   string // §7.5.1 落點

	class        ErrorClass
	sellerAction bool
	successEq    bool
	notFound     bool
	accountLevel bool
	breaker      bool
	known        bool
}

const (
	bucketRetryable = "RETRYABLE"
	bucketPermanent = "PERMANENT"
	bucketSeller    = "SELLER_ACTION_REQUIRED"
	bucketUnknown   = "UNKNOWN"
)

// classificationTable 是**逐碼**（不是抽樣）的對照表。
//
// 新增或修改任何一列時，先讀 §4.5.6 的處置表與 §7.5.1 的三分類表；兩處不一致時，
// 精確碼一律勝出於區間——那是 §7.5.1 自身的內部規則（它把 3050131 列進 RETRYABLE，
// 同時把含它的 3050121~3050141 列進 PERMANENT）。
var classificationTable = []codeCase{
	// ── 通用碼 ──
	{code: "10", bucket: bucketRetryable, class: ClassRetryable, breaker: true, known: true},
	{code: "11", bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "12", bucket: bucketPermanent, class: ClassManual, accountLevel: true, known: true},
	{code: "13", bucket: bucketPermanent, class: ClassManual, accountLevel: true, known: true},
	{code: "14", bucket: bucketRetryable, class: ClassRetryable, breaker: true, known: true},
	{code: "15", bucket: bucketRetryable, class: ClassRetryable, known: true},
	{code: "16", bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "17", bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "18", bucket: bucketRetryable, class: ClassRetryable, breaker: true, known: true},
	{code: "19", bucket: bucketPermanent, class: ClassManual, accountLevel: true, known: true},
	{code: "20", bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "21", bucket: bucketRetryable, class: ClassRetryable, breaker: true, known: true},
	{code: "22", bucket: bucketPermanent, class: ClassManual, accountLevel: true, known: true},
	{code: "23", bucket: bucketPermanent, class: ClassPermanent, known: true},

	// ── 查詢／驗證類共用碼 ──
	{code: "34", endpoint: EndpointInvoiceList, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "51", endpoint: EndpointInvoiceQuery, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "52", endpoint: EndpointInvoiceQuery, bucket: bucketRetryable, class: ClassRetryable, known: true},
	{code: "71", endpoint: EndpointInvoiceQuery, bucket: bucketPermanent, class: ClassPermanent, notFound: true, known: true},
	{code: "99", endpoint: EndpointTrackAll, bucket: bucketPermanent, class: ClassManual, known: true},

	// ── f0401 ──
	{code: "3040111", endpoint: EndpointIssue, bucket: bucketSeller, class: ClassManual, sellerAction: true, known: true},
	{code: "3040191", endpoint: EndpointIssue, bucket: bucketSeller, class: ClassManual, sellerAction: true, known: true},
	{code: "3040112", endpoint: EndpointIssue, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "3040113", endpoint: EndpointIssue, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "3040121", endpoint: EndpointIssue, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "3040131", endpoint: EndpointIssue, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "3040163", endpoint: EndpointIssue, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "3040172", endpoint: EndpointIssue, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "3040174", endpoint: EndpointIssue, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "3040184", endpoint: EndpointIssue, bucket: bucketPermanent, class: ClassPermanent, known: true},
	// 一碼兩義：兩個分支各一列（第三種杜撰措辭另有獨立測試）。
	{code: "3040171", msg: "OrderId 重複，該訂單編號正在註銷重開中", endpoint: EndpointIssue,
		bucket: bucketRetryable, class: ClassRetryable, known: true},
	{code: "3040171", msg: "OrderId 重複", endpoint: EndpointIssue,
		bucket: bucketUnknown, class: ClassReconcile, successEq: true, known: true},

	// ── f0501 ──
	{code: "3050113", endpoint: EndpointVoid, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "3050121", endpoint: EndpointVoid, bucket: bucketRetryable, class: ClassRetryable, known: true},
	{code: "3050122", endpoint: EndpointVoid, bucket: bucketUnknown, class: ClassReconcile, successEq: true, known: true},
	{code: "3050123", endpoint: EndpointVoid, bucket: bucketPermanent, class: ClassManual, known: true},
	{code: "3050125", endpoint: EndpointVoid, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "3050126", endpoint: EndpointVoid, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "3050131", endpoint: EndpointVoid, bucket: bucketRetryable, class: ClassRetryable, known: true},
	{code: "3050141", endpoint: EndpointVoid, bucket: bucketPermanent, class: ClassPermanent, known: true},

	// ── g0401 ──
	{code: "4040121", endpoint: EndpointAllowance, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "4040123", endpoint: EndpointAllowance, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "4040139", endpoint: EndpointAllowance, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "4040145", endpoint: EndpointAllowance, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "4040151", endpoint: EndpointAllowance, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "4040161", endpoint: EndpointAllowance, bucket: bucketUnknown, class: ClassReconcile, successEq: true, known: true},
	{code: "4040163", endpoint: EndpointAllowance, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "4040171", endpoint: EndpointAllowance, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "4040173", endpoint: EndpointAllowance, bucket: bucketPermanent, class: ClassPermanent, known: true},

	// ── g0501 ──
	// 4050131「折讓開立中」刻意偏離 §7.5.1 的字面（該節把它留在 PERMANENT 區間）：
	// 第三方文件明文寫「4050131 ＋ 4050141 ⇒ 剛開立的折讓單不能立刻作廢，必須等
	// allowance_status 的 status 走到 99」——兩碼是同一個暫時狀態的兩半，而 §7.5.1
	// 只把 4050141 列進 RETRYABLE。理由與代價見 codes.go 該列註解。
	{code: "4050131", endpoint: EndpointVoidAllowance, bucket: bucketRetryable, class: ClassRetryable, known: true},
	{code: "4050132", endpoint: EndpointVoidAllowance, bucket: bucketUnknown, class: ClassReconcile, successEq: true, known: true},
	{code: "4050135", endpoint: EndpointVoidAllowance, bucket: bucketPermanent, class: ClassManual, known: true},
	{code: "4050141", endpoint: EndpointVoidAllowance, bucket: bucketRetryable, class: ClassRetryable, known: true},

	// ── barcode ──
	{code: "9000111", endpoint: EndpointBarcode, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "9000112", endpoint: EndpointBarcode, bucket: bucketPermanent, class: ClassPermanent, known: true},
	{code: "9000113", endpoint: EndpointBarcode, bucket: bucketPermanent, class: ClassPermanent, known: true},

	// ── 兩張分類表都沒有登記的碼（走 default）──
	// §2.3.3／§2.10 有定義但 §4.5.6 與 §7.5.1 都沒有收錄，因此走 fail-safe 的
	// Reconcile：對唯讀查詢而言代價是多打一次 query，遠小於誤判成永久失敗。
	{code: "3050111", endpoint: EndpointVoid, bucket: bucketUnknown, class: ClassReconcile},
	{code: "3050112", endpoint: EndpointVoid, bucket: bucketUnknown, class: ClassReconcile},
	{code: "4050112", endpoint: EndpointVoidAllowance, bucket: bucketUnknown, class: ClassReconcile},
	{code: "4050121", endpoint: EndpointVoidAllowance, bucket: bucketUnknown, class: ClassReconcile},
}

// TestClassificationTable 逐碼（不是抽樣）比對每一個登記碼的四分類與旗標。
func TestClassificationTable(t *testing.T) {
	t.Parallel()
	for _, tc := range classificationTable {
		name := tc.code
		if tc.msg != "" {
			name += "|" + tc.msg
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := classify(tc.endpoint, tc.code, tc.msg)
			if got.class != tc.class {
				t.Errorf("class = %s, want %s（§7.5.1 落點 %s）", got.class, tc.class, tc.bucket)
			}
			if got.sellerAction != tc.sellerAction {
				t.Errorf("sellerAction = %v, want %v", got.sellerAction, tc.sellerAction)
			}
			if got.successEquivalent != tc.successEq {
				t.Errorf("successEquivalent = %v, want %v", got.successEquivalent, tc.successEq)
			}
			if got.notFound != tc.notFound {
				t.Errorf("notFound = %v, want %v", got.notFound, tc.notFound)
			}
			if got.accountLevel != tc.accountLevel {
				t.Errorf("accountLevel = %v, want %v", got.accountLevel, tc.accountLevel)
			}
			if got.known != tc.known {
				t.Errorf("known = %v, want %v", got.known, tc.known)
			}
			if got.breaker != tc.breaker {
				t.Errorf("breaker = %v, want %v", got.breaker, tc.breaker)
			}
		})
	}
}

// TestSellerActionOnlyForTrackExhaustion 釘住 §7.5.1 與 §4.5 唯一有語意差的那一格。
//
// 落錯桶到 FAILED_PERMANENT 的票再也回不來：INV.trk 的 J33 promote 只掃
// SELLER_ACTION_REQUIRED。
func TestSellerActionOnlyForTrackExhaustion(t *testing.T) {
	t.Parallel()
	want := map[string]bool{"3040111": true, "3040191": true}
	for _, tc := range classificationTable {
		got := classify(tc.endpoint, tc.code, tc.msg).sellerAction
		if got != want[tc.code] {
			t.Errorf("code %s sellerAction = %v, want %v（只有字軌耗盡是賣家自己解得掉的）", tc.code, got, want[tc.code])
		}
	}
}

// TestCode14IsRetryableAndCountsTowardBreaker 是 §4.5.6 與 §7.5.1 都特別立文的一條。
//
// 歸 PERMANENT 會把故障期間的每一筆 operation 都變成需人工逐筆重排的死件，
// 而它是全平台一起壞、白名單登記完就全部會成功的故障。
func TestCode14IsRetryableAndCountsTowardBreaker(t *testing.T) {
	t.Parallel()
	c := classify(EndpointIssue, "14", "IP 錯誤")
	if c.class != ClassRetryable {
		t.Fatalf("code 14 class = %s, want %s", c.class, ClassRetryable)
	}
	if !c.breaker {
		t.Fatal("code 14 必須計入斷路器：它是我方出口 IP 的全域問題，不是單一賣家的問題")
	}
	err := newProviderError(EndpointIssue, "14", "IP 錯誤", 200)
	if !countsTowardBreaker(err) {
		t.Fatal("countsTowardBreaker(code 14) 必須為 true")
	}
}

// TestReissuingMarkerBranches 是 3040171 一碼兩義的四支測試（§4.5.6a）。
func TestReissuingMarkerBranches(t *testing.T) {
	t.Parallel()
	t.Run("永久衝突 → Reconcile", func(t *testing.T) {
		t.Parallel()
		c := classify(EndpointIssue, "3040171", "OrderId 重複")
		if c.class != ClassReconcile {
			t.Fatalf("class = %s, want %s", c.class, ClassReconcile)
		}
		if !c.successEquivalent {
			t.Fatal("永久衝突代表『已經開過了』⇒ 對帳取回號碼即視為成功")
		}
	})
	t.Run("註銷重開中 → Retryable", func(t *testing.T) {
		t.Parallel()
		c := classify(EndpointIssue, "3040171", "OrderId 重複，該訂單編號正在註銷重開中")
		if c.class != ClassRetryable {
			t.Fatalf("class = %s, want %s", c.class, ClassRetryable)
		}
	})
	t.Run("杜撰的第三種措辭 → 保守走 Reconcile", func(t *testing.T) {
		t.Parallel()
		c := classify(EndpointIssue, "3040171", "OrderId 重複，系統忙碌請稍後")
		if c.class != ClassReconcile {
			t.Fatalf("class = %s, want %s（msg 比對只是省一次往返的最佳化，不是正確性的來源）", c.class, ClassReconcile)
		}
	})
	t.Run("marker 常數就是判別依據", func(t *testing.T) {
		t.Parallel()
		// 這一支釘住常數本身：把 amegoReissuingMarker 改成別的字串，
		// 「註銷重開中 → Retryable」那支必然轉紅（變異測試 ④）。
		if !strings.Contains("OrderId 重複，該訂單編號正在註銷重開中", amegoReissuingMarker) {
			t.Fatalf("amegoReissuingMarker = %q 不出現在官方措辭裡", amegoReissuingMarker)
		}
	})
}

// TestUnknownCodeFallsBackToReconcile 是 default 分支的鑑別力測試。
//
// default 選 Permanent ⇒ 丟掉一張本來開得出來的發票；選 Retryable ⇒ 白燒法定時限；
// 這一支同時擋住「把 default 改成 panic 或 Permanent」與「把原始碼寫回 metric label」。
func TestUnknownCodeFallsBackToReconcile(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"3049999", "4049999", "8888888", "1234567", "not-a-number"} {
		c := classify(EndpointIssue, code, "杜撰")
		if c.class != ClassReconcile {
			t.Errorf("未知碼 %s class = %s, want %s", code, c.class, ClassReconcile)
		}
		if c.known {
			t.Errorf("未知碼 %s 不得標成 known", code)
		}
		e := newProviderError(EndpointIssue, code, "杜撰", 200)
		if got := e.MetricCodeLabel(); got != CodeOther {
			t.Errorf("未知碼 %s 的 metric label = %q, want %q（原始碼是無界基數）", code, got, CodeOther)
		}
		if e.Code != code {
			t.Errorf("provider_code 必須原樣落庫為字串: %q", e.Code)
		}
	}
}

// TestKnownCodeKeepsItsOwnMetricLabel：已登記的碼有界，可以當 label。
func TestKnownCodeKeepsItsOwnMetricLabel(t *testing.T) {
	t.Parallel()
	e := newProviderError(EndpointIssue, "3040111", "字軌不足", 200)
	if got := e.MetricCodeLabel(); got != "3040111" {
		t.Fatalf("MetricCodeLabel = %q, want 3040111", got)
	}
}

// TestBreakerExclusions 釘住「哪些失敗算 AMEGO 自己不健康」。
//
// 用排除而非「回報成功」：後者會把 ConsecutiveFailures 歸零，於是真實故障期間夾雜
// 幾筆格式錯誤就能讓斷路器永遠跳不起來。
func TestBreakerExclusions(t *testing.T) {
	t.Parallel()
	counted := map[string]bool{"10": true, "14": true, "18": true, "21": true}
	for _, tc := range classificationTable {
		err := newProviderError(tc.endpoint, tc.code, tc.msg, 200)
		if got := countsTowardBreaker(err); got != counted[tc.code] {
			t.Errorf("code %s countsTowardBreaker = %v, want %v", tc.code, got, counted[tc.code])
		}
	}
	// 帳號層的碼絕不可計入：一個賣家送錯統編不該讓其他賣家開不出發票。
	for _, code := range []string{"12", "13", "19", "22", "16", "3040171"} {
		if countsTowardBreaker(newProviderError(EndpointIssue, code, "", 200)) {
			t.Errorf("code %s 不得計入斷路器", code)
		}
	}
	// 傳輸層與解碼失敗計入。
	for _, kind := range []string{ErrorKindTimeout, ErrorKindConn, ErrorKindDecode} {
		if !countsTowardBreaker(&APIError{Kind: kind}) {
			t.Errorf("kind %s 必須計入斷路器", kind)
		}
	}
	// ENV_GUARD 根本沒有外呼，與 AMEGO 的健康無關。
	if countsTowardBreaker(&APIError{Code: CodeEnvGuard, Class: ClassManual}) {
		t.Error("ENV_GUARD 不得計入斷路器")
	}
	if !isExcludedFromBreaker(&APIError{Code: CodeEnvGuard}) {
		t.Error("isExcludedFromBreaker 必須是 countsTowardBreaker 的反面")
	}
}

// TestOpenStateMapsToRetryable：開路時 operation 不得標成失敗（W-18）。
func TestOpenStateMapsToRetryable(t *testing.T) {
	t.Parallel()
	for _, err := range []error{gobreaker.ErrOpenState, gobreaker.ErrTooManyRequests} {
		got := asOpenStateError(EndpointIssue, err)
		if got == nil {
			t.Fatalf("asOpenStateError(%v) 回 nil", err)
		}
		if got.Class != ClassRetryable {
			t.Fatalf("class = %s, want %s", got.Class, ClassRetryable)
		}
	}
	if asOpenStateError(EndpointIssue, errors.New("其他錯")) != nil {
		t.Fatal("非 breaker 錯誤不得被當成開路")
	}
}

// TestAPIErrorNeverCarriesAppKey：error message 是 AppKey 最容易外洩的地方之一
// （%w 鏈會一路往上傳）。
func TestAPIErrorNeverCarriesAppKey(t *testing.T) {
	t.Parallel()
	e := newProviderError(EndpointIssue, "16", "sign(簽名)驗證錯誤", 200)
	if strings.Contains(e.Error(), officialAppKey) {
		t.Fatal("APIError.Error() 不得包含 AppKey")
	}
	cred := Credential{IssuerID: 1, Invoice: "12345678", AppKey: officialAppKey}
	if strings.Contains(fmt.Sprintf("%v", cred), officialAppKey) {
		t.Fatal("Credential 的字串化不得印出 AppKey")
	}
}

// TestErrorClassIsExhaustive 擋住「悄悄多一個分類」——四分類是與 worker 的契約，
// 多一個值會讓 worker 的 switch 落進 default 而靜默走錯路。
func TestErrorClassIsExhaustive(t *testing.T) {
	t.Parallel()
	seen := map[ErrorClass]bool{}
	for _, tc := range classificationTable {
		seen[classify(tc.endpoint, tc.code, tc.msg).class] = true
	}
	want := []ErrorClass{ClassRetryable, ClassPermanent, ClassReconcile, ClassManual}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("分類表沒有任何一列產出 %s，覆蓋率不足", w)
		}
	}
	if len(seen) != len(want) {
		got := make([]string, 0, len(seen))
		for k := range seen {
			got = append(got, string(k))
		}
		sort.Strings(got)
		t.Errorf("出現了未預期的分類: %v", got)
	}
}

// TestEndpointsCoverInterface：介面涵蓋的端點集合 ＝ §4.5.9 的 nginx 白名單。
func TestEndpointsCoverInterface(t *testing.T) {
	t.Parallel()
	eps := Endpoints()
	if len(eps) != 15 {
		t.Fatalf("端點數 = %d, want 15（16 個方法打 15 支端點，兩支 query 共用 invoice_query）", len(eps))
	}
	seen := map[string]bool{}
	for _, e := range eps {
		if seen[e] {
			t.Errorf("端點重複: %s", e)
		}
		if !strings.HasPrefix(e, "/json/") {
			t.Errorf("端點必須是路徑不是完整 URL: %s", e)
		}
		seen[e] = true
	}
}
