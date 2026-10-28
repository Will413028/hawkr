// Package response 是 Gateway 專屬的 API 回應層：對通用 envelope 契約
// （Envelope／GrpcErr／PageData／ProtoToRaw 等）的呼叫全部轉發給
// pkg/envelope（2026-08-14 pkg/envelope 抽取計畫），本檔案只保留真正
// Gateway 專屬的商業政策——業務碼拒絕（BizDeny／Biz）、連線數上限限流 header
// （SetRateLimitHeaders／TooManyRequests／ConnCapRetryAfter）、AI／採購回應的
// 快取政策值（CacheControlPrivateNoStore）。所有匯出
// 函式簽名與既有呼叫端（response.OK(...)、response.GrpcErr(...)、
// response.Envelope{...} 等，gateway/internal/handler|middleware|proxy|router
// 共 80+ 處）維持完全不變，這次抽取只換掉函式內部實作，不要求任何呼叫端
// 跟著改 import。
package response

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/bizcarrier"
	"github.com/Will413028/hawkr/go-pkg/envelope"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Envelope 是 pkg/envelope.Envelope 的型別別名（非結構相同的獨立型別）：
// gateway 既有的 response.Envelope{...} 字面值建構（如 proxy/reverse_proxy.go）與 swag `@Success 200 {object} response.Envelope`
// 標註無需改動即可繼續編譯／解析。
type Envelope = envelope.Envelope

// PageData 是 pkg/envelope.PageData 的型別別名，理由同 Envelope
// （既有 response.PageData{...} 字面值建構，如 handler/order.go、
// handler/aftersales.go、handler/seller_product_import.go）。
type PageData = envelope.PageData

// ==================== 出口守衛（bizcarrier）====================
//
// 🔴 **本 package 的每一個出口都必須呼叫 guard／guardNonSuccess，一個都不能少。**
//
// 述詞是「**這個出口實際寫進 envelope 的 Code 是不是 0**」，不是「呼叫了哪一個函式」。
// 2026-08-31 的第一版把述詞訂成後者：只有 OK／OKProto／Page／GrpcResult 查 carrier，
// 其餘 18 個出口呼叫 bizcarrier.MarkEmitted。於是 `response.Biz(c, 0, "success", data)`
// —— 一個**已經存在且正在被使用**的 exported helper（handler/checkout.go:553/643/651
// 用的是同一支）—— 就是第十種繞道，而且不需要一行新程式碼：它輸出
// {"code":0,"message":"success","data":…} HTTP 200，同時 MarkEmitted 把 emitted 設成
// true（正是 Middleware 收尾偵測讀的那個旗標）⇒ carrier 原封不動 armed 著沒人看、
// swallowed／escaped 兩個計數器都不動、ERROR log 一行都不出。
// **那比沉默更糟：它還蓋了一個「已檢查」的章。**
//
// guard 的三種結果：
//   - emittedCode != 0        → 不是假成功，只留痕跡並消化掉同碼的 arm，回 false
//   - emittedCode == 0，無 arm → 今天所有既有流量，回 false，逐位元零改變
//   - emittedCode == 0，有 arm → 已就地改寫成 Biz(pendingCode, pendingMsg, data)，
//                                回 true，呼叫端必須立刻 return
//
// 每一支出口「宣告的碼 == 真的寫出去的碼」由 bizcarrier_emitter_matrix_test.go
// 逐支實測（idle／armed 兩種狀態各跑一次 ＋ 逃逸計數器對帳），不是靠這段註解宣稱。

// writeBizEnvelope 是「HTTP 200 ＋ 非零業務碼 ＋ data」這個形狀的**唯一**寫出點，
// Biz 與 carrier 改寫共用它 —— 兩者逐位元相同正是「刪掉 handler 內的守門不會改變
// 任何一個既有回應」的依據，共用同一個寫出點讓那件事不必靠人維護。
func writeBizEnvelope(c *gin.Context, code int32, message string, data interface{}) {
	c.JSON(http.StatusOK, Envelope{
		Code:    code,
		Message: message,
		Data:    data,
		TraceID: envelope.ExtractTraceID(c),
	})
}

// guard 見上方區塊註解。emittedCode 必須是呼叫端接下來真的會寫進 envelope 的 Code；
// data 是改寫時要原樣帶走的 payload（非零碼出口傳 nil，那條路不會改寫）。
func guard(c *gin.Context, emittedCode int32, data interface{}) bool {
	code, msg, rewrite := bizcarrier.Emit(c, emittedCode)
	if !rewrite {
		return false
	}
	writeBizEnvelope(c, code, msg, data)
	return true
}

// guardNonSuccess 供「envelope code 由下游決定、但保證恆非零」的出口使用（今天只有
// GrpcErr 家族：碼是 pkg/envelope.GrpcErr 內部那個 switch 依 gRPC status 決定的，
// 呼叫點拿不到，重算一次等於把那個 switch 抄第二份）。
//
// ⚠️ 它是本檔唯一不交出 code 的守衛，也因此是最容易退化成第二個 MarkEmitted 的地方。
// 守它的是 TestGrpcErrNeverWritesZeroCode（對每一個 gRPC status code 實跑並斷言
// body 的 code != 0），不是這段註解。新增出口時不要圖方便用它。
func guardNonSuccess(c *gin.Context) {
	bizcarrier.EmitNonSuccess(c)
}

// ==================== 成功回應（轉發至 pkg/envelope）====================
//
// EnvelopeGuardInterceptor 對 preserveData 業務碼刻意不轉 error（它們的 data 過不了
// status.Error），於是「resp.Code 非零而 err 為 nil」的形狀會抵達 handler。handler
// 只要忘記看 resp.GetCode() 就會走到這裡，把一個業務失敗寫成 code=0 —— 端點 44
// （AMEGO 憑證驗證失敗 9331 被顯示成「檢測成功」）就是這樣發生的。
//
// 這裡查一次 carrier，那件事就變成寫不出來的，而不是「71 支 handler 每支都要記得」。
// 自動改正**同時**記 ERROR log 與 gateway_bizcode_swallowed_total —— 它是安全網不是
// 正常路徑，計數器長期非零代表有 handler 該回去寫對。
//
// 改正後的輸出與 handler 自己寫 `response.Biz(c, resp.GetCode(), resp.GetMessage(),
// data)` 逐位元相同（HTTP 200 ＋ 非零 code ＋ data 原樣），所以把 handler 內的檢查
// 刪掉不會改變任何一個既有回應。

// OK 成功回應 {code:0, message:"success", data:{...}}
//
// ⚠️ 若本請求有未處理的 preserveData 業務碼，本函式會改送 Biz(code, message, data)。
func OK(c *gin.Context, data interface{}) {
	if guard(c, 0, data) {
		return
	}
	envelope.OK(c, data)
}

// OKProto 成功回應（proto message 直接 passthrough，不二次編碼）
//
// ⚠️ 同 OK：有未處理的業務碼時改送 Biz。data 走 ProtoToRaw 保持與成功路徑同一種
// 序列化（json.RawMessage，不會被二次編碼）。
func OKProto(c *gin.Context, msg proto.Message) {
	if guard(c, 0, ProtoToRaw(msg)) {
		return
	}
	envelope.OKProto(c, msg)
}

// Page 分頁成功回應
//
// ⚠️ 同 OK。改正時 data 手搭成 PageData —— 與 envelope.Page 內部搭的那個逐欄相同
// （records/total/page/pageSize），所以分頁端點被改正後前端讀到的形狀不變。
func Page(c *gin.Context, records interface{}, total, page, pageSize string) {
	if guard(c, 0, PageData{
		Records:  records,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}) {
		return
	}
	envelope.Page(c, records, total, page, pageSize)
}

// ==================== 錯誤回應（轉發至 pkg/envelope；自動帶 traceId）====================

// Err 通用錯誤回應
func Err(c *gin.Context, httpStatus int, code int32, message string) {
	// code 是呼叫端給的，可能是 0（envelope.Err 會照寫）⇒ 必須交出去讓 guard 判。
	if guard(c, code, nil) {
		return
	}
	envelope.Err(c, httpStatus, code, message)
}

// BadRequest 400 錯誤
func BadRequest(c *gin.Context, message string) {
	if guard(c, 400, nil) {
		return
	}
	envelope.BadRequest(c, message)
}

// FieldError is one field-level validation failure, carried as part of a
// 400's structured detail. No generic helper for this existed in this
// codebase before the PAYUni merchant-profile PATCH (issue #2806 Phase 2,
// spec §6.3 "缺必要聯絡資料等，回 structured field errors") — every other
// handler that declares `binding` tags relies on c.ShouldBindJSON's own
// message-only 400 (see gateway/internal/handler/address.go), and there is
// no manual binding.Validator.ValidateStruct call anywhere else to model
// this on. This is deliberately built on the existing Envelope{Data:...}
// pattern already used by Biz/BizStatusData rather than a new envelope
// shape — only the per-endpoint Data payload (this type) is new, the same
// way ValidationErrorResponse is new-but-Envelope-shaped for CSV import.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// BadRequestFields is BadRequest plus structured per-field detail in Data,
// for a caller that needs more than one message to fix a multi-field
// request (e.g. a binding-tag validation failure on a JSON body).
func BadRequestFields(c *gin.Context, message string, fields []FieldError) {
	if guard(c, http.StatusBadRequest, nil) {
		return
	}
	c.JSON(http.StatusBadRequest, Envelope{
		Code:    http.StatusBadRequest,
		Message: message,
		Data:    gin.H{"fieldErrors": fields},
		TraceID: envelope.ExtractTraceID(c),
	})
}

// Unauthorized 401 未授權
func Unauthorized(c *gin.Context) {
	if guard(c, 401, nil) {
		return
	}
	envelope.Unauthorized(c)
}

// AbortUnauthorized 401 未授權 + 自訂 message + 中止 handler chain。
// 用於 middleware 風格的身分守門（取不到 accountId 就不該讓後續 handler 跑），
// Unauthorized 只寫回應不中止、且訊息固定，兩者差異僅此。
func AbortUnauthorized(c *gin.Context, message string) {
	// Abort 在 guard 之前：本函式的契約是「一定中止 chain」，改寫路徑也不例外
	// （改寫代表守門拒絕 ＋ 有未消化的業務碼，更沒有理由讓後續 handler 跑）。
	c.Abort()
	if guard(c, 401, nil) {
		return
	}
	envelope.AbortUnauthorized(c, message)
}

// AbortForbidden 403 ＋ 中止 handler chain，與 AbortUnauthorized 同一條契約：
// middleware 風格的守門拒絕之後，後續 handler 一律不得執行。
//
// ⚠️ 與 Forbidden 的差別只有「會不會中止」。守門用這支、handler 內部用 Forbidden
// ——混用的後果是守門寫了回應卻讓後續 handler 照跑，而那在測試裡看起來像「回了 403」。
func AbortForbidden(c *gin.Context, message string) {
	// Abort 在 guard 之前，理由同 AbortUnauthorized。
	c.Abort()
	if guard(c, 403, nil) {
		return
	}
	envelope.Forbidden(c, message)
}

// Forbidden 403 禁止存取
func Forbidden(c *gin.Context, message string) {
	if guard(c, 403, nil) {
		return
	}
	envelope.Forbidden(c, message)
}

// NotFound 404 找不到資源
func NotFound(c *gin.Context, message string) {
	if guard(c, 404, nil) {
		return
	}
	envelope.NotFound(c, message)
}

// ServiceUnavailable 503 微服務不可達
func ServiceUnavailable(c *gin.Context, service string) {
	if guard(c, 503, nil) {
		return
	}
	envelope.ServiceUnavailable(c, service)
}

// ==================== Gateway 專屬商業政策（不搬移）====================

// BizDeny 送出 Gateway 原生的業務拒絕：HTTP 409（塌縮）+ envelope 帶業務碼
// （前端解析 code / messageZh，不用 HTTP status 分流；409 也用於 duplicate，見
// reference_gateway_bizcode_collapses_to_409）。輸出 envelope 形狀與 GrpcDomainErr
// 的業務碼分支一致（Code=業務碼、Message=MessageZh=乾淨中文）。
func BizDeny(c *gin.Context, code int32, message string) {
	if guard(c, code, nil) {
		return
	}
	c.JSON(http.StatusConflict, Envelope{
		Code:      code,
		Message:   message,
		MessageZh: message,
		TraceID:   envelope.ExtractTraceID(c),
	})
}

// Biz 送出「HTTP 200 + 非零業務碼 + data」的 envelope。
//
// GrpcResult 對 code≠0 一律丟棄 data、Err 系列也不帶 data，但有一類業務拒絕本身就
// 附帶結構化明細（結帳 4001 unavailableItems / 4091 priceChanged、OAuth 1303/1304
// 的續行資訊），前端要靠 data 才能把對話框畫出來。此時仍是統一 envelope
// {code,message,data}，只是 code 非零，HTTP 維持 200（業務碼不走 HTTP status 分流，
// 見 reference_gateway_bizcode_collapses_to_409）。
func Biz(c *gin.Context, code int32, message string, data interface{}) {
	// 🔴 code 是呼叫端給的。`Biz(c, 0, "success", data)` 就是第十種繞道（checkout.go
	// 的三個呼叫點用的是同一支），所以這裡**必須**把 code 交出去，不能像第一版那樣
	// 無條件 MarkEmitted —— 那等於替一個假成功蓋「已檢查」的章。
	if guard(c, code, data) {
		return
	}
	writeBizEnvelope(c, code, message, data)
}

// ConnCapRetryAfter 是連線數上限（SSE / WebSocket）429 給客戶端的退避提示。
//
// 連線槽沒有時間窗——它在既有連線結束時才釋放，所以這裡沒有「本窗還剩幾秒」可以
// 算，只能給一個退避建議。取 10 秒的理由：唯一已知消費者（web 的 SSE client）在
// 沒有 Retry-After 時退回固定 8 秒，給比它更長的值才不會把重連壓得更密；同時遠低
// 於該 client 對 Retry-After 的 30 秒上限，不會白等。
const ConnCapRetryAfter = 10 * time.Second

// SetRateLimitHeaders 寫入 429 的重試提示 header 組。limit <= 0 代表「上限不可知」
// （例如連線數上限的呼叫端拿不到當下生效的 cap），此時不輸出 X-RateLimit-Limit：
// 少一個欄位好過給一個假數字。
//
// retryAfter 送秒數而非 HTTP-date（RFC 7231 兩者皆合法，秒數對客戶端最好處理）。
// X-RateLimit-Reset 是「最遲何時重置」的上界：固定窗限流器不記錄窗起始時刻，
// 等滿一個窗一定能通過，等更短則不保證。
//
// ⚠️ 跨源前端要讀得到這些 header，入口（單體）的 CORS 必須把它們列進
// Access-Control-Expose-Headers——Retry-After 與 X-RateLimit-* 都不在 CORS
// safelist，沒列就恆為 null。
func SetRateLimitHeaders(c *gin.Context, limit, remaining int, retryAfter time.Duration) {
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	c.Header("Retry-After", strconv.Itoa(seconds))
	if limit > 0 {
		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
	}
	if remaining < 0 {
		remaining = 0
	}
	c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(retryAfter).Unix(), 10))
}

// TooManyRequests 429 業務節流（連線數上限這類無時間窗的容量拒絕）。
//
// 這是 Gateway 四個 429 產生點之一（01 設計書 §3.2.9）：handler/chat.go 的 WS 連線
// 數上限走這裡。GrpcErr 的 ResourceExhausted 分支刻意**不**帶這組 header——那個額度
// 屬於上游微服務，Gateway 既不知窗長也不知重置時刻，硬給只會產生騙人的數字（#2272）。
func TooManyRequests(c *gin.Context, message string) {
	SetRateLimitHeaders(c, 0, 0, ConnCapRetryAfter)
	Err(c, http.StatusTooManyRequests, 429, message)
}

// ==================== 快取政策（Gateway 專屬）====================

// CacheControlPrivateNoStore 是 AI 與採購（Hawkr 幫找）回應的 Cache-Control 值。
// 這條路徑的回應逐一綁定登入者身分（對話內容、候選廠商、報價金額），被共享快取或
// 瀏覽器磁碟快取留下，同一台機器的下一個使用者就讀得到。no-store 連寫入都禁止，
// 比原本的 no-cache（允許寫入、只要求重新驗證）強一級（#2447 BE-01A）。
//
// 這是 AI／採購端點 Cache-Control 的唯一來源：middleware.PrivateNoStore 與兩處
// SSE header 都引用這個常數，不各自寫字面值。seller_city_summary 的 `public, max-age=300` 是刻意不同的快取
// 政策（可共享的公開資料），不要合併過來。判準見 API 設計規範「回應快取政策」。
const CacheControlPrivateNoStore = "private, no-store"

// ==================== gRPC 回應轉發（轉發至 pkg/envelope）====================

// GrpcResult 轉發 gRPC 回應
// code=0 → 成功回應帶 data
// code≠0 → 錯誤回應帶 traceId（不帶 data）
func GrpcResult(c *gin.Context, code int32, message string, data interface{}) {
	// code 由呼叫端給：==0 走成功形狀（要查 carrier），!=0 走 envelope.GrpcResult 的
	// 錯誤形狀（丟 data，結帳 4092 走這條）。兩者都交給 guard 用實際的 code 判定。
	if guard(c, code, data) {
		return
	}
	envelope.GrpcResult(c, code, message, data)
}

// GrpcErr 將 gRPC 錯誤轉為 HTTP 錯誤回應
// 根據 gRPC status code 映射對應的 HTTP 狀態碼
func GrpcErr(c *gin.Context, service string, err error) {
	// 🔴 唯一使用 guardNonSuccess 的地方：envelope code 由下面那個 switch ／
	// pkg/envelope.GrpcErr 依 gRPC status 決定（401/403/404/409/429/499/500/503 或
	// "[NNNN] " 還原出的業務碼），呼叫點算不出來，而它**恆非零**——這件事由
	// TestGrpcErrNeverWritesZeroCode 對每一個 status code 實跑斷言，不是宣稱。
	guardNonSuccess(c)
	// 代行營運的授權拒絕（9262 能力不足、9263 不是這家店的資源、9261 工作階段已撤銷）
	// 與參數拒絕（9260）由微服務側的 delegation.Validate 以 "[NNNN] " 前綴帶回，而
	// pkg/envelope 的 PermissionDenied／InvalidArgument 兩個分支不做前綴還原。不在這裡
	// 先攔一次就只剩通用的 403／400，前端分不出上述三種——而它們的恢復動作完全不同
	// （換一個能力／回列表重選店／重新交接）。
	//
	// 攔截條件是「訊息帶已登錄的業務碼前綴」，未登錄的碼一律沿用該 gRPC 分支原本的
	// status——每個 case 的 fallback 就是 envelope.GrpcErr 對同一分支使用的值，所以未
	// 登錄碼逐位元無變化。登錄表在 bizhttp.go。
	//
	// ⚠️ **每一個「會讓業務碼帶著前綴活著抵達這裡」的 gRPC 分支都必須列在下面**，漏一
	// 個就是一條繞過登錄表的暗路。最容易漏的是 FailedPrecondition：
	// EnvelopeGuardInterceptor 對**沒有登錄在 envelopeCodeToGRPC 的業務碼**一律轉成
	// FailedPrecondition ＋ "[NNNN] " 前綴，那一支才是絕大多數領域業務碼的實際入口。
	// 商品認證的 3025／3028 就是漏了它而在 dev 上雙雙塌成 409（#2755）——登錄表是對的、
	// BizStatus 是對的，只是那兩個碼從來沒有走到查表的那條路。
	if st, ok := grpcstatus.FromError(err); ok {
		switch st.Code() {
		case codes.PermissionDenied:
			if writeBizCodeEnvelopeMapped(c, http.StatusForbidden, st.Message()) {
				return
			}
		case codes.InvalidArgument:
			if writeBizCodeEnvelopeMapped(c, http.StatusBadRequest, st.Message()) {
				return
			}
		case codes.FailedPrecondition:
			if writeBizCodeEnvelopeMapped(c, http.StatusConflict, st.Message()) {
				return
			}
		case codes.NotFound:
			if writeBizCodeEnvelopeMapped(c, http.StatusNotFound, st.Message()) {
				return
			}
		case codes.ResourceExhausted:
			if writeBizCodeEnvelopeMapped(c, http.StatusTooManyRequests, st.Message()) {
				return
			}
		}
	}
	envelope.GrpcErr(c, service, err)
}

// GrpcDomainErr 為 GrpcErr 的相容別名（issue #1479 起兩者行為完全一致：GrpcErr 的
// FailedPrecondition 分支已統一做業務碼透傳，不再需要獨立函式區分「業務碼模式」）。
// 保留此名稱只為既有呼叫點不必改動；新 handler 一律直接呼叫 GrpcErr。
func GrpcDomainErr(c *gin.Context, service string, err error) {
	// 轉呼本地的 GrpcErr 而非 envelope.GrpcDomainErr：上面那行「兩者行為完全一致」
	// 是這個別名存在的全部理由，而本地 GrpcErr 比 envelope 版多了代行業務碼的前綴
	// 還原。直接委派 envelope 會讓兩者從今天起不再等價——19 個既有呼叫點今天都不是
	// 代行路徑所以看不出差別，但下一個把代行 handler 接到這個別名上的人會拿到一個
	// 塌掉業務碼的 403／400，而註解還在向他保證兩者一致。
	// （envelope.GrpcDomainErr 自己也只是轉呼 envelope.GrpcErr，見 pkg/envelope/envelope.go:259。）
	GrpcErr(c, service, err)
}

// ==================== Proto 序列化（轉發至 pkg/envelope）====================

// ProtoToRaw 將 proto message 序列化為 json.RawMessage，
// 可直接放入 gin.H 或 Envelope.Data 而不會被二次編碼。
func ProtoToRaw(msg proto.Message) json.RawMessage { return envelope.ProtoToRaw(msg) }
