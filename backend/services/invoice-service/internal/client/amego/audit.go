package amego

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

// Exchange 是一次與 AMEGO 的往返，落 invoice_provider_audit_logs（append-only，5 年保存）。
type Exchange struct {
	// Endpoint 是 /json/f0401 這種**路徑**，不是完整 URL：
	// 同一個數字碼跨 API 意義不同（71 在 invoice_file 是查無資料、99 在 ban_query
	// 是統編長度錯），只存 code 無法還原問題。
	Endpoint string
	// IssuerID 取 Credential.IssuerID，**不是 store_id**：平台自身開票時沒有 store_id。
	IssuerID int64
	// OperationID／AttemptSeq 由 ctx 帶入（WithAuditContext）。
	// 答不出「這筆到底送了幾遍」，而那是與光貿對帳的第一個問題。
	OperationID int64
	AttemptSeq  int
	// RequestMasked 是送出的 data JSON（業務內容原樣，供事後重算）＋ time ＋
	// **只留前 8 碼的 sign**。全長 sign ＋ 已落庫的 data／time ＝ 離線暴力破解
	// AppKey 的 oracle。AppKey 本身永不出現在這裡。
	RequestMasked map[string]string
	// ResponseMasked 是回應原文（含 msg）。稅務舉證需要原文。
	ResponseMasked string
	// HTTPStatus 恆為 200，但仍要記：哪天不是 200 就是重要訊號。
	HTTPStatus int
	// ProviderCode 一律以字串寫入（折讓類回 String、查詢類回 Number）。
	ProviderCode string
	// ProviderMessage 是與 ProviderCode 成對的訊息原文（#3089 第 7 項）。
	//
	// 🔴 **只存 code 還原不出問題**：同一個數字在官方碼表出現多次而語意不同
	// （53 是最明確的一個）。原文雖然也在 ResponseMasked 裡，但那是一團 jsonb，
	// ADM 的服務商往來列表要一眼看到必須有獨立欄。成功時為空。
	ProviderMessage string
	// ErrorKind 是 TIMEOUT／CONNECTION／PROVIDER_ERROR／DECODE 四選一，成功時為空。
	ErrorKind  string
	LatencyMS  int64
	OccurredAt time.Time
}

// AuditSink 是稽核落庫的 hook，形狀照抄 PayUni 的 PayUniRequestLogger。
//
// 🔴 Record **不得回 error、不得 panic 出去**：稽核寫入失敗永遠不可以讓一次開票
// 失敗（發票本身已記在 invoice_documents，這張表是診斷與舉證用、不是稅務紀錄，
// 為它擋住開票只會白燒法定期限）。實作端把失敗推 invoice_audit_write_failed_total。
type AuditSink interface {
	Record(ctx context.Context, ex Exchange)
}

// NoopAuditSink 是預設實作 ⇒ client package 在無 DB 的單元測試裡可用。
type NoopAuditSink struct{}

// Record 什麼都不做。
func (NoopAuditSink) Record(context.Context, Exchange) {}

type auditCtxKey struct{}

type auditCtxValue struct {
	operationID int64
	attemptSeq  int
}

// WithAuditContext 把 operation_id 與 attempt_seq 綁進 ctx，讓 client 不必在每個
// 方法簽章上多兩個與業務無關的參數。
func WithAuditContext(ctx context.Context, operationID int64, attemptSeq int) context.Context {
	return context.WithValue(ctx, auditCtxKey{}, auditCtxValue{operationID: operationID, attemptSeq: attemptSeq})
}

// auditFirstAttempt 是 attempt_seq 的下限。
//
// 🔴 **不是 0**：`chk_invpaudit_attempt` 是 `attempt_seq >= 1`，而
// `repoAuditSink.record` 的失敗只會被 `logFail` 記成一行 error 就吞掉
// （吞掉本身是對的——稽核寫不進去不該讓開票失敗）。兩者相乘的結果是
// **整張 invoice_provider_audit_logs 一列都沒有，而沒有任何東西會紅**。
//
// 2026-09-01 實測：上線至今 0 列，同 DB 的 invoice_status_history 有 89 列、
// invoice_admin_audit_logs 有 3 列（對照組，證明查得到）。成因是沒有任何
// production 呼叫端呼叫過 WithAuditContext ⇒ 這裡回 0 ⇒ 每一筆都違反 CHECK。
//
// 語意上這個預設也是對的：不繫在可重試 operation 上的呼叫（連線測試、健康檢查）
// 就是它自己的第一次嘗試。
const auditFirstAttempt = 1

func auditFromContext(ctx context.Context) (int64, int) {
	v, ok := ctx.Value(auditCtxKey{}).(auditCtxValue)
	if !ok {
		return 0, auditFirstAttempt
	}
	if v.attemptSeq < auditFirstAttempt {
		// 呼叫端算錯（例如把 0-based 的 op.AttemptCount 直接傳進來）也不能讓
		// 整列稽核消失 —— 落一列 attempt_seq=1 遠勝於落 0 列。
		return v.operationID, auditFirstAttempt
	}
	return v.operationID, v.attemptSeq
}

// safeRecord 保證 sink 的任何 panic 都不會外洩到開票路徑上。
//
// ctx 一律用 context.WithoutCancel：外呼逾時時 ctx 已取消，直接沿用會讓
// **最需要留下證據的那一筆反而寫不進去**（同一手法見 pkg/database 的 rollback 路徑）。
func safeRecord(ctx context.Context, sink AuditSink, ex Exchange) {
	if sink == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	sink.Record(context.WithoutCancel(ctx), ex)
}

// ensureCorrelationID 保證 ctx 帶著這一次往返的識別碼（#3031）。
//
// 取值順序，前兩者是「真的可以拿去別處追」的：
//
//	① gRPC 請求路徑由 CorrelationInterceptor 種進來的 correlation id
//	② OTel 的 trace id（本服務目前沒有任何地方起 span，所以實務上不會命中；
//	   留著是因為哪天 worker 起了 span，這一格會自動變成真的 trace key）
//	③ 都沒有 ⇒ 現生一個。它只識別這一次往返本身。
//
// 已經有值時**不覆蓋**：同一次請求打多次 AMEGO（例如開票後立刻查詢）應該共用同一個 id。
func ensureCorrelationID(ctx context.Context) context.Context {
	if interceptor.CorrelationIDFromContext(ctx) != "" {
		return ctx
	}
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		return interceptor.WithCorrelationID(ctx, sc.TraceID().String())
	}
	return interceptor.WithCorrelationID(ctx, uuid.NewString())
}
