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

func correlationIDOf(ctx context.Context) string {
	if cid := interceptor.CorrelationIDFromContext(ctx); cid != "" {
		return cid
	}
	if sc := trace.SpanContextFromContext(ctx); sc.HasTraceID() {
		return sc.TraceID().String()
	}
	return ""
}

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
	if ex.ProviderMessage != "" {
		m := ex.ProviderMessage
		entry.ProviderMessage = &m
	}
	if ex.ErrorKind != "" {
		k := ex.ErrorKind
		entry.ErrorKind = &k
	}
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
