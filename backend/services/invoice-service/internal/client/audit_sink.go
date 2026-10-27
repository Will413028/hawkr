package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/Will413028/hawkr/invoice-service/internal/client/amego"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)

// correlationIDOf 取這一次往返的追查識別碼（#3031）。
//
// 先問 CorrelationInterceptor 種在 ctx 的值（gRPC 請求路徑），沒有就退回 OTel 的
// trace id（排程 worker 路徑）。兩者都沒有時回空字串——那是合法的（例如本機測試），
// 欄位保持 NULL 而不是塞一個編出來的值。
func correlationIDOf(ctx context.Context) string {
	if cid := interceptor.CorrelationIDFromContext(ctx); cid != "" {
		return cid
	}
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}

// repoAuditSink 把 AMEGO 的每一次往返落進 invoice_provider_audit_logs（§8.2.1 的
// 三種必要紀錄之一，5 年保存）。
//
// 在它接上之前這裡是 amego.NoopAuditSink ⇒ **發票開得出來、稽核表一列都沒有**。
// 2026-08-30 的 dev E2E 實際踩到：document 是 ISSUED、發票號碼回填了，而
// invoice_provider_audit_logs 是 0 列。沒有任何測試會紅，因為沒有任何斷言看它。
// 那張表是 5 年後被稽核時「我方到底送了什麼給光貿」的唯一證據。
type repoAuditSink struct {
	repo domain.AuditRepository
	log  *zap.Logger
}

var _ amego.AuditSink = (*repoAuditSink)(nil)

// Record 依 AuditSink 契約**不得回 error、不得 panic 出去**：稽核寫入失敗永遠不可以
// 讓一次開票失敗（發票本身已記在 invoice_documents，為它擋住開票只會白燒法定期限）。
// 失敗的歸宿是 Error log **＋ M14**（`amego.ObserveAuditWriteFailed`）。
//
// 🔴 這裡曾經寫著「invoice_audit_write_failed_total 由 worker 層持有，本層不 import
// 它以免產生 client → worker 的反向依賴」——依賴方向的判斷是對的，但結論停在
// 「所以不遞增」，於是告警 InvoiceAuditWriteFailing 從上線起就是永久靜默的擺設
// （2026-09-05 第 6 輪稽核實測：該指標全服務零觀測點）。正解是把指標搬到依賴的
// **下游**（amego package），不是放棄遞增：一個只寫進 log 的失敗，等於沒有人會知道。
func (s *repoAuditSink) Record(ctx context.Context, ex amego.Exchange) {
	if s == nil || s.repo == nil {
		return
	}
	reqJSON, err := json.Marshal(ex.RequestMasked)
	if err != nil {
		s.logFail(ex, "marshal request", err)
		return
	}
	// ResponseMasked 是原文字串；包成 JSON 字串以符合 jsonb 欄位。
	respJSON, err := json.Marshal(ex.ResponseMasked)
	if err != nil {
		s.logFail(ex, "marshal response", err)
		return
	}
	sum := sha256.Sum256([]byte(ex.ResponseMasked))

	entry := domain.NewProviderAuditLogEntry{
		IssuerID:       ex.IssuerID,
		Provider:       "AMEGO",
		Endpoint:       ex.Endpoint,
		AttemptSeq:     int32(ex.AttemptSeq),
		RequestMasked:  reqJSON,
		ResponseMasked: respJSON,
		RawBodyHash:    hex.EncodeToString(sum[:]),
		Now:            ex.OccurredAt,
	}
	if ex.OperationID != 0 {
		id := ex.OperationID
		entry.OperationID = &id
	}
	if ex.HTTPStatus != 0 {
		st := int32(ex.HTTPStatus)
		entry.HTTPStatus = &st
	}
	if ex.ProviderCode != "" {
		c := ex.ProviderCode
		entry.ProviderCode = &c
	}
	// #3089 第 7 項：與 provider_code 成對的訊息原文。proto 的
	// ProviderExchangeItem.provider_message 逐字要求「必須同時保存 msg」，
	// 而在此之前**這一欄根本不存在於 DB**，ADM 因此分不出同一個數字碼的兩種語意。
	if ex.ProviderMessage != "" {
		m := ex.ProviderMessage
		entry.ProviderMessage = &m
	}
	if ex.ErrorKind != "" {
		k := ex.ErrorKind
		entry.ErrorKind = &k
	}
	// #3031：correlation_id ——「這一次往返」在我方 log／trace 上的識別碼。
	//
	// 🔴 在此之前**從來沒有人填它**（dev 實測 2397 列全部 NULL），而
	// ProviderExchangeItem.request_id 與 SellerProviderAccountData.last_request_id
	// 兩個對外欄位都以它為來源 ⇒ 那兩欄接上之後仍然恆為空。
	// 「有賦值但值恆為空」是 producer gate 第一種擋不住的形態，只有實機驗得到。
	//
	// 兩段來源：gRPC 請求路徑由 CorrelationInterceptor 種進 ctx；排程 worker
	// （INV.op 是主要的呼叫者）沒有請求可種 ⇒ 退回 OTel 的 trace id，
	// 那也是拿去 Loki／Tempo 追同一次呼叫的那個值。
	if cid := correlationIDOf(ctx); cid != "" {
		entry.CorrelationID = &cid
	}
	if ex.LatencyMS != 0 {
		ms := int32(ex.LatencyMS)
		entry.LatencyMs = &ms
	}
	if err := s.repo.AppendProviderAuditLog(ctx, entry); err != nil {
		s.logFail(ex, "append", err)
	}
}

func (s *repoAuditSink) logFail(ex amego.Exchange, stage string, err error) {
	// 🔴 遞增在 nil-logger 早退**之前**：計數是這條路徑的唯一機械訊號，
	// 讓它依賴「有沒有裝 logger」等於把訊號綁在一個與失敗無關的條件上。
	amego.ObserveAuditWriteFailed()
	if s.log == nil {
		return
	}
	s.log.Error("invoice.provider_audit.write_failed",
		zap.String("stage", stage),
		zap.String("endpoint", ex.Endpoint),
		zap.Int64("operation_id", ex.OperationID),
		zap.Int("attempt_seq", ex.AttemptSeq),
		zap.Error(err))
}
