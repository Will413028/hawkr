package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/go-pkg/pgxutil"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// AdjustmentRepository 讀 invoice_adjustments 與折讓明細。
type AdjustmentRepository struct {
	baseRepo
}

// NewAdjustmentRepository 建立 repository。
func NewAdjustmentRepository(pool *pgxpool.Pool) *AdjustmentRepository {
	return &AdjustmentRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.AdjustmentRepository = (*AdjustmentRepository)(nil)

// GetByID 查單筆調整（作廢或折讓）。
func (r *AdjustmentRepository) GetByID(ctx context.Context, id int64) (domain.Adjustment, error) {
	row, err := r.q(ctx).GetInvoiceAdjustmentByID(ctx, id)
	if err != nil {
		return domain.Adjustment{}, notFound(err, bizcode.InvoiceDocumentNotFound)
	}
	return toAdjustment(row), nil
}

// ListByDocument 回一張憑證上的全部調整。
func (r *AdjustmentRepository) ListByDocument(ctx context.Context, documentID int64) ([]domain.Adjustment, error) {
	rows, err := r.q(ctx).ListInvoiceAdjustmentsByDocument(ctx, documentID)
	if err != nil {
		return nil, err
	}
	return toAdjustments(rows), nil
}

// ListStaleBlocked 回卡在 BLOCKED 太久的調整，給 ADM 工作台與告警用（唯讀，不搶佔）。
func (r *AdjustmentRepository) ListStaleBlocked(ctx context.Context, staleBefore time.Time, limit int32) ([]domain.Adjustment, error) {
	rows, err := r.q(ctx).ListStaleBlockedAdjustments(ctx, gen.ListStaleBlockedAdjustmentsParams{
		StaleBefore: ts(staleBefore),
		RowLimit:    limit,
	})
	if err != nil {
		return nil, err
	}
	return toAdjustments(rows), nil
}

// ListLines 回一筆折讓的明細行。
//
// TaxCategory 是 smallint，值域只有 1/2/3（chk_invadjline_tax_category）；讀回來要走
// domain.TaxCategoryFromDB，拿 Code 去 FromValue 必然 ok=false（§C.10 ②）。
func (r *AdjustmentRepository) ListLines(ctx context.Context, adjustmentID int64) ([]domain.AdjustmentLine, error) {
	rows, err := r.q(ctx).ListInvoiceAdjustmentLines(ctx, adjustmentID)
	if err != nil {
		return nil, err
	}
	return toAdjustmentLines(rows)
}

// CorrectionRequestRepository 讀 invoice_correction_requests。
type CorrectionRequestRepository struct {
	baseRepo
}

// NewCorrectionRequestRepository 建立 repository。
func NewCorrectionRequestRepository(pool *pgxpool.Pool) *CorrectionRequestRepository {
	return &CorrectionRequestRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.CorrectionRequestRepository = (*CorrectionRequestRepository)(nil)

// Insert 建一筆調整（G01／G02／G03 共用）。
//
// 🔴 SQL 的 ON CONFLICT DO NOTHING **不指名 target**：本表有三個 partial unique，
// 指名其一會讓另兩個衝突時 23505 並使整個交易 aborted（25P02）。
//
// created=false ⇒ 這筆調整已經建過（唯一鍵擋下），**呼叫端不得再建 operation**——
// 重複建會送出第二次作廢，而作廢不可逆。
//
// ⚠️ VOID 的六個 ALLOWANCE 專用欄一律傳 nil：chk_invadj_allowance_shape 是雙向的，
// 傳錯組合當場 23514（不是回錯誤碼，是 constraint violation）。
func (r *AdjustmentRepository) Insert(ctx context.Context, in domain.NewAdjustment) (domain.Adjustment, bool, error) {
	params := gen.InsertInvoiceAdjustmentParams{
		ID:                  in.ID,
		DocumentID:          in.DocumentID,
		AdjustmentType:      in.AdjustmentType,
		Status:              in.Status,
		TriggerKind:         in.TriggerKind,
		RefundNo:            in.RefundNo,
		CorrectionRequestID: in.CorrectionRequestID,
		AllowanceNumber:     in.AllowanceNumber,
		NetAmountExTax:      in.NetAmountExTax,
		TaxAmount:           in.TaxAmount,
		GrossAmount:         in.GrossAmount,
		VoidReasonCode:      in.VoidReasonCode,
		VoidReasonDetail:    in.VoidReasonDetail,
		Now:                 ts(in.Now),
	}
	if in.AllowanceDateTaipei != nil {
		params.AllowanceDateTaipei = dateParam(*in.AllowanceDateTaipei)
	}

	row, err := r.q(ctx).InsertInvoiceAdjustment(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Adjustment{}, false, nil
	}
	if err != nil {
		return domain.Adjustment{}, false, err
	}
	return toAdjustment(row), true, nil
}

// GetByID 查單筆更正請求。
func (r *CorrectionRequestRepository) GetByID(ctx context.Context, id int64) (domain.CorrectionRequest, error) {
	row, err := r.q(ctx).GetInvoiceCorrectionRequestByID(ctx, id)
	if err != nil {
		return domain.CorrectionRequest{}, notFound(err, bizcode.InvoiceCorrectionNotAllowed)
	}
	return toCorrectionRequest(row), nil
}

// GetOpenByDocument 回這張憑證上進行中的更正請求；查無列回 9307。
//
// 呼叫端要用它擋「同一張憑證同時開兩個更正請求」時，**不得**寫成「先查再寫」——
// 那在併發下擋不住，唯一防線是 DB 的部分唯一索引。
func (r *CorrectionRequestRepository) GetOpenByDocument(ctx context.Context, documentID int64) (domain.CorrectionRequest, error) {
	row, err := r.q(ctx).GetOpenCorrectionRequestByDocument(ctx, documentID)
	if err != nil {
		return domain.CorrectionRequest{}, notFound(err, bizcode.InvoiceCorrectionNotAllowed)
	}
	return toCorrectionRequest(row), nil
}

// GetLatestByOrder 回這張**訂單**上最近一次修改請求（不分狀態）；查無列回 9307。
//
// 🔴 與 GetOpenByDocument 分成兩支：那一支只認 REQUESTED／VOID_REISSUE_IN_PROGRESS
// ⇒ 被婉拒的那一筆查不到，而 B04 最需要顯示的正是它（買家要看 reject_reason_code
// 才知道該改走折讓還是申訴）。SQL 側已明寫 ORDER BY r.id DESC LIMIT 1。
//
// 🔴 **範圍是訂單不是單一 document**（2026-09-05 修）：更正請求掛在被作廢的那張
// document 上，而作廢重開完成後訂單的最新 document 是重開的新那張、身上沒有任何
// correction 列 ⇒ 以「最新 document」查會讓每一次**成功**的作廢重開把
// latest_correction_id 打回空字串。詳見 SQL 檔內該支的紅字。
func (r *CorrectionRequestRepository) GetLatestByOrder(ctx context.Context, orderNo string, buyerID int64) (domain.CorrectionRequest, error) {
	row, err := r.q(ctx).GetLatestCorrectionRequestByOrder(ctx, gen.GetLatestCorrectionRequestByOrderParams{
		SourceRef: orderNo,
		BuyerID:   buyerID,
	})
	if err != nil {
		return domain.CorrectionRequest{}, notFound(err, bizcode.InvoiceCorrectionNotAllowed)
	}
	return toCorrectionRequest(row), nil
}

// Insert 是 R01（買家於 B06 送出）。
//
// 🔴 created=false ⇒ uq_invcorr_open_per_document 擋下：這張發票已有進行中的修改請求。
// 呼叫端應回 9351 並改讀 GetOpenByDocument，**不是重試**。
func (r *CorrectionRequestRepository) Insert(ctx context.Context, in domain.NewCorrectionRequest) (domain.CorrectionRequest, bool, error) {
	row, err := r.q(ctx).InsertInvoiceCorrectionRequest(ctx, gen.InsertInvoiceCorrectionRequestParams{
		ID:                      in.ID,
		DocumentID:              in.DocumentID,
		RequestedByBuyerID:      in.RequestedByBuyerID,
		ExpectedSnapshotVersion: in.ExpectedSnapshotVersion,
		BeforePayload:           in.BeforePayload,
		AfterPayload:            in.AfterPayload,
		Now:                     ts(in.Now),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.CorrectionRequest{}, false, nil
	}
	if err != nil {
		return domain.CorrectionRequest{}, false, err
	}
	return toCorrectionRequest(row), true, nil
}

// Reject 是 R02（預檢不過）與 R05（f0501 作廢被 provider 拒絕）的共同落點。
//
// 兩條邊共用一句 SQL：差別只在**發現的時點**（預檢時 vs 送出後 AMEGO 才說），
// 買家看到的答案是同一句、reject_reason_code 取自同一組 7 值（兩條邊各自只用其中一部分：
// ALREADY_REPORTED 只有 R02 寫、PROVIDER_STATE_CONFLICT 只有 R05 寫）。
//
// 回 false ＝ rows=0（狀態已不在 REQUESTED／VOID_REISSUE_IN_PROGRESS）。不是錯誤，
// 但呼叫端**不得當成成功**——那代表有別人先動了這一列。
func (r *CorrectionRequestRepository) Reject(ctx context.Context, in domain.RejectCorrection) (bool, error) {
	code := in.ReasonCode
	rows, err := r.q(ctx).RejectInvoiceCorrectionRequest(ctx, gen.RejectInvoiceCorrectionRequestParams{
		RejectReasonCode:   &code,
		RejectReasonDetail: in.ReasonDetail,
		Now:                ts(in.Now),
		ID:                 in.ID,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// MarkUpdatedBeforeIssue 是 R03 的落地（REQUESTED → UPDATED_BEFORE_ISSUE，終態）。
//
// 🔴 本句**不是** R03 的守衛。守衛是 operation 的 HELD CAS rows=1：搶不到就代表那筆
// 已經在打 AMEGO，R03 不成立、要改走 R04。順序反了會落一個「快照改了但發票已經用
// 舊內容送出去」的狀態，而那不會有任何東西報錯。
func (r *CorrectionRequestRepository) MarkUpdatedBeforeIssue(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceCorrectionUpdatedBeforeIssue(ctx,
		gen.MarkInvoiceCorrectionUpdatedBeforeIssueParams{Now: &now, ID: id})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// StartVoidReissue 是 R04（REQUESTED → VOID_REISSUE_IN_PROGRESS）。
//
// voidAdjustmentID 指向同一事務內建立的 VOID adjustment（fk_invcorr_void）。
// rows=0 ⇒ 狀態已離開 REQUESTED，不是錯誤但不得當成成功。
func (r *CorrectionRequestRepository) StartVoidReissue(ctx context.Context, id, voidAdjustmentID int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).StartInvoiceCorrectionVoidReissue(ctx, gen.StartInvoiceCorrectionVoidReissueParams{
		VoidAdjustmentID: &voidAdjustmentID,
		Now:              ts(now),
		ID:               id,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// ComplaintRepository 讀 invoice_complaints 與時間軸，並跑三支 SLA 集合式 UPDATE。
type ComplaintRepository struct {
	baseRepo
}

// NewComplaintRepository 建立 repository。
func NewComplaintRepository(pool *pgxpool.Pool) *ComplaintRepository {
	return &ComplaintRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.ComplaintRepository = (*ComplaintRepository)(nil)

// Create 建立一件申訴（K01）。
//
// ON CONFLICT DO NOTHING **不指名 target**：本表的 uq_invcomp_open_per_document 是
// partial UNIQUE(document_id)，指名它以外的任何欄位都會讓衝突變成 23505，而
// PostgreSQL 的交易一旦有一句失敗就整個 aborted（25P02）——之後那句「查既有案件」
// 的 SELECT 會炸成 500，而正確行為是回 9310 並把買家導向既有案件。
//
// inserted=false 代表這份 document 已有一件未結案的申訴。**回傳的 row 是零值**
// （DO NOTHING 不 RETURNING），呼叫端必須自己去查既有那一件。
func (r *ComplaintRepository) Create(ctx context.Context, in domain.NewComplaint) (domain.Complaint, bool, error) {
	row, err := r.q(ctx).InsertInvoiceComplaint(ctx, gen.InsertInvoiceComplaintParams{
		ID:                 in.ID,
		DocumentID:         in.DocumentID,
		IssuerID:           in.IssuerID,
		OrderNo:            snp(in.OrderNo),
		ComplainantBuyerID: in.ComplainantBuyerID,
		ReasonCode:         in.ReasonCode,
		Description:        snp(in.Description),
		SlaDueAt:           ts(in.SlaDueAt),
		Now:                ts(in.Now),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Complaint{}, false, nil
	}
	if err != nil {
		return domain.Complaint{}, false, err
	}
	return toComplaint(row), true, nil
}

// CountWithdrawnByDocument 數這份 document 已撤回的案件數（K01 守衛②）。
func (r *ComplaintRepository) CountWithdrawnByDocument(ctx context.Context, documentID int64) (int64, error) {
	return r.q(ctx).CountWithdrawnComplaintsByDocument(ctx, documentID)
}

// EscalateByBuyer 是 K07：SELLER_RESPONDED → ESCALATED_TO_ADM（端點 63）。
func (r *ComplaintRepository) EscalateByBuyer(ctx context.Context, id, buyerID, expectedVersion int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).EscalateInvoiceComplaintByBuyer(ctx, gen.EscalateInvoiceComplaintByBuyerParams{
		Now:                ts(now),
		ID:                 id,
		ComplainantBuyerID: buyerID,
		ExpectedVersion:    expectedVersion,
	})
	return rows == 1, err
}

// Withdraw 是 K11：OPEN／OVERDUE／SELLER_RESPONDED → WITHDRAWN（端點 64）。
func (r *ComplaintRepository) Withdraw(ctx context.Context, id, buyerID, expectedVersion int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).WithdrawInvoiceComplaint(ctx, gen.WithdrawInvoiceComplaintParams{
		Now:                ts(now),
		ID:                 id,
		ComplainantBuyerID: buyerID,
		ExpectedVersion:    expectedVersion,
	})
	return rows == 1, err
}

// Reopen 是 K13：RESOLVED → OPEN（端點 65）。
//
// reopen_count 與 reopen_allowed_after 的守衛在 SQL 的 WHERE 讀轉移前的舊值，
// 本層不重複判——重複判會多出一條「Go 說可以、SQL 說不行」的分歧路徑。
func (r *ComplaintRepository) Reopen(ctx context.Context, id, buyerID, expectedVersion int64, slaDueAt, now time.Time) (bool, error) {
	rows, err := r.q(ctx).ReopenInvoiceComplaint(ctx, gen.ReopenInvoiceComplaintParams{
		SlaDueAt:           ts(slaDueAt),
		Now:                ts(now),
		ID:                 id,
		ComplainantBuyerID: buyerID,
		ExpectedVersion:    expectedVersion,
	})
	return rows == 1, err
}

// AppendEvent 追加一列時間軸。
//
// attachment_object_keys 恆為空陣列：Phase 1 買家端不收附件（58A B07「移除
// AfterSalePhotoUploader」），而該欄是 NOT NULL DEFAULT '[]' —— 傳 nil 會讓
// pgx 送出 SQL NULL 而不是走 DEFAULT，當場 23502。
func (r *ComplaintRepository) AppendEvent(ctx context.Context, in domain.NewComplaintEvent) (domain.ComplaintEvent, error) {
	var actorID *int64
	if in.ActorAccountID != 0 {
		actorID = &in.ActorAccountID
	}
	row, err := r.q(ctx).InsertInvoiceComplaintEvent(ctx, gen.InsertInvoiceComplaintEventParams{
		ComplaintID:          in.ComplaintID,
		EventType:            in.EventType,
		Action:               snp(in.Action),
		ActorType:            in.ActorType,
		ActorAccountID:       actorID,
		Message:              snp(in.Message),
		AttachmentObjectKeys: json.RawMessage(`[]`),
		Now:                  ts(in.Now),
	})
	if err != nil {
		return domain.ComplaintEvent{}, err
	}
	return toComplaintEvent(row), nil
}

// GetByID 查單件申訴。
func (r *ComplaintRepository) GetByID(ctx context.Context, id int64) (domain.Complaint, error) {
	row, err := r.q(ctx).GetInvoiceComplaintByID(ctx, id)
	if err != nil {
		return domain.Complaint{}, notFound(err, bizcode.InvoiceComplaintNotFound)
	}
	return toComplaint(row), nil
}

// ListEvents 回申訴的時間軸。
// GetForBuyer 是買家面的授權讀取：complainant_buyer_id 進 SQL 的 WHERE（§8.1.4(b)）。
//
// 查不到與越權共用 notFound 這一行 ⇒ 同一個業務碼、同一段 message、同一條路徑，
// 兩者刻意不可分辨。
func (r *ComplaintRepository) GetForBuyer(ctx context.Context, id, buyerID int64) (domain.Complaint, error) {
	row, err := r.q(ctx).GetInvoiceComplaintForBuyer(ctx, gen.GetInvoiceComplaintForBuyerParams{
		ID:                 id,
		ComplainantBuyerID: buyerID,
	})
	if err != nil {
		return domain.Complaint{}, notFound(err, bizcode.InvoiceComplaintNotFound)
	}
	return toComplaint(row), nil
}

func (r *ComplaintRepository) ListEvents(ctx context.Context, complaintID int64) ([]domain.ComplaintEvent, error) {
	rows, err := r.q(ctx).ListInvoiceComplaintEvents(ctx, complaintID)
	if err != nil {
		return nil, err
	}
	return toComplaintEvents(rows), nil
}

// CountConfirmedViolationOrders 是違規窗口的**滑動窗即時計算**。
//
// 🔴 不得改成維護一個 violation_count 計數器欄（§0 關鍵參數速查 C1）：計數器一旦與
// 事實漂移就需要一支 reconciliation job，而那是《後端服務責任邊界規範》明令禁止寫進
// application code 的東西。
func (r *ComplaintRepository) CountConfirmedViolationOrders(ctx context.Context, issuerID int64, windowStart time.Time) (int64, error) {
	return r.q(ctx).CountConfirmedViolationOrders(ctx, gen.CountConfirmedViolationOrdersParams{
		IssuerID:    issuerID,
		WindowStart: tp(windowStart),
	})
}

// MarkOverdue 把逾 SLA 的申訴標成逾期，回被改到的列。
func (r *ComplaintRepository) MarkOverdue(ctx context.Context, now time.Time, limit int32) ([]domain.Complaint, error) {
	rows, err := r.q(ctx).MarkOverdueComplaints(ctx, gen.MarkOverdueComplaintsParams{
		Now:      tp(now),
		RowLimit: limit,
	})
	if err != nil {
		return nil, err
	}
	return toComplaints(rows), nil
}

// EscalateOverdue 把逾期太久的申訴升級給平台處理。
func (r *ComplaintRepository) EscalateOverdue(ctx context.Context, now, escalateBefore time.Time, limit int32) ([]domain.Complaint, error) {
	rows, err := r.q(ctx).EscalateOverdueComplaints(ctx, gen.EscalateOverdueComplaintsParams{
		Now:            ts(now),
		EscalateBefore: tp(escalateBefore),
		RowLimit:       limit,
	})
	if err != nil {
		return nil, err
	}
	return toComplaints(rows), nil
}

// ResolveExpiredBuyerReviews 把買家逾期未確認的申訴自動結案。
func (r *ComplaintRepository) ResolveExpiredBuyerReviews(ctx context.Context, now, reopenAllowedAfter time.Time, resolutionCode string, limit int32) ([]domain.Complaint, error) {
	rows, err := r.q(ctx).ResolveExpiredBuyerReviews(ctx, gen.ResolveExpiredBuyerReviewsParams{
		Now:                tp(now),
		ReopenAllowedAfter: tp(reopenAllowedAfter),
		ResolutionCode:     sp(resolutionCode),
		RowLimit:           limit,
	})
	if err != nil {
		return nil, err
	}
	return toComplaints(rows), nil
}

// DecideEscalated 是 K08 ESCALATED_TO_ADM → RESOLVED ＋ K09 → VIOLATION_CONFIRMED。
//
// 兩條邊 SET 形狀相同（都寫 resolved_at ＋ decided_by ＋ decided_at）⇒ 共用一句，
// 目標值由 nextStatus 傳入。chk_invcomp_resolved_pair 把兩個目標綁在同一條雙向等式裡，
// 少寫 resolved_at 當場 23514。
func (r *ComplaintRepository) DecideEscalated(ctx context.Context, in domain.DecideComplaint) (bool, error) {
	rows, err := r.q(ctx).DecideEscalatedInvoiceComplaint(ctx, gen.DecideEscalatedInvoiceComplaintParams{
		NextStatus:         in.NextStatus,
		Now:                tp(in.Now),
		ResolutionCode:     snp(in.ResolutionCode),
		ResolutionNote:     snp(in.ResolutionNote),
		ReopenAllowedAfter: in.ReopenAllowedAfter,
		DecidedByAccountID: &in.DecidedByAccountID,
		ID:                 in.ID,
		ExpectedVersion:    in.ExpectedVersion,
	})
	return rows > 0, err
}

// ReturnToSeller 是 K10 ESCALATED_TO_ADM → OPEN（ADM 要求賣家再次處理）。
//
// sla_due_at 由呼叫端以工作天曆重算後傳入：不重置的話這一列一進 OPEN 就落回逾期掃描
// 的候選集合，賣家連看都還沒看到就被再標一次 OVERDUE。
func (r *ComplaintRepository) ReturnToSeller(ctx context.Context, id, expectedVersion int64, slaDueAt, now time.Time) (bool, error) {
	rows, err := r.q(ctx).ReturnInvoiceComplaintToSeller(ctx, gen.ReturnInvoiceComplaintToSellerParams{
		SlaDueAt:        ts(slaDueAt),
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	return rows > 0, err
}

// MarkSellerResponded 是 K02：OPEN／OVERDUE → SELLER_RESPONDED（端點 36）。
//
// buyerReviewDueAt 由呼叫端算（now + 72h）。它是 K06「逾期視為接受 → RESOLVED」唯一的
// 觸發依據，也是 idx_invcomp_buyer_review_due 的鍵——少寫**不會報錯**，那一列只是從 K06
// 的候選集合裡消失，該案永遠不會自動結案。
//
// 🔴 storeID 與 expectedVersion 都在 SQL 的 WHERE（經 invoice_issuers 解析）：越權、
// 狀態不允許、版本被搶先三者共用 rows=0，呼叫端一律回 9336。
func (r *ComplaintRepository) MarkSellerResponded(ctx context.Context, id, storeID, expectedVersion int64, buyerReviewDueAt, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceComplaintSellerResponded(ctx, gen.MarkInvoiceComplaintSellerRespondedParams{
		BuyerReviewDueAt: tp(buyerReviewDueAt),
		Now:              ts(now),
		ID:               id,
		ExpectedVersion:  expectedVersion,
		StoreID:          storeID,
	})
	return rows == 1, err
}

// EscalateBySeller 是 K03：OPEN → ESCALATED_TO_ADM（端點 36 的 NEED_PLATFORM_HELP）。
//
// 🔴 allowed-from 只有 OPEN，**不含 OVERDUE**：與 K02 分成兩支正是為了這個差別。
// 合成一句 2 值 allowed-from ⇒ 逾期之後才請求平台協助會靜默成功而狀態沒變。
func (r *ComplaintRepository) EscalateBySeller(ctx context.Context, id, storeID, expectedVersion int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).EscalateInvoiceComplaintBySeller(ctx, gen.EscalateInvoiceComplaintBySellerParams{
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
		StoreID:         storeID,
	})
	return rows == 1, err
}

// MarkReissueFailed 是 R08（VOID_REISSUE_IN_PROGRESS → REISSUE_FAILED）。
//
// rows==0 代表那一列已經不在 VOID_REISSUE_IN_PROGRESS——**這裡當成 false 而非 error**，
// 由呼叫端決定。R08 的呼叫端會把它當失敗（它剛剛才確認過狀態），但同一個 adapter
// 在 ADM 重試路徑上會遇到合法的 rows==0。
func (r *CorrectionRequestRepository) MarkReissueFailed(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceCorrectionReissueFailed(ctx, gen.MarkInvoiceCorrectionReissueFailedParams{
		Now: ts(now),
		ID:  id,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// GetReissueFailedByDocument 找停在 REISSUE_FAILED 的修改請求。
//
// SQL 帶 ORDER BY id DESC LIMIT 1：同一張發票連續兩次作廢重開都失敗時會有多列，
// 取最新那一筆（id 是 TSID，遞增即時序）。查無回 ErrNotFound，與 GetOpenByDocument 同。
func (r *CorrectionRequestRepository) GetReissueFailedByDocument(ctx context.Context, documentID int64) (domain.CorrectionRequest, error) {
	row, err := r.q(ctx).GetReissueFailedCorrectionByDocument(ctx, documentID)
	if err != nil {
		return domain.CorrectionRequest{}, notFound(err, bizcode.InvoiceCorrectionNotAllowed)
	}
	return toCorrectionRequest(row), nil
}

// RetryReissue 是 R09（REISSUE_FAILED → VOID_REISSUE_IN_PROGRESS）。
//
// rows==0 是常態而非錯誤：端點 52 對**每一次**重試都會嘗試連動，而絕大多數重試
// 的 document 上根本沒有停在 REISSUE_FAILED 的請求。
func (r *CorrectionRequestRepository) RetryReissue(ctx context.Context, id, expectedVersion int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).RetryInvoiceCorrectionReissue(ctx, gen.RetryInvoiceCorrectionReissueParams{
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// Complete 是 R10（→ COMPLETED，寫 completed_at 與 reissued_document_id）。
//
// 守衛「原 document 已轉 SUPERSEDED」跨表 ⇒ 不在本句，呼叫端要在同一交易內先做
// MarkSuperseded。這裡只守 correction 自己的來源狀態。
func (r *CorrectionRequestRepository) Complete(ctx context.Context, id, reissuedDocumentID int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).CompleteInvoiceCorrectionRequest(ctx, gen.CompleteInvoiceCorrectionRequestParams{
		ReissuedDocumentID: &reissuedDocumentID,
		Now:                &now,
		ID:                 id,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// ---------------------------------------------------------------------------
// M7 執行軸的 adapter（§7.3.7 G01 明細／G08–G15）
//
// 🔴 這一層之前**整層不存在**：19 支 SQL 與 sqlc 生成物都在，但沒有任何 Go 方法把
// 它們包出來、domain 介面也只宣告了 5 個唯讀方法 ⇒ 折讓的執行面等於不存在
// （#3073 的稽查結論）。每一支都是 CAS，rows=0 一律回 false 讓呼叫端當衝突處理。
// ---------------------------------------------------------------------------

// InsertLines 建折讓明細（G01 的一部分，與 adjustment 同交易）。
func (r *AdjustmentRepository) InsertLines(
	ctx context.Context, adjustmentID int64, lines []domain.NewAdjustmentLine, now time.Time,
) error {
	q := r.q(ctx)
	for _, l := range lines {
		if _, err := q.InsertInvoiceAdjustmentLine(ctx, gen.InsertInvoiceAdjustmentLineParams{
			AdjustmentID:        adjustmentID,
			LineSeq:             l.LineSeq,
			SourceLineID:        l.SourceLineID,
			OriginalInvoiceNo:   l.OriginalInvoiceNo,
			OriginalInvoiceDate: pgxutil.DateParam(&l.OriginalInvoiceDate),
			OriginalDescription: l.OriginalDescription,
			Quantity:            l.Quantity,
			UnitPriceExTax:      l.UnitPriceExTax,
			NetAmountExTax:      l.NetAmountExTax,
			TaxAmount:           l.TaxAmount,
			TaxCategory:         l.TaxCategory,
			Now:                 ts(now),
		}); err != nil {
			return err
		}
	}
	return nil
}

// NextAllowanceNumber 取我方自產的折讓單號（§5.3.3）。
//
// SEQUENCE 的 nextval 不受交易回滾影響 —— 跳號可接受、重複不可（同 NextReceiptNo）。
func (r *AdjustmentRepository) NextAllowanceNumber(ctx context.Context) (string, error) {
	return r.q(ctx).NextAllowanceNumber(ctx)
}

// MarkSending 是 G08（PENDING → SENDING）。
func (r *AdjustmentRepository) MarkSending(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceAdjustmentSending(ctx, gen.MarkInvoiceAdjustmentSendingParams{
		ID: id, Now: ts(now),
	})
	return rows > 0, err
}

// MarkCompleted 是 G09／G13／G17（→ COMPLETED）。
func (r *AdjustmentRepository) MarkCompleted(ctx context.Context, in domain.MarkAdjustmentCompleted) (bool, error) {
	now := in.Now
	rows, err := r.q(ctx).MarkInvoiceAdjustmentCompleted(ctx, gen.MarkInvoiceAdjustmentCompletedParams{
		ID:                      in.ID,
		Now:                     &now,
		ProviderUploadStatus:    in.ProviderUploadStatus,
		ProviderUploadCheckedAt: in.ProviderUploadCheckedAt,
	})
	return rows > 0, err
}

// MarkFailedPermanent 是 G10（SENDING → FAILED_PERMANENT ＋ blocked_reason_code）。
//
// reasonCode 必填：chk_invadj_blocked_reason 要求 FAILED_PERMANENT ⇒ 該欄非空，
// 傳空字串會撞 23514 而不是一句看得懂的錯誤。
func (r *AdjustmentRepository) MarkFailedPermanent(
	ctx context.Context, id int64, reasonCode string, now time.Time,
) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceAdjustmentFailedPermanent(ctx, gen.MarkInvoiceAdjustmentFailedPermanentParams{
		ID: id, BlockedReasonCode: &reasonCode, Now: ts(now),
	})
	return rows > 0, err
}

// Reschedule 是 G11／G14（→ PENDING）。
//
// 退避表達在 operation 的 next_attempt_at 上，adjustment 只回到 PENDING；
// 折讓單號**不換**（§7.2.2 細節 5），否則同一筆退款會產生兩張折讓單。
func (r *AdjustmentRepository) Reschedule(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).RescheduleInvoiceAdjustment(ctx, gen.RescheduleInvoiceAdjustmentParams{
		ID: id, Now: ts(now),
	})
	return rows > 0, err
}

// MarkWaitingConfirmation 是 G12（SENDING → WAITING_CONFIRMATION）。
func (r *AdjustmentRepository) MarkWaitingConfirmation(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceAdjustmentWaitingConfirmation(ctx,
		gen.MarkInvoiceAdjustmentWaitingConfirmationParams{ID: id, Now: ts(now)})
	return rows > 0, err
}

// MarkFailedUnknown 是 G15（WAITING_CONFIRMATION → FAILED_UNKNOWN，逾 24h）。
func (r *AdjustmentRepository) MarkFailedUnknown(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceAdjustmentFailedUnknown(ctx,
		gen.MarkInvoiceAdjustmentFailedUnknownParams{ID: id, Now: ts(now)})
	return rows > 0, err
}

// ---------------------------------------------------------------------------
// 冪等反查（#3073）
//
// ⚠️ 這裡**刻意沒有** G04／G05／G06／G07 的 adapter（閘門釋放）：那四條邊的 SQL 早就
// 存在，但它們釋放的 `BLOCKED_*` 狀態在全 repo 零寫入者 —— 原因寫在
// `usecase/refund_adjustment.go` 的 `refundPlanRetryLater`（chk_invadj_allowance_shape
// 要求 ALLOWANCE 帶齊六欄，而 BLOCKED 的觸發條件正是「那六欄還算不出來」）。
// 加一組沒有呼叫端的 adapter 會讓下一個人以為閘門釋放已經接好了。
// ---------------------------------------------------------------------------

// GetByRefund 是 uq_invadj_allowance_refund 的反查（I-11）。
//
// 冪等一律走這一句，**不要**靠 INSERT 撞唯一鍵：PostgreSQL 上撞唯一鍵會讓整個
// 交易 aborted（25P02），之後每一句 SQL 都會被拒絕，連 catch 之後的 SELECT 也是。
// found=false 代表這筆退款還沒開過折讓。
func (r *AdjustmentRepository) GetByRefund(
	ctx context.Context, documentID int64, refundNo string,
) (domain.Adjustment, bool, error) {
	row, err := r.q(ctx).GetInvoiceAdjustmentByRefund(ctx, gen.GetInvoiceAdjustmentByRefundParams{
		DocumentID: documentID, RefundNo: &refundNo,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Adjustment{}, false, nil
		}
		return domain.Adjustment{}, false, err
	}
	return toAdjustment(row), true, nil
}

// CancelUnsentForIssuer 是 P14／P16 的第三個副作用（#3073）：憑證停用／撤銷之後，
// 該 issuer 底下**還沒送出去**的調整永遠送不出去了。
//
// 🔴 allowed-from 只取「還沒送出去」的四個（述詞在 SQL）。
// WAITING_CONFIRMATION／FAILED_UNKNOWN **不動**：折讓可能已經在 AMEGO 端生效，
// cancel 它會讓帳對不上；它們由 invoice_adjustment_stuck_rows 曝光給人處理。
func (r *AdjustmentRepository) CancelUnsentForIssuer(
	ctx context.Context, issuerID int64, cancelReasonCode string, now time.Time,
) (int64, error) {
	return r.q(ctx).CancelUnsentAdjustmentsForIssuer(ctx, gen.CancelUnsentAdjustmentsForIssuerParams{
		IssuerID: issuerID, CancelReasonCode: &cancelReasonCode, Now: ts(now),
	})
}

// AdminRetry 是 G16（FAILED_PERMANENT → PENDING）。
//
// 🔴 沒有它，ADM 對一個失敗的折讓 job 按端點 52 重試會**必然再失敗**：operation 回到
// READY，而 adjustment 還停在 FAILED_PERMANENT ⇒ 下一輪 claim 的 MarkSending
// （allowed-from = PENDING）rows=0 ⇒ 整條軸再死一次，而 ADM 只會看到它又失敗了。
func (r *AdjustmentRepository) AdminRetry(
	ctx context.Context, id int64, expectedVersion int64, now time.Time,
) (bool, error) {
	rows, err := r.q(ctx).AdminRetryInvoiceAdjustment(ctx, gen.AdminRetryInvoiceAdjustmentParams{
		ID: id, ExpectedVersion: expectedVersion, Now: ts(now),
	})
	return rows > 0, err
}

// AdminCancel 是 G19（PENDING／兩個 BLOCKED_* → CANCELLED）。
//
// allowed-from **不含 SENDING**：租約持有中代表可能正在打 AMEGO，那時取消會讓
// 「我方帳上已取消、對方端已生效」。rows=0 在這裡是**正確結果**不是錯誤。
func (r *AdjustmentRepository) AdminCancel(
	ctx context.Context, id int64, expectedVersion int64, cancelReasonCode string, now time.Time,
) (bool, error) {
	rows, err := r.q(ctx).AdminCancelInvoiceAdjustment(ctx, gen.AdminCancelInvoiceAdjustmentParams{
		ID: id, ExpectedVersion: expectedVersion, CancelReasonCode: &cancelReasonCode, Now: ts(now),
	})
	return rows > 0, err
}
