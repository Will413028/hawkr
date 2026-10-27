// Package envelope 提供 Hawkr 後端服務共用的統一 API 回應 envelope：
// 成功／錯誤／分頁／gRPC 錯誤映射／proto passthrough 序列化，一份實作供
// Gateway 與各微服務共用，不必各自手刻
// c.JSON(status, gin.H{"code":N,"message"/"msg":...})。
//
// 本套件是 gateway/internal/response 的抽取結果（2026-08-14 pkg/envelope
// 抽取計畫）：只搬移不含 Gateway 專屬商業政策的通用機制（Envelope／GrpcErr／
// PageData／ProtoToRaw 及其依賴閉包）；Gateway 專屬的商業層（BizDeny／Biz／
// OAuth 手機驗證／限流 429 header）仍留在 gateway/internal/response，該套件
// 現在是本套件之上的一層薄轉發＋政策層，對外函式簽名與行為完全不變。
//
// ExtractTraceID 目前只認得 gin context key "trace_id"（由 gateway 的
// TracingMiddleware 寫入）；非 gateway 呼叫端（尚未接等效 middleware）會退回
// x-trace-id / x-request-id header fallback，取不到就是空字串——這是已知、
// 可接受的漸進式行為，不是本套件的 bug。
package envelope

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Envelope 統一 API 回應結構
// 成功: {code:0, message:"success", data:{...}}
// 錯誤（通用）: {code:N, message:"...", traceId:"..."}
// 錯誤（業務碼）: {code:9201, message:"...", messageZh:"...", traceId:"..."}
type Envelope struct {
	Data      interface{} `json:"data,omitempty"`
	Message   string      `json:"message"`
	MessageZh string      `json:"messageZh,omitempty"`
	TraceID   string      `json:"traceId,omitempty"`
	Code      int32       `json:"code"`
}

// bizMsgRE 解析 "[9201] 不能傳訊息給自己" 格式
var bizMsgRE = regexp.MustCompile(`^\[(\d+)\]\s*(.*)$`)

// parseBizMessage 從 "[NNNN] message" 解析出業務碼與訊息。
// 若格式不符回傳 (0, "")。
func parseBizMessage(msg string) (int32, string) {
	m := bizMsgRE.FindStringSubmatch(msg)
	if m == nil {
		return 0, ""
	}
	n, err := strconv.ParseInt(m[1], 10, 32)
	if err != nil {
		return 0, ""
	}
	return int32(n), m[2]
}

// ParseBizMessage 是 parseBizMessage 的公開入口。
//
// 開放它是因為 Gateway 需要在本套分支策略之外做自己的 HTTP status 分流（代行營運
// §6.5.2：9260 對 400、9262 對 403、9266 對 409 各自不同），而那張登錄表是 Gateway
// 專屬概念、不該進共用模組。與其讓呼叫端各自重刻一份正則——規則一漂移就靜默解析
// 失敗、回一個沒有恢復指引的通用碼——不如把唯一那份解析規則公開。
func ParseBizMessage(msg string) (int32, string) { return parseBizMessage(msg) }

// PageData 標準分頁結構（放在 Envelope.Data 中）
type PageData struct {
	Records  interface{} `json:"records"`
	Total    string      `json:"total"`
	Page     string      `json:"page"`
	PageSize string      `json:"pageSize"`
}

// ==================== 成功回應 ====================

// OK 成功回應 {code:0, message:"success", data:{...}}
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Envelope{Code: 0, Message: "success", Data: data})
}

// OKProto 成功回應（proto message 直接 passthrough，不二次編碼）
func OKProto(c *gin.Context, msg proto.Message) { OK(c, ProtoToRaw(msg)) }

// Page 分頁成功回應
func Page(c *gin.Context, records interface{}, total, page, pageSize string) {
	c.JSON(http.StatusOK, Envelope{
		Code:    0,
		Message: "success",
		Data: PageData{
			Records:  records,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

// ==================== 錯誤回應（自動帶 traceId）====================

// Err 通用錯誤回應
func Err(c *gin.Context, httpStatus int, code int32, message string) {
	c.JSON(httpStatus, Envelope{
		Code:    code,
		Message: message,
		TraceID: ExtractTraceID(c),
	})
}

// BadRequest 400 錯誤
func BadRequest(c *gin.Context, message string) {
	Err(c, http.StatusBadRequest, 400, message)
}

// Unauthorized 401 未授權
func Unauthorized(c *gin.Context) {
	Err(c, http.StatusUnauthorized, 401, "unauthorized")
}

// AbortUnauthorized 401 未授權 + 自訂 message + 中止 handler chain。
// 用於 middleware 風格的身分守門（取不到 accountId 就不該讓後續 handler 跑），
// Unauthorized 只寫回應不中止、且訊息固定，兩者差異僅此。
func AbortUnauthorized(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, Envelope{
		Code:    401,
		Message: message,
		TraceID: ExtractTraceID(c),
	})
}

// Forbidden 403 禁止存取
func Forbidden(c *gin.Context, message string) {
	Err(c, http.StatusForbidden, 403, message)
}

// NotFound 404 找不到資源
func NotFound(c *gin.Context, message string) {
	Err(c, http.StatusNotFound, 404, message)
}

// ServiceUnavailable 503 微服務不可達
func ServiceUnavailable(c *gin.Context, service string) {
	Err(c, http.StatusServiceUnavailable, 503, service+" service unavailable")
}

// ==================== gRPC 回應轉發 ====================

// GrpcResult 轉發 gRPC 回應
// code=0 → 成功回應帶 data
// code≠0 → 錯誤回應帶 traceId（不帶 data）
func GrpcResult(c *gin.Context, code int32, message string, data interface{}) {
	if code != 0 {
		c.JSON(http.StatusOK, Envelope{
			Code:    code,
			Message: message,
			TraceID: ExtractTraceID(c),
		})
		return
	}
	c.JSON(http.StatusOK, Envelope{Code: 0, Message: "success", Data: data})
}

// writeBizCodeEnvelopeIfPrefixed 若 message 符合 "[NNNN] xxx" 前綴格式，還原業務碼
// 進 envelope Code（message 去前綴），回傳 true；否則不寫入回應、回傳 false 交由呼叫端
// 走預設塌縮路徑。GrpcErr 各 status 分支共用同一份還原邏輯。
//
// httpStatus 由呼叫端的 gRPC status 分支決定：業務碼一律不走 HTTP status 分流，
// 但同一個 status 分支內的塌縮值必須與該分支的預設一致，否則同一類錯誤會因為
// 「有沒有帶業務碼」而回不同的 HTTP status。
func writeBizCodeEnvelopeIfPrefixed(c *gin.Context, httpStatus int, msg string) bool {
	bizCode, bizMsg := parseBizMessage(msg)
	if bizCode <= 0 {
		return false
	}
	c.JSON(httpStatus, Envelope{
		Code:      bizCode,
		Message:   bizMsg,
		MessageZh: bizMsg,
		TraceID:   ExtractTraceID(c),
	})
	return true
}

// GrpcErr 將 gRPC 錯誤轉為 HTTP 錯誤回應
// 根據 gRPC status code 映射對應的 HTTP 狀態碼
func GrpcErr(c *gin.Context, service string, err error) {
	st, ok := grpcstatus.FromError(err)
	if !ok {
		ServiceUnavailable(c, service)
		return
	}
	switch st.Code() {
	case codes.Unauthenticated:
		Unauthorized(c)
	case codes.PermissionDenied:
		Forbidden(c, st.Message())
	case codes.NotFound:
		if writeBizCodeEnvelopeIfPrefixed(c, http.StatusNotFound, st.Message()) {
			return
		}
		NotFound(c, st.Message())
	case codes.InvalidArgument:
		BadRequest(c, st.Message())
	case codes.AlreadyExists:
		Err(c, http.StatusConflict, 409, st.Message())
	case codes.FailedPrecondition:
		// 業務碼統一透傳（issue #1479）：訊息帶 "[NNNN] " 前綴即還原為 envelope Code，
		// message 去前綴。無前綴維持 code=409 塌縮。行為與 GrpcDomainErr 一致
		// （該函式現為 GrpcErr 的相容別名）。
		if writeBizCodeEnvelopeIfPrefixed(c, http.StatusConflict, st.Message()) {
			return
		}
		Err(c, http.StatusConflict, 409, st.Message())
	case codes.Aborted:
		// 樂觀鎖衝突 / 同事務衝突等：屬業務拒絕，HTTP 409 透傳訊息
		Err(c, http.StatusConflict, 409, st.Message())
	case codes.ResourceExhausted:
		// 容量上限（並發 job 數、quota 等）：HTTP 429 透傳訊息。帶 "[NNNN] " 前綴者
		// 還原成業務碼——限流 middleware 也寫 HTTP 429 但 envelope code 是 429，前端
		// 得靠 code 才分得出「額度用完」與「請求太頻繁」（#2272）。
		if writeBizCodeEnvelopeIfPrefixed(c, http.StatusTooManyRequests, st.Message()) {
			return
		}
		Err(c, http.StatusTooManyRequests, 429, st.Message())
	case codes.Internal:
		Err(c, http.StatusInternalServerError, 500, st.Message())
	case codes.Unavailable, codes.DeadlineExceeded:
		// 上游服務（含外部 API）不可用，透傳 message 讓前端顯示具體原因
		msg := st.Message()
		if msg == "" {
			msg = service + " service unavailable"
		}
		Err(c, http.StatusServiceUnavailable, 503, msg)
	case codes.Canceled:
		// 呼叫方主動斷線（分頁關閉/App 背景化/導頁）會讓 c.Request.Context() 取消，
		// 沿此 context 發出的 gRPC 呼叫收到 codes.Canceled——這不是下游服務異常。
		// 寫 499（nginx 慣例 "Client Closed Request"）而非 503，讓
		// http_requests_total 的 5xx 率不被 client 噪音污染。
		if c.Request.Context().Err() != nil {
			Err(c, 499, 499, service+" request canceled by client")
			return
		}
		ServiceUnavailable(c, service)
	default:
		ServiceUnavailable(c, service)
	}
}

// GrpcDomainErr 為 GrpcErr 的相容別名（issue #1479 起兩者行為完全一致：GrpcErr 的
// FailedPrecondition 分支已統一做業務碼透傳，不再需要獨立函式區分「業務碼模式」）。
// 保留此名稱只為既有呼叫點不必改動；新 handler 一律直接呼叫 GrpcErr。
func GrpcDomainErr(c *gin.Context, service string, err error) {
	GrpcErr(c, service, err)
}

// ==================== Proto 序列化 ====================

// protoMarshaler 使用 protojson 序列化，不會丟掉 false / 0 / "" 等零值
var protoMarshaler = protojson.MarshalOptions{EmitUnpopulated: true}

// ProtoToRaw 將 proto message 序列化為 json.RawMessage，
// 可直接放入 gin.H 或 Envelope.Data 而不會被二次編碼。
func ProtoToRaw(msg proto.Message) json.RawMessage {
	b, err := protoMarshaler.Marshal(msg)
	if err != nil {
		return nil
	}
	return b
}

// ==================== 內部工具 ====================

// ExtractTraceID 從請求中提取 trace ID。
//
// 呼叫端若不是 gateway（尚未接等效的 tracing middleware 寫入 "trace_id"
// context key），會退回讀 header——沒有 header 就是空字串，這是預期的
// 漸進式行為，見本檔套件文件。
func ExtractTraceID(c *gin.Context) string {
	// 優先從 TracingMiddleware 注入的 gin context 取（key 與
	// gateway/internal/middleware/tracing.go 一致）
	if tid, exists := c.Get("trace_id"); exists {
		if s, ok := tid.(string); ok && s != "" {
			return s
		}
	}
	// fallback: 上游 propagate 的 header
	if tid := c.GetHeader("x-trace-id"); tid != "" {
		return tid
	}
	if tid := c.GetHeader("x-request-id"); tid != "" {
		return tid
	}
	return ""
}
