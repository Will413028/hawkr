// Package bizcarrier 讓「上游回了一個非零業務碼」這件事跨越函式邊界活到回應寫出的
// 那一刻，於是「handler 有沒有記得檢查 resp.GetCode()」不再是正確性的前提。
//
// # 這個 package 存在的理由
//
// EnvelopeGuardInterceptor 對 envelopeCodePreserveData 名單內的業務碼**不轉 error**
// （命中就 `return nil`），因為那些碼的 data 本身是契約的一部分（9331 的
// providerRequestId、9347 的 documentId、4001 的 unavailableItems…），而
// status.Error 只帶得動一個字串、data 會在邊界上蒸發。代價是 handler 的
// `if err != nil` 對這些碼完全失明：resp.Code 非零而 err 為 nil，handler 只要接著
// 走 response.OK，envelope 的 code 就寫成 0 —— **業務失敗被包裝成成功回應**，
// HTTP 200、log 乾淨、沒有任何測試會紅。
//
// 2026-08-31 的實例：端點 44 HealthCheckProviderAccountForAdmin 就是這個形狀。AMEGO
// 回報憑證驗證失敗（9331），ADM 收到 code=0 顯示「檢測成功」，該賣家之後每一筆開票
// 都會失敗。同檔的端點 66 寫對了 —— 兩支下游是同一支 usecase，差別純粹是有沒有人記得。
//
// 在此之前守這條不變式的是一道 AST gate（掃 handler 原始碼找「呼叫點之後有沒有
// GetCode 檢查」）。它被 9 種寫法繞過，其中最乾淨的一種完全不需惡意：把 import 取
// 別名成 `rsp "…/response"`，純風格 refactor，編譯過、gofmt 過、既有測試全綠，而
// gate 硬寫識別字 `"response"` 於是整個檔找不到任何成功出口、全部靜默放行。
//
// 兩個根因都是「原始碼形狀」這個判定材料本身的極限：
//
//   - **per-site fail-OPEN**：「這個呼叫點之後找不到成功出口」被當成安全的證明，
//     但它同樣可以是「我看不到而已」（emit 被抽進 helper、call 被抽進 helper、
//     import 取別名…）。
//   - **有 token ≠ 真的擋住**：AST 只問得出「子樹裡存不存在一個 response.Biz 節點」，
//     問不出「非零碼到不了成功出口」——後者是**路徑性質**，靜態判不了。
//
// 本 package 把判定材料換成執行期事實：業務碼由 interceptor 掛上（Arm），
// 只有真的走到成功出口時才會被取下（Emit）。取下時它還在，就證明有人要用
// code=0 送出一個失敗 —— 這不需要看懂任何原始碼形狀。
//
// # 不變式
//
//	凡是經由 internal/response 寫出的 envelope，其 Code 欄位為 0 時，該請求必無
//	未消化的 arm；否則該回應會被自動改寫成 response.Biz(code, message, data)。
//
// 🔴 述詞是「**實際寫出去的 envelope code 是不是 0**」，不是「呼叫了哪一個函式」。
// 2026-08-31 的第一版把述詞訂成後者（四個「成功出口」查 carrier、其餘 18 個出口
// 呼叫 MarkEmitted），於是 `response.Biz(c, 0, "success", data)` —— 一個**已經存在
// 且正在被使用**的 exported helper（handler/checkout.go 的三個呼叫點）—— 就是第十種
// 繞道：輸出 `{"code":0,"message":"success","data":…}` HTTP 200，而 MarkEmitted 把
// `emitted` 設成 true，正是收尾偵測讀的那個旗標 ⇒ carrier 原封不動 armed 著沒人看、
// 兩個計數器都不動、ERROR log 一行都不出。**那比沉默更糟：它還蓋了一個「已檢查」的章。**
//
// 現在 internal/response 的每一個出口都呼叫 Emit 並交出它即將寫進 envelope 的 Code，
// 由 Emit 判斷要不要改寫。「宣告的碼 == 真的寫出去的碼」由
// internal/response 的 emitter 矩陣測試逐支實測（idle / armed 兩種狀態各跑一次），
// 不是靠註解宣稱。
//
// # 生命週期
//
//	Middleware()                    每個請求建立一個 holder，掛進 c.Request 的 context
//	Arm(ctx, method, code, msg)     interceptor 收到 preserveData 業務碼時掛上（逐 method）
//	Disarm(ctx, method)             interceptor 收到同一支 method 的 code==0 時解除
//	Emit(c, emittedCode)            internal/response 的**每一個**出口都要呼叫
//	EmitNonSuccess(c)               碼由下游決定但保證恆非零的出口（GrpcErr 家族）
//	Accept(c, code)                 handler 顯式聲明「這個碼我看過了，就是要當成功回」
//
// # 作用域（寫下來並被機制強制，不是假設）
//
// carrier 的容器是**單一 HTTP 請求**；arm 的鍵是**發出它的 gRPC full method**。
//
//   - 一個請求裡任何一支 RPC 掛上的 arm，都會擋住這個請求裡任何一個 code==0 的
//     envelope，**不分業務領域**。領域無關是刻意的：只要有一件業務失敗沒被消化，
//     寫 code:0 就是說謊，跟它來自哪個服務無關。
//   - 解除只由**同一支 method** 的成功回應觸發（重試成功會蓋掉前一次失敗，這是
//     唯一一種「後來的成功真的取代了先前的失敗」）。別支 RPC 的成功**不再**解除它。
//
// 第一版的 Disarm 是無條件的，於是「先失敗的 RPC 被後面成功的 RPC 抹掉」——那正是
// 這個 package 要消滅的那個根因（拿「沒有觀察到危險」推論「沒有危險」）搬了家。
// ⚠️ 同一份設計裡 Arm 做對了（回 false 就撤銷 preserveData 特權、落回 status.Error），
// Disarm 沒有；兩個相鄰決策方向相反，本身就是訊號。
//
// 2026-08-31 實測（gateway/internal 全域 AST 掃描，腳本形狀見本節末）：會 arm 的
// RPC（invoice 全部 ＋ order/cart 的 Checkout*）出現在 **74** 個函式裡，其中
// **0 個**在 arm 之後還會再打第二支 RPC 或呼叫任何 h.<helper>()。⇒ 把 Disarm 收斂
// 成逐 method 對今天的流量是零行為改變；它買到的是「哪天有人在 arm 之後加第二支
// RPC」時不會靜默吞碼。掃描形狀：以 brace depth 切出 top-level 函式，比對
// `h.Invoice.X(` / `h.Order.Checkout*(` 的行號與其後的 `X.Y(ctx…)` / `h.foo(` 行號。
//
// 反方向也查過（handler 之前跑的東西會不會 arm）：internal/middleware、router、
// buyerid、sellerid、authz、proxy 全數 **零命中** `h.Invoice.` 與 `Checkout*(` ——
// 沒有任何 middleware／resolver 打得出 preserveData 碼，所以「middleware 先 arm、
// handler 的成功 RPC 後 Disarm」這條路今天不存在。這一句是量出來的，不是推定的：
// 少了它，收斂 Disarm 就可能把某條 middleware 路徑的成功回應改寫掉。
//
// # 已知的失效模式（寫在這裡而不是散在別處，因為它們是這個設計的一部分）
//
//  1. **response.OK 變成隱式**：讀 handler 的人看不出它可能吐 9331。這一條無法
//     消除，只能靠 ERROR log ＋ gateway_bizcode_swallowed_total 讓它不是無聲的。
//     這是把「71 支各自要記得」換成「一個保證」的必付代價。
//
//  2. **同一請求打多支 RPC 且刻意忽略前一支的失敗**：現在**不會**被靜默抹掉了 ——
//     未消化的 arm 會把成功回應改寫成業務碼回應並記 ERROR。要刻意忽略請用 Accept
//     顯式聲明（它指名 code，換一個碼就不再成立）。
//
//  3. **Accept 退化成 allowlist**：它的判定材料是呼叫者自己生成的 ⇒ 依 04-maintenance
//     的判準只能提醒不能強制。每個呼叫點必須附一行理由；今天零呼叫點。
//
//  4. **完全繞過 internal/response**（自己 c.JSON／直呼 pkg/envelope）：carrier 讀不到
//     那條路徑，只能在 Middleware 的收尾偵測到「有未消化的 arm 且沒有任何 response
//     出口留下痕跡」並告警 —— **這是偵測不是阻擋，而且沒有任何東西能把它變成阻擋**。
//
//     🔴 **兩條 lint（banDirectJSON／banEnvelopeSuccessEmitters）只縮小這條路，關不掉它。**
//     典型反例：handler 自己組 response.Envelope{Code: 0, ...} 再用 c.Data 寫出，
//     兩條 lint 都不命中（一條認 c.JSON、一條認 envelope.OK/Page/...），實跑 0 issues。
//     同形狀的出口還有 c.Render／c.String／直接寫 Writer。
//
//     ⇒ 這一格的殘餘風險由 gateway_bizcode_escaped_total 承擔，**永遠是偵測不是阻擋**。
//     要判斷某條新路徑有沒有落在這一格，問「它有沒有經過 internal/response 的任一出口」，
//     不要問「lint 會不會抓到」。
package bizcarrier

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

type ctxKey struct{}

// pending 是一筆尚未消化的業務失敗。method 是掛上它的 gRPC full method，
// 也是解除它的唯一鑰匙（見 package doc「作用域」）。
type pending struct {
	method string
	code   int32
	msg    string
}

// holder 綁定單一 HTTP 請求。gin 的 handler chain 是同一個 goroutine，但
// c.Request.Context() 會被下游（gRPC client 的 WithTimeout）包一層再傳給
// interceptor，而 interceptor 有可能在別的 goroutine 收尾，故仍上鎖。
//
// arms 保持插入序：改寫時取最舊的那一筆（第一件出錯的事才是因）。
type holder struct {
	mu      sync.Mutex
	arms    []pending
	emitted bool
}

// dropCodeLocked 移除所有 code 相符的 arm，回傳是否有命中。呼叫端須持有 mu。
func (h *holder) dropCodeLocked(code int32) bool {
	kept := h.arms[:0]
	hit := false
	for _, p := range h.arms {
		if p.code == code {
			hit = true
			continue
		}
		kept = append(kept, p)
	}
	h.arms = kept
	return hit
}

var (
	// bizcodeSwallowedTotal 數的是「被攔下來的吞碼」：某支 handler 拿著非零業務碼
	// 走進一個 code==0 的出口，被 carrier 改寫成正確的 envelope。這個計數器
	// **不該長期非零** —— 非零代表有一支 handler 正在依賴自動改正，應該回去把它寫對。
	bizcodeSwallowedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_bizcode_swallowed_total",
			Help: "Non-zero business codes that reached a zero-code emitter and were rewritten by bizcarrier",
		},
		[]string{"code", "path"},
	)

	// bizcodeEscapedTotal 數的是「攔不到的」：carrier 還有未消化的 arm，而回應不是任何
	// 一個 internal/response 出口寫的 ⇒ 這條路徑繞過了整個 response 層，carrier 無從改寫。
	bizcodeEscapedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_bizcode_escaped_total",
			Help: "Non-zero business codes whose response bypassed internal/response entirely (detection only)",
		},
		[]string{"code", "path"},
	)

	// bizcodeAcceptedTotal 數的是顯式豁免。它與 swallowed 分開，因為兩者的意義相反：
	// 一個是「有人忘了」，一個是「有人想清楚了」。混在一起數就分不出來。
	bizcodeAcceptedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gateway_bizcode_accepted_total",
			Help: "Non-zero business codes explicitly accepted by a handler via bizcarrier.Accept",
		},
		[]string{"code", "path"},
	)
)

// Middleware 為每個請求建立 carrier，並在請求收尾時偵測「繞過 internal/response」。
//
// 掛載位置：全域鏈（infrastructure/di.go 的 newEngine），不是某幾個路由群組。
// 掛在群組上會讓 Arm 的 fail-closed 分支對群組外的路由誤觸發 —— 那些路徑今天的行為
// 是 `return nil`（假成功），改成硬錯誤是另一種破壞，不是修復。全域掛載之後
// fail-closed 只在真正異常（handler 傳了 detached context）時才會走到。
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := &holder{}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxKey{}, h))

		// 🔴 defer 而不是寫在 c.Next() 之後：panic 時 c.Next() 不會正常返回，收尾偵測
		// 整段被跳過 ⇒ 計數器 0→0、零 log。而 panic 正是「回應不是走 response 層寫
		// 出去」最可能發生的時刻 —— 訊號在最需要它的時候消失（06 §3.10）。
		defer func() {
			h.mu.Lock()
			arms := append([]pending(nil), h.arms...)
			emitted := h.emitted
			h.mu.Unlock()

			// 有未消化的 arm 且沒有任何 response 出口留下痕跡 ⇒ 回應是別人寫的
			// （c.JSON、直呼 pkg/envelope、c.Data…），carrier 沒有機會改寫它。
			//
			// 只告警不改寫是刻意的：要「補正已經送出的 body」必須扣住 Write/WriteHeader
			// 到 c.Next() 之後，那會破壞 Flush()、SSE 與 http.ResponseController 的
			// deadline 設定（見 middleware/logger.go 的 responseBodyWriter，它是
			// observe-and-pass-through，不是 buffer-and-rewrite）。在 hot path 上為了
			// 一條 lint 只能縮小、關不掉的路徑（見檔頭失效模式 4）引入一整類新 bug，不划算。
			if len(arms) > 0 && !emitted {
				p := arms[0]
				label := strconv.FormatInt(int64(p.code), 10)
				bizcodeEscapedTotal.WithLabelValues(label, c.FullPath()).Inc()
				zap.L().Error("bizcarrier: business code escaped without passing through internal/response",
					zap.Int32("biz_code", p.code),
					zap.String("biz_method", p.method),
					zap.String("pending_codes", joinCodes(arms)),
					zap.String("path", c.FullPath()),
					zap.String("method", c.Request.Method),
					zap.Int("http_status", c.Writer.Status()),
				)
			}
		}()

		c.Next()
	}
}

func joinCodes(arms []pending) string {
	parts := make([]string, 0, len(arms))
	for _, p := range arms {
		parts = append(parts, strconv.FormatInt(int64(p.code), 10)+"@"+p.method)
	}
	return strings.Join(parts, ",")
}

func fromCtx(ctx context.Context) *holder {
	if ctx == nil {
		return nil
	}
	h, _ := ctx.Value(ctxKey{}).(*holder)
	return h
}

func fromGin(c *gin.Context) *holder {
	if c == nil || c.Request == nil {
		return nil
	}
	return fromCtx(c.Request.Context())
}

// Arm 記下「method 這支 RPC 回了一個必須活著送到前端的非零業務碼」。
//
// 回傳 false 代表這條 gRPC 呼叫的 context 上沒有 carrier（handler 傳了 detached
// context，或這支呼叫根本不在 HTTP 請求路徑上），或 code 是 0（不是業務失敗）。
// 呼叫端**必須**把 false 當成「我保護不了這一次」並改走一般錯誤映射 —— 那會丟掉
// data（降級），但不會產生假成功。
//
// 這是本設計與舊 AST gate 最關鍵的差別：舊 gate 把「我看不到」當成安全的證明
// （len(after)==0 就 continue）。這裡把「我保護不到」當成撤銷特權的理由。
//
// 同一支 method 重複 Arm 時後者取代前者（重試的最新結果才算數）；不同 method 各自
// 佔一格，互不覆蓋 —— 否則「A 失敗、B 也失敗」會讓 A 憑空消失。
func Arm(ctx context.Context, method string, code int32, msg string) bool {
	if code == 0 {
		return false
	}
	h := fromCtx(ctx)
	if h == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.arms {
		if h.arms[i].method == method {
			h.arms[i].code, h.arms[i].msg = code, msg
			return true
		}
	}
	h.arms = append(h.arms, pending{method: method, code: code, msg: msg})
	return true
}

// Disarm 在**同一支 method** 回報成功時解除它自己掛上的 arm。
//
// 🔴 逐 method 是本函式的全部重點。無條件解除（第一版）等於「後面任何一支 RPC 成功
// 就當作前面那支沒失敗過」—— 拿「沒有觀察到危險」推論「沒有危險」。別支 RPC 的成功
// 不是前一支失敗被處理掉的證據；真的要忽略請用 Accept 指名那個碼。
func Disarm(ctx context.Context, method string) {
	h := fromCtx(ctx)
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	kept := h.arms[:0]
	for _, p := range h.arms {
		if p.method == method {
			continue
		}
		kept = append(kept, p)
	}
	h.arms = kept
}

// Emit 由 internal/response 的**每一個**出口呼叫，交出它即將寫進 envelope 的 Code。
//
// 述詞是「實際寫出去的 code 是不是 0」，不是「呼叫了哪個函式」—— 後者正是第十種
// 繞道（`response.Biz(c, 0, "success", data)`）鑽的洞。
//
//   - emittedCode != 0：這不是假成功。標記「回應由 response 層寫出」（讓收尾偵測不
//     誤報），並消化掉同碼的 arm（handler 自己把那個碼送出去了，就是處理過了）。
//     回傳 ok=false。
//   - emittedCode == 0 且有未消化的 arm：回傳最舊那筆的 (code, msg, true)，呼叫端
//     **必須**改寫成 Biz(code, msg, data)。就地記 ERROR log ＋ metric，不交給呼叫端
//     ——交出去就會有人忘記，而忘記正是本設計要消滅的東西。
//   - emittedCode == 0 且沒有 arm：今天所有既有流量，回傳 ok=false，零行為改變。
func Emit(c *gin.Context, emittedCode int32) (int32, string, bool) {
	h := fromGin(c)
	if h == nil {
		return 0, "", false
	}

	h.mu.Lock()
	h.emitted = true
	if emittedCode != 0 {
		h.dropCodeLocked(emittedCode)
		h.mu.Unlock()
		return 0, "", false
	}
	if len(h.arms) == 0 {
		h.mu.Unlock()
		return 0, "", false
	}
	arms := append([]pending(nil), h.arms...)
	h.arms = nil
	h.mu.Unlock()

	p := arms[0]
	label := strconv.FormatInt(int64(p.code), 10)
	bizcodeSwallowedTotal.WithLabelValues(label, c.FullPath()).Inc()
	zap.L().Error("bizcarrier: handler emitted a zero-code envelope while a non-zero business code was pending; rewriting to response.Biz",
		zap.Int32("biz_code", p.code),
		zap.String("biz_message", p.msg),
		zap.String("biz_method", p.method),
		zap.String("pending_codes", joinCodes(arms)),
		zap.String("path", c.FullPath()),
		zap.String("method", c.Request.Method),
	)
	return p.code, p.msg, true
}

// EmitNonSuccess 供「envelope code 由下游決定、但**保證恆非零**」的出口使用。
// 今天只有 GrpcErr 家族：它的 code 是 pkg/envelope.GrpcErr 內部那個 switch 依 gRPC
// status 決定的（401/403/404/409/429/499/500/503 或 "[NNNN] " 還原出的業務碼），
// 呼叫點拿不到，重算一次等於把那個 switch 抄第二份。
//
// ⚠️ 這是本 package 唯一「不交出 code」的出口，也因此是最容易退化成第二個
// MarkEmitted 的地方。守它的兩件東西都在 internal/response 的測試裡，而且都是
// 行為面的、不是宣稱：
//
//  1. TestGrpcErrNeverWritesZeroCode 對每一個 gRPC status code（含 nil error、
//     非 status error、client-cancel）實跑 response.GrpcErr 並斷言 body 的 code != 0。
//  2. TestEmitterMatrix_NonZeroEmittersNeverGoUndetected 對每一支非零出口在 armed
//     狀態下實跑，斷言 body 不變**且**逃逸計數器沒有跳動。
//
// 新增出口時不要圖方便用它：只要 code 是呼叫點算得出來的，就用 Emit 交出去。
func EmitNonSuccess(c *gin.Context) {
	h := fromGin(c)
	if h == nil {
		return
	}
	h.mu.Lock()
	h.emitted = true
	h.mu.Unlock()
}

// Accept 是唯一的逃生口：handler 已經看過這個業務碼、確定要當成成功回應。
//
// 必須指名 code（不是「解除全部」）：寫下 9310 的人得先知道自己在放行 9310，
// 換成別的碼時這行就不再成立、carrier 會照常改正。
//
// ⚠️ 每個呼叫點必須在旁邊寫一行「為什麼這個碼可以當成功」。這條只能是提醒不能是
// 強制 —— 判定材料（理由是否成立）是呼叫者自己生成的。今天零呼叫點；數量只增不減
// 的時候要人工 review，那正是它退化成 allowlist 的徵狀。
func Accept(c *gin.Context, code int32) bool {
	h := fromGin(c)
	if h == nil {
		return false
	}
	h.mu.Lock()
	hit := h.dropCodeLocked(code)
	h.mu.Unlock()

	if !hit {
		return false
	}
	label := strconv.FormatInt(int64(code), 10)
	bizcodeAcceptedTotal.WithLabelValues(label, c.FullPath()).Inc()
	zap.L().Info("bizcarrier: business code explicitly accepted by handler",
		zap.Int32("biz_code", code),
		zap.String("path", c.FullPath()),
	)
	return true
}

// EscapedCounterFor 回傳逃逸計數器的某一條 series。
//
// 開放它是因為 internal/response 的出口矩陣測試必須對帳「非零出口有沒有被記成繞過」
// —— 那是「這支出口到底有沒有留下痕跡」唯一的行為面判準（漏了守衛的非零出口 body
// 完全不變，只有這個計數器會動）。判準必須錨在計數器本身，不是錨在測試作者記得。
func EscapedCounterFor(code int32, path string) prometheus.Counter {
	return bizcodeEscapedTotal.WithLabelValues(strconv.FormatInt(int64(code), 10), path)
}

// Armed 回報最舊那筆未消化 arm 的碼，不改變狀態。供測試與診斷使用。
func Armed(c *gin.Context) (int32, bool) {
	h := fromGin(c)
	if h == nil {
		return 0, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.arms) == 0 {
		return 0, false
	}
	return h.arms[0].code, true
}

// PendingCodes 回報所有未消化 arm 的碼（插入序），供測試與診斷使用。
func PendingCodes(c *gin.Context) []int32 {
	h := fromGin(c)
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]int32, 0, len(h.arms))
	for _, p := range h.arms {
		out = append(out, p.code)
	}
	return out
}
