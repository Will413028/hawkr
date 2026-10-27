package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"github.com/Will413028/hawkr/invoice-service/internal/repository/postgres/gen"
)

// OperationRepository 是 invoice_operations 的 thin adapter，同時服務兩支 worker：
//
//	INV.op（狀態集合 READY / RETRY_SCHEDULED / SENDING，租約 120s，寫入類副作用）
//	INV.rc（狀態集合 WAITING_CONFIRMATION / FAILED_UNKNOWN / CANCEL_REQUESTED /
//	        CANCEL_BLOCKED_UNKNOWN / PROVIDER_STATUS_RECHECK，租約 60s，唯讀對帳）
//
// 兩組候選述詞落在同一個租約欄（next_attempt_at），所以「狀態集合不相交」是**正確性
// 條件**而不是風格問題：交集非空 ⇒ 60s 那支會在 120s 那支還在打 f0401 的途中合法搶走
// 同一列 ⇒ 重複開票。述詞本身在 db/queries/invoice_operations.sql，本層只轉參數。
type OperationRepository struct {
	baseRepo
}

// NewOperationRepository 建立 repository。
func NewOperationRepository(pool *pgxpool.Pool) *OperationRepository {
	return &OperationRepository{baseRepo: baseRepo{pool: pool}}
}

var _ domain.OperationRepository = (*OperationRepository)(nil)

// FetchDue 是 INV.op 的候選（依 statutory_deadline_at 排序，NULLS LAST）。
func (r *OperationRepository) FetchDue(ctx context.Context, now time.Time, limit int32) ([]domain.Operation, error) {
	rows, err := r.q(ctx).FetchDueInvoiceOperations(ctx, gen.FetchDueInvoiceOperationsParams{
		Now:      ts(now),
		RowLimit: limit,
	})
	if err != nil {
		return nil, err
	}
	return toOperations(rows), nil
}

// Claim 搶佔一列給 INV.op。leaseUntil 由呼叫端逐筆現算（見 ClaimHealthCheck 的說明）。
func (r *OperationRepository) Claim(ctx context.Context, id int64, now, leaseUntil time.Time) (bool, error) {
	rows, err := r.q(ctx).ClaimInvoiceOperation(ctx, gen.ClaimInvoiceOperationParams{
		LeaseUntil:      ts(leaseUntil),
		Now:             ts(now),
		ID:              id,
		ClaimableBefore: ts(now),
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// MarkDone 把 operation 推到 SUCCEEDED。
//
// 回 false 代表守衛述詞不成立（SQL 內帶 AND status = 'SENDING'）——那不是「已經完成
// 過」就是「這一列已被別人改走」，呼叫端不得當成功繼續往下發事件。
func (r *OperationRepository) MarkDone(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceOperationDone(ctx, gen.MarkInvoiceOperationDoneParams{
		Now: tp(now),
		ID:  id,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// MarkFailed 把 operation 推到 FAILED_PERMANENT。
func (r *OperationRepository) MarkFailed(ctx context.Context, id int64, lastError string, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceOperationFailed(ctx, gen.MarkInvoiceOperationFailedParams{
		LastError: sp(lastError),
		Now:       tp(now),
		ID:        id,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// UpdateAttempt 記一次失敗並重排下次嘗試（status 回 RETRY_SCHEDULED）。
//
// ⚠️ 這支**只給 INV.op 用**。INV.rc 若拿它記「這輪查完仍未確定」，那一列會被寫成
// RETRY_SCHEDULED ⇒ 下一輪被 INV.op 撈走並真的重送一次 f0401 ⇒ 重複開票（§4.6 W-03）。
// INV.rc 的對應落點是 UpdateRecheckAttempt。
func (r *OperationRepository) UpdateAttempt(ctx context.Context, id int64, attemptCount int32, nextAttemptAt time.Time, lastError *string, now time.Time) error {
	return r.q(ctx).UpdateInvoiceOperationAttempt(ctx, gen.UpdateInvoiceOperationAttemptParams{
		AttemptCount:  attemptCount,
		NextAttemptAt: ts(nextAttemptAt),
		LastError:     lastError,
		Now:           ts(now),
		ID:            id,
	})
}

// FetchPendingRechecks 是 INV.rc 在同一張表上的第二組候選。
func (r *OperationRepository) FetchPendingRechecks(ctx context.Context, now time.Time, limit int32) ([]domain.Operation, error) {
	rows, err := r.q(ctx).FetchPendingInvoiceRechecks(ctx, gen.FetchPendingInvoiceRechecksParams{
		Now:      ts(now),
		RowLimit: limit,
	})
	if err != nil {
		return nil, err
	}
	return toOperations(rows), nil
}

// ClaimRecheck 搶佔一列給 INV.rc（租約 60s，與 INV.op 的 120s 不同且不得混用）。
func (r *OperationRepository) ClaimRecheck(ctx context.Context, id int64, now, leaseUntil time.Time) (bool, error) {
	rows, err := r.q(ctx).ClaimInvoiceRecheck(ctx, gen.ClaimInvoiceRecheckParams{
		LeaseUntil:      ts(leaseUntil),
		Now:             ts(now),
		ID:              id,
		ClaimableBefore: ts(now),
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// MarkRecheckResolved 是對帳**有確定結果**的落點（J23 SUCCEEDED／J24 CANCELLED_BY_ADMIN／
// J25 RETRY_SCHEDULED＝確認未開立、可安全補送）。
//
// 「查不到」是 J25 不是 FAILED_PERMANENT：判成永久失敗會讓一張補得回來的發票停在終態。
func (r *OperationRepository) MarkRecheckResolved(ctx context.Context, in domain.MarkRecheckResult) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceRecheckResolved(ctx, gen.MarkInvoiceRecheckResolvedParams{
		NextStatus:      in.NextStatus,
		NextAttemptAt:   ts(in.NextAttemptAt),
		ProviderCode:    in.ProviderCode,
		ProviderMessage: in.ProviderMessage,
		CompletedAt:     in.CompletedAt,
		// J24 專屬。SQL 那側是 COALESCE(narg, 原值) ⇒ J23／J25 傳 nil 不會抹掉
		// 端點 54 已寫好的 A4 取消證據。
		CancelEvidenceKind: in.CancelEvidenceKind,
		CancelConfirmedAt:  in.CancelConfirmedAt,
		Now:                ts(in.Now),
		ID:                 in.ID,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// MarkRecheckUnresolved 是對帳**仍不確定且要換狀態**的落點
// （J26 CANCEL_BLOCKED_UNKNOWN／J17 FAILED_UNKNOWN，後者要 unknown_since 超過 24h）。
func (r *OperationRepository) MarkRecheckUnresolved(ctx context.Context, in domain.MarkRecheckResult) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceRecheckUnresolved(ctx, gen.MarkInvoiceRecheckUnresolvedParams{
		NextStatus:      in.NextStatus,
		NextAttemptAt:   ts(in.NextAttemptAt),
		ProviderCode:    in.ProviderCode,
		ProviderMessage: in.ProviderMessage,
		LastError:       in.LastError,
		Now:             ts(in.Now),
		ID:              in.ID,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// UpdateRecheckAttempt 是「仍不確定且留在重查迴圈」的落點（J31 回 WAITING_CONFIRMATION／
// 自轉移到 PROVIDER_STATUS_RECHECK），並依 unknown_since 重排 next_attempt_at。
//
// INV.rc 沒有「次數耗盡」這條路：「未知」不會因為查詢次數用完就變成「已知」，停止重查的
// 唯一後果是把一張**可能已經開立**的發票永遠留在未知狀態。
func (r *OperationRepository) UpdateRecheckAttempt(ctx context.Context, in domain.UpdateRecheckAttempt) (bool, error) {
	rows, err := r.q(ctx).UpdateInvoiceRecheckAttempt(ctx, gen.UpdateInvoiceRecheckAttemptParams{
		NextStatus:      in.NextStatus,
		NextAttemptAt:   ts(in.NextAttemptAt),
		RecheckListPage: in.RecheckListPage,
		ProviderCode:    in.ProviderCode,
		ProviderMessage: in.ProviderMessage,
		Now:             ts(in.Now),
		ID:              in.ID,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// ReleaseExpiredHolds 是 INV.op RunOnce 的前置步驟①：集合式、無外部副作用的單句 UPDATE
// ⇒ 不需要 claim。
func (r *OperationRepository) ReleaseExpiredHolds(ctx context.Context, now time.Time) (int64, error) {
	return r.q(ctx).ReleaseExpiredInvoiceOperationHolds(ctx, ts(now))
}

// Hold 是 J08（READY／RETRY_SCHEDULED → HELD），也是 §7.3.5 R03 的守衛本身。
//
// 🔴 rows=0 ⇒ **R03 不成立，改走 R04**。述詞刻意不含 SENDING：已經在打 AMEGO 的
// 那一筆攔不下來。把它當成暫時性失敗去重試，會在對方已經開出發票之後才改快照——
// 而那張發票的內容是舊的，不會有任何東西報錯。
func (r *OperationRepository) Hold(ctx context.Context, id int64, now, heldUntil time.Time) (bool, error) {
	rows, err := r.q(ctx).HoldInvoiceOperation(ctx, gen.HoldInvoiceOperationParams{
		HeldUntil: &heldUntil,
		Now:       ts(now),
		ID:        id,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// ReleaseHold 是 R03 的收尾（HELD → READY，同一事務內，不等租約到期）。
//
// 🔴 少了它不會報錯、不會有測試紅，只是那張發票有 60 秒開不出去——
// HELD 不在兩支 worker 的候選集合裡。
func (r *OperationRepository) ReleaseHold(ctx context.Context, id int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).ReleaseInvoiceOperationHold(ctx, gen.ReleaseInvoiceOperationHoldParams{
		Now: ts(now),
		ID:  id,
	})
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// PromoteDueTriggers 是前置步驟②（AWAITING_TRIGGER → READY，J05）。
// 少了它，那條狀態邊沒有任何元件會執行——不是編譯錯、不是測試紅，是那批列永遠不動。
func (r *OperationRepository) PromoteDueTriggers(ctx context.Context, now time.Time) (int64, error) {
	return r.q(ctx).PromoteDueInvoiceOperationTriggers(ctx, ts(now))
}

// PromoteSellerActionRequired 是 J33（SELLER_ACTION_REQUIRED → READY，字軌餘量恢復）。
//
// 🔴 在 2026-09-04 之前**這支 adapter 不存在**，於是 J33 全 repo 零執行者
// （#3089 A 組第 3 項）。CodeMeta 對該狀態的 description 逐字寫著「由 INV.trk 的
// RunOnce 前置步驟 promote 回 READY」——而那個步驟從來沒有被寫出來。
// 加上 J34 的讀端閘門也壞著（同票第 8 項），那個狀態的**兩個出口都不通**。
//
// issuer_id 必帶：字軌是 per-issuer 的資源，少了它一個賣家配到字軌會把
// 全平台卡住的票一起放行（SQL 檔頭已寫過同一件事）。
func (r *OperationRepository) PromoteSellerActionRequired(ctx context.Context, issuerID int64, now time.Time) (int64, error) {
	return r.q(ctx).PromoteSellerActionRequiredOperations(ctx,
		gen.PromoteSellerActionRequiredOperationsParams{Now: ts(now), IssuerID: issuerID})
}

// ---------------------------------------------------------------------------
// M2 的業務轉移（J01–J04、J13、J14、J32）——排程以外的人走的邊
// ---------------------------------------------------------------------------

// Create 建立一列 operation。inserted=false 有兩種意義（見 domain 介面註解），
// 呼叫端必須用 GetByKey 分辨，不是猜。
//
// ON CONFLICT DO NOTHING **不指名 target**：本表有兩個唯一索引（uq_invop_operation_key
// 與 partial 的 uq_invop_open_adjustment_per_document ＝ I5 序列閘門）。
func (r *OperationRepository) Create(ctx context.Context, in domain.NewOperation) (domain.Operation, bool, error) {
	row, err := r.q(ctx).InsertInvoiceOperation(ctx, gen.InsertInvoiceOperationParams{
		ID:                  in.ID,
		DocumentID:          in.DocumentID,
		AdjustmentID:        inp(in.AdjustmentID),
		OperationType:       in.OperationType,
		OperationKey:        in.OperationKey,
		ProviderRequestNo:   in.ProviderRequestNo,
		RequestSnapshotHash: in.RequestSnapshotHash,
		MaxAttempts:         in.MaxAttempts,
		NextAttemptAt:       ts(in.NextAttemptAt),
		StatutoryDeadlineAt: in.StatutoryDeadlineAt,
		Now:                 ts(in.Now),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Operation{}, false, nil
	}
	if err != nil {
		return domain.Operation{}, false, err
	}
	return toOperation(row), true, nil
}

// GetByID 讀單筆 operation。
func (r *OperationRepository) GetByID(ctx context.Context, id int64) (domain.Operation, error) {
	row, err := r.q(ctx).GetInvoiceOperationByID(ctx, id)
	if err != nil {
		return domain.Operation{}, notFound(err, bizcode.InvoiceJobNotFound)
	}
	return toOperation(row), nil
}

// GetByKey 依冪等鍵讀。Create 回 inserted=false 之後用它分辨「同一把 key 重送」
// 與「序列閘門被別的調整佔用」：查得到就是前者。
func (r *OperationRepository) GetByKey(ctx context.Context, operationKey string) (domain.Operation, error) {
	row, err := r.q(ctx).GetInvoiceOperationByKey(ctx, operationKey)
	if err != nil {
		return domain.Operation{}, notFound(err, bizcode.InvoiceJobNotFound)
	}
	return toOperation(row), nil
}

// Promote 是 J02／J03／J04：CREATED → READY／AWAITING_TRIGGER／NOT_APPLICABLE，
// **建列的同一事務內**判定。
func (r *OperationRepository) Promote(ctx context.Context, id int64, nextStatus string, nextAttemptAt, now time.Time) (bool, error) {
	rows, err := r.q(ctx).PromoteCreatedInvoiceOperation(ctx, gen.PromoteCreatedInvoiceOperationParams{
		NextStatus:    nextStatus,
		NextAttemptAt: ts(nextAttemptAt),
		Now:           ts(now),
		ID:            id,
	})
	return rows > 0, err
}

// MarkWaitingConfirmation 是 J13 SENDING → WAITING_CONFIRMATION。
//
// 轉出後這一列離開 INV.op 的候選集合、落進 INV.rc 的（兩組狀態集合互斥＝I-55）。
// ⚠️ **不發 MQ**：invoice.issue.pending_confirmation 不在 §6.3 的 P1–P12 內，
// 發到沒有 binding 的 routing key 會靜默消失、不進 DLQ、不報錯。
func (r *OperationRepository) MarkWaitingConfirmation(ctx context.Context, in domain.MarkOperationUnknown) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceOperationWaitingConfirmation(ctx, gen.MarkInvoiceOperationWaitingConfirmationParams{
		Now:             ts(in.Now),
		NextAttemptAt:   ts(in.NextAttemptAt),
		ProviderCode:    snp(in.ProviderCode),
		ProviderMessage: snp(in.ProviderMessage),
		LastError:       snp(in.LastError),
		ID:              in.ID,
	})
	return rows > 0, err
}

// MarkRetryExhausted 是 J14 RETRY_SCHEDULED → FAILED_RETRY_EXHAUSTED。
func (r *OperationRepository) MarkRetryExhausted(ctx context.Context, id int64, lastError string, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceOperationRetryExhausted(ctx, gen.MarkInvoiceOperationRetryExhaustedParams{
		LastError: snp(lastError),
		Now:       tp(now),
		ID:        id,
	})
	return rows > 0, err
}

// MarkSellerActionRequired 是 J32 SENDING → SELLER_ACTION_REQUIRED。
func (r *OperationRepository) MarkSellerActionRequired(ctx context.Context, in domain.MarkOperationProviderResult) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceOperationSellerActionRequired(ctx, gen.MarkInvoiceOperationSellerActionRequiredParams{
		ProviderCode:    snp(in.ProviderCode),
		ProviderMessage: snp(in.ProviderMessage),
		LastError:       snp(in.LastError),
		Now:             ts(in.Now),
		ID:              in.ID,
	})
	return rows > 0, err
}

// MarkFailedWithProviderResult 是 ④'（FAILED_PERMANENT ＋ 保存 provider 原話）。
//
// 🔴 與 MarkFailed 分開的理由寫在 SQL 檔頭：MarkFailed 的呼叫者
// （pkg/outbox，重試預算用完）手上沒有 provider 回應，把兩個空值寫進去會洗掉
// 先前某一次真的有 code 的記錄。
func (r *OperationRepository) MarkFailedWithProviderResult(ctx context.Context, in domain.MarkOperationProviderResult) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceOperationFailedWithProviderResult(ctx,
		gen.MarkInvoiceOperationFailedWithProviderResultParams{
			LastError:       snp(in.LastError),
			ProviderCode:    snp(in.ProviderCode),
			ProviderMessage: snp(in.ProviderMessage),
			Now:             tp(in.Now),
			ID:              in.ID,
		})
	return rows > 0, err
}

// ConfirmCancel 是 J28（CANCEL_BLOCKED_UNKNOWN → CANCELLED_BY_ADMIN）。
//
// 🔴 這支 adapter 在 2026-09-04 之前**不存在** —— SQL 與 sqlc 生成物都在，
// 而那條邊零呼叫端（#3089 A 組第 2 項）。對照組：緊鄰的 J29
// （AllowManualProcess）一直都有 adapter。
func (r *OperationRepository) ConfirmCancel(ctx context.Context, in domain.ConfirmOperationCancel) (bool, error) {
	rows, err := r.q(ctx).ConfirmInvoiceOperationCancel(ctx, gen.ConfirmInvoiceOperationCancelParams{
		CancelConfirmedBy:       &in.ConfirmedBy,
		Now:                     tp(in.Now),
		CancelEvidenceKind:      &in.EvidenceKind,
		CancelEvidenceObjectKey: in.EvidenceObjectKey,
		ID:                      in.ID,
		ExpectedVersion:         in.ExpectedVersion,
	})
	return rows > 0, err
}

// ListByDocument 回一份文件底下的全部 operation（created_at DESC，吃 idx_invop_document）。
//
// 沒有分頁：I5 序列閘門讓同一份文件上的 operation 數量有實質上界（每次 issue／void／
// allowance 各一列），ADM 詳情頁要的是全部而不是一頁。
func (r *OperationRepository) ListByDocument(ctx context.Context, documentID int64) ([]domain.Operation, error) {
	rows, err := r.q(ctx).ListInvoiceOperationsByDocument(ctx, documentID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		out = append(out, toOperation(row))
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// ADM 寫入面（§6.1.14 端點 52／53／56）
// ---------------------------------------------------------------------------
//
// 三支都回 CAS 的 rows>0。false ＝ 守衛不成立（allowed-from 不符或版本被搶先），
// 呼叫端必須回明確業務碼——把 false 當成功往下走就是靜默失敗：ADM 按下去沒反應、
// 狀態沒動，而且不會有任何錯誤訊號。

// AdminRetryAfterFix 是 J18 FAILED_PERMANENT → READY（ADM 修正資料後重試）。
//
// attempt_count **不歸零**（與 AdminResetAttempts 的差異）：資料錯了才走這條，
// 重試預算不該因為改了資料而重新開始。
func (r *OperationRepository) AdminRetryAfterFix(ctx context.Context, id, expectedVersion int64, requestSnapshotHash string, now time.Time) (bool, error) {
	rows, err := r.q(ctx).AdminRetryInvoiceOperationAfterFix(ctx, gen.AdminRetryInvoiceOperationAfterFixParams{
		RequestSnapshotHash: requestSnapshotHash,
		Now:                 ts(now),
		ID:                  id,
		ExpectedVersion:     expectedVersion,
	})
	return rows > 0, err
}

// AdminResetAttempts 是 J19 FAILED_RETRY_EXHAUSTED → READY ＋ J34 SELLER_ACTION_REQUIRED → READY。
//
// 兩條邊共用一句是 §7.3.2 J34 的逐字要求（不要求新的 snapshot_version）；
// attempt_count = 0 是兩者共同的副作用。
func (r *OperationRepository) AdminResetAttempts(ctx context.Context, id, expectedVersion int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).AdminResetInvoiceOperationAttempts(ctx, gen.AdminResetInvoiceOperationAttemptsParams{
		Now:             ts(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	return rows > 0, err
}

// RequestCancel 是 J21（五個來源狀態 → CANCEL_REQUESTED，端點 53）。
//
// SQL 在同一句把 next_attempt_at 推成 now ⇒ 這一列立刻落進 INV.rc 的候選集合去做
// 強制最終查詢。少了那一件，從 RETRY_SCHEDULED 取消的那筆會沿用舊退避、最壞延遲
// 一整個週期才被接手。
func (r *OperationRepository) RequestCancel(ctx context.Context, in domain.RequestOperationCancel) (bool, error) {
	rows, err := r.q(ctx).RequestInvoiceOperationCancel(ctx, gen.RequestInvoiceOperationCancelParams{
		CancelRequestedBy: &in.ActorAccountID,
		Now:               tp(in.Now),
		CancelReasonCode:  &in.ReasonCode,
		CancelReason:      snp(in.ReasonDetail),
		ID:                in.ID,
		ExpectedVersion:   in.ExpectedVersion,
	})
	return rows > 0, err
}

// MarkResolvedExternally 是 J30 MANUAL_PROCESS_ALLOWED → RESOLVED_EXTERNALLY（終態）。
//
// 端點 56（ADM 代錄）與端點 62（賣家自報）共用這一支：§6.1.14 逐字「兩支都要走同一個
// transition helper，不得各寫一份 CAS」。
func (r *OperationRepository) MarkResolvedExternally(ctx context.Context, id, expectedVersion int64, now time.Time) (bool, error) {
	rows, err := r.q(ctx).MarkInvoiceOperationResolvedExternally(ctx, gen.MarkInvoiceOperationResolvedExternallyParams{
		Now:             tp(now),
		ID:              id,
		ExpectedVersion: expectedVersion,
	})
	return rows > 0, err
}

// AllowManualProcess 是 J29 CANCELLED_BY_ADMIN → MANUAL_PROCESS_ALLOWED
// （端點 55 第二位管理員覆核解鎖）。
//
// 🔴 這是**第二道**雙人守衛，與 invoice_manual_unlocks 那一句 CAS 守的不是同一件事：
// 本句守「覆核的人不得是提出取消的人」（cancel_requested_by <> actor），那一句守
// 「覆核的人不得是發起解鎖的人」。兩位發起人未必是同一位 ADM ⇒ 兩道都要，而且兩道
// 都在 SQL 裡——把任何一道搬回應用層，兩個分頁同時送出時就擋不住。
func (r *OperationRepository) AllowManualProcess(ctx context.Context, id, expectedVersion, actorAccountID int64, now time.Time) (bool, error) {
	actor := actorAccountID
	rows, err := r.q(ctx).AllowInvoiceOperationManualProcess(ctx, gen.AllowInvoiceOperationManualProcessParams{
		Now:             ts(now),
		ID:              id,
		ActorAccountID:  &actor,
		ExpectedVersion: expectedVersion,
	})
	return rows > 0, err
}

// FailOpenForIssuer 是 M4 P14（端點 45 停用）／P16（端點 67 撤銷）的集合式副作用：
// 該 issuer **全部未終態** operation 轉 FAILED_PERMANENT。
//
// 🔴 這一批在 P15（ADM 恢復連線）之後**不自動復活**：它們是 M2 的非終態，由 ADM 在 A06
// 逐筆按端點 52 安全重試（J18／J19）。自動復活會在憑證其實仍不對時再打一輪 AMEGO，
// 而那一輪的失敗會被算進 attempt_count。
//
// 集合式、無外部副作用（outbox 是同一交易內的列，不是網路呼叫）⇒ 依《多副本排程搶佔規範》
// 例外 A 不需要 claim：推出候選集合的寫入就是狀態轉移本身，第二個副本 rows=0。
func (r *OperationRepository) FailOpenForIssuer(ctx context.Context, in domain.FailOpenOperationsForIssuer) (int64, error) {
	return r.q(ctx).FailOpenInvoiceOperationsForIssuer(ctx, gen.FailOpenInvoiceOperationsForIssuerParams{
		CancelReasonCode: snp(in.CancelReasonCode),
		LastError:        snp(in.LastError),
		Now:              tp(in.Now),
		IssuerID:         in.IssuerID,
	})
}

// CountOpenForIssuer 是端點 45／67 回應裡的 pendingOrderCount。
//
// 🔴 必須與 FailOpenForIssuer 在**同一交易**內呼叫（§7.4 C-8）：分開查有 TOCTOU 窗口
// （dialog 開啟到按下之間又付款了一筆），ADM 看到的數字就與實際被打掉的數量不一致。
// 述詞與 FailOpenForIssuer 的 allowed-from 逐字相同，改一邊就要改兩邊。
func (r *OperationRepository) CountOpenForIssuer(ctx context.Context, issuerID int64) (int64, error) {
	return r.q(ctx).CountOpenInvoiceOperationsForIssuer(ctx, issuerID)
}

// CountCrossPeriodPending 是 M20 的資料源（§8.2.3）：發票日期屬已結束期別、
// 且仍有非終態 VOID／ALLOWANCE／VOID_ALLOWANCE operation 的筆數。
//
// 唯讀，不改任何狀態（§4.5.10 明訂它是唯讀檢查）。期別以台北時間分期，判定寫在
// SQL 內——放進 PromQL 會用 UTC 算，每兩個月的邊界日剛好錯 8 小時。
func (r *OperationRepository) CountCrossPeriodPending(ctx context.Context, now time.Time) (int64, error) {
	return r.q(ctx).CountCrossPeriodPending(ctx, ts(now))
}

// CancelBeforeIssue 是 J06 → CANCELLED_BEFORE_ISSUE。
//
// allowed-from 自 2026-09-01 起含 CREATED（#2909 作廢重開）：在那之前一筆
// CREATED 的 operation **沒有任何路徑取消得了它**。
func (r *OperationRepository) CancelBeforeIssue(ctx context.Context, id int64, reasonCode, reason string, now time.Time) (bool, error) {
	rows, err := r.q(ctx).CancelInvoiceOperationBeforeIssue(ctx, gen.CancelInvoiceOperationBeforeIssueParams{
		CancelReasonCode: &reasonCode,
		CancelReason:     nilIfBlank(reason),
		Now:              &now,
		ID:               id,
	})
	return rows > 0, err
}
