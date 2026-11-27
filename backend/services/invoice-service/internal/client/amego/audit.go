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

const auditFirstAttempt = 1

func auditFromContext(ctx context.Context) (int64, int) {
	v, ok := ctx.Value(auditCtxKey{}).(auditCtxValue)
	if !ok {
		return 0, auditFirstAttempt
	}
	if v.attemptSeq < auditFirstAttempt {
		return v.operationID, auditFirstAttempt
	}
	return v.operationID, v.attemptSeq
}

func safeRecord(ctx context.Context, sink AuditSink, ex Exchange) {
	if sink == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	sink.Record(context.WithoutCancel(ctx), ex)
}

func ensureCorrelationID(ctx context.Context) context.Context {
	if interceptor.CorrelationIDFromContext(ctx) != "" {
		return ctx
	}
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		return interceptor.WithCorrelationID(ctx, sc.TraceID().String())
	}
	return interceptor.WithCorrelationID(ctx, uuid.NewString())
}
