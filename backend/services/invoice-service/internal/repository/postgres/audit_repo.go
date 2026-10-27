package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// AuditRepository 蓋三張 append-only 表：狀態歷程、provider 往返稽核、ADM 操作稽核。
//
// 三張表都沒有 UPDATE 與 DELETE，這是保存義務（5 年，§9.5）在資料層的形狀。清理由
// 維運 SQL 依保存期限執行，不寫進 application code（《後端服務責任邊界規範》）。
type AuditRepository struct {
	baseRepo
}

// NewAuditRepository 建立 repository。
func NewAuditRepository(pool *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.AuditRepository = (*AuditRepository)(nil)

// AppendStatusHistory 記一次狀態轉移。
//
// 必須與造成該轉移的那次寫入在**同一個交易**內（呼叫端包 TxManager.WithTx）：分開寫
// 會讓「狀態已變但歷程沒記」成為可能，而那正是事後查不出「誰把它改成這樣」的來源。
func (r *AuditRepository) AppendStatusHistory(ctx context.Context, in domain.NewStatusHistoryEntry) error {
	return r.q(ctx).InsertInvoiceStatusHistory(ctx, gen.InsertInvoiceStatusHistoryParams{
		SubjectType:    in.SubjectType,
		SubjectID:      in.SubjectID,
		FromStatus:     in.FromStatus,
		ToStatus:       in.ToStatus,
		ReasonCode:     in.ReasonCode,
		ReasonDetail:   in.ReasonDetail,
		ActorType:      in.ActorType,
		ActorAccountID: in.ActorAccountID,
		CorrelationID:  in.CorrelationID,
		Now:            ts(in.Now),
	})
}

// ListStatusHistory 回某個主體的完整狀態歷程。
func (r *AuditRepository) ListStatusHistory(ctx context.Context, subjectType string, subjectID int64) ([]domain.StatusHistoryEntry, error) {
	rows, err := r.q(ctx).ListInvoiceStatusHistory(ctx, gen.ListInvoiceStatusHistoryParams{
		SubjectType: subjectType,
		SubjectID:   subjectID,
	})
	if err != nil {
		return nil, err
	}
	return toStatusHistoryEntrys(rows), nil
}

// AppendProviderAuditLog 記一次 AMEGO 往返。
//
// ⚠️ RequestMasked／ResponseMasked 由呼叫端負責遮罩，本層不做也不補救——它不知道哪些
// 欄位敏感。AppKey 本來就不在 AMEGO 的四欄 form 裡（只有 invoice／data／time／sign），
// 所以存 request 原文不會存到 AppKey；sign 是 md5 輸出，原像抗性足夠，可以原樣入庫。
func (r *AuditRepository) AppendProviderAuditLog(ctx context.Context, in domain.NewProviderAuditLogEntry) error {
	return r.q(ctx).InsertInvoiceProviderAuditLog(ctx, gen.InsertInvoiceProviderAuditLogParams{
		OperationID:     in.OperationID,
		IssuerID:        in.IssuerID,
		Provider:        in.Provider,
		Endpoint:        in.Endpoint,
		AttemptSeq:      in.AttemptSeq,
		RequestMasked:   in.RequestMasked,
		ResponseMasked:  in.ResponseMasked,
		RawBodyHash:     in.RawBodyHash,
		HttpStatus:      in.HTTPStatus,
		ProviderCode:    in.ProviderCode,
		ProviderMessage: in.ProviderMessage,
		LatencyMs:       in.LatencyMs,
		ErrorKind:       in.ErrorKind,
		CorrelationID:   in.CorrelationID,
		Now:             ts(in.Now),
	})
}

// ListProviderAuditLogsByOperation 回某次開票作業的全部往返紀錄。
func (r *AuditRepository) ListProviderAuditLogsByOperation(ctx context.Context, operationID int64) ([]domain.ProviderAuditLogEntry, error) {
	rows, err := r.q(ctx).ListInvoiceProviderAuditLogsByOperation(ctx, &operationID)
	if err != nil {
		return nil, err
	}
	return toProviderAuditLogEntrys(rows), nil
}

// LatestProviderCorrelationID 回這位 issuer 最近一次有 correlation_id 的往返（#3031）。
//
// pgx.ErrNoRows ⇒ 空字串而不是 error：這位賣家還沒打過 AMEGO 是完全正常的狀態
// （剛設定憑證、還沒驗證），把它當錯誤會讓 S05 整頁打不開。
func (r *AuditRepository) LatestProviderCorrelationID(ctx context.Context, issuerID int64) (string, error) {
	v, err := r.q(ctx).GetLatestProviderAuditCorrelationByIssuer(ctx, issuerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	if v == nil {
		return "", nil
	}
	return *v, nil
}

// AppendAdminAuditLog 記一次 ADM 操作，回新列的 id。
func (r *AuditRepository) AppendAdminAuditLog(ctx context.Context, in domain.NewAdminAuditLogEntry) (int64, error) {
	return r.q(ctx).InsertInvoiceAdminAuditLog(ctx, gen.InsertInvoiceAdminAuditLogParams{
		ActorAccountID:         in.ActorAccountID,
		Action:                 in.Action,
		ResourceType:           in.ResourceType,
		ResourceID:             in.ResourceID,
		BeforeState:            in.BeforeState,
		AfterState:             in.AfterState,
		ReasonCode:             in.ReasonCode,
		ReasonDetail:           in.ReasonDetail,
		ApprovalActorAccountID: in.ApprovalActorAccountID,
		TraceID:                in.TraceID,
		RequestID:              in.RequestID,
		CorrelationID:          in.CorrelationID,
		ClientIp:               in.ClientIP,
		UserAgent:              in.UserAgent,
		Now:                    ts(in.Now),
	})
}

// ListAdminAuditLogsByResource 回某個資源上的 ADM 操作紀錄。
func (r *AuditRepository) ListAdminAuditLogsByResource(ctx context.Context, resourceType string, resourceID int64, limit int32) ([]domain.AdminAuditLogEntry, error) {
	rows, err := r.q(ctx).ListInvoiceAdminAuditLogsByResource(ctx, gen.ListInvoiceAdminAuditLogsByResourceParams{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		RowLimit:     limit,
	})
	if err != nil {
		return nil, err
	}
	return toAdminAuditLogEntrys(rows), nil
}

// BusinessDayRepository 是行事曆鏡像（§5.1.14），每日整段 DELETE+INSERT。
//
// 交易由呼叫端用 TxManager.WithTx 包住 DeleteAll ＋ N 次 Insert——本層不自己 Begin。
// 沒有這張表，invoice_complaints.sla_due_at（NOT NULL）算不出來，K01 整條申訴 SLA 停擺。
type BusinessDayRepository struct {
	baseRepo
}

// NewBusinessDayRepository 建立 repository。
func NewBusinessDayRepository(pool *pgxpool.Pool) *BusinessDayRepository {
	return &BusinessDayRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.BusinessDayRepository = (*BusinessDayRepository)(nil)

// DeleteAll 清空鏡像。
//
// 🔴 只能在與後續 Insert 同一個交易內呼叫：分開跑會出現一段「鏡像是空的」的窗口，
// 而那段時間算出來的工作天全是錯的（未涵蓋的日期一律視為假日）。
func (r *BusinessDayRepository) DeleteAll(ctx context.Context) (int64, error) {
	return r.q(ctx).DeleteAllBusinessDays(ctx)
}

// Insert 寫一天假日。
func (r *BusinessDayRepository) Insert(ctx context.Context, holidayDate, syncedAt time.Time) error {
	return r.q(ctx).InsertBusinessDay(ctx, gen.InsertBusinessDayParams{
		HolidayDate: dateParam(holidayDate),
		SyncedAt:    ts(syncedAt),
	})
}

// ListHolidays 回區間內的假日清單。
func (r *BusinessDayRepository) ListHolidays(ctx context.Context, from, to time.Time) ([]time.Time, error) {
	rows, err := r.q(ctx).ListBusinessDayHolidays(ctx, gen.ListBusinessDayHolidaysParams{
		FromDate: dateParam(from),
		ToDate:   dateParam(to),
	})
	if err != nil {
		return nil, err
	}
	out := make([]time.Time, 0, len(rows))
	for _, row := range rows {
		if d := pgxutil.DateValue(row.HolidayDate); d != nil {
			out = append(out, *d)
		}
	}
	return out, nil
}

// SyncedAt 回鏡像最後一次同步的時間；鏡像為空時回 nil。
//
// 回 nil 而不是零值：呼叫端必須把它當「今天算不出工作天」處理，不得當成「沒有假日」——
// 後者會把 3 工作天 SLA 算成 3 自然日，誤判賣家違規。
func (r *BusinessDayRepository) SyncedAt(ctx context.Context) (*time.Time, error) {
	v, err := r.q(ctx).GetBusinessDayMirrorSyncedAt(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // 空表，不是錯誤
	}
	if err != nil {
		return nil, err
	}
	if !v.Valid {
		return nil, nil
	}
	t := v.Time
	return &t, nil
}
