package usecase

import (
	"context"
	"errors"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
)


const (
	unlockEvidenceMaxRunes = 1000

	unlockReasonRequested = "ADMIN_MANUAL_UNLOCK_REQUESTED"
	unlockReasonApproved  = "ADMIN_MANUAL_UNLOCK_APPROVED"
	unlockReasonRejected  = "ADMIN_MANUAL_UNLOCK_REJECTED"

	unlockReasonCancelConfirmed = "ADMIN_CANCEL_CONFIRMED"

	unlockAuditActionRequest = "INVOICE_MANUAL_UNLOCK_REQUEST"
	unlockAuditActionApprove = "INVOICE_MANUAL_UNLOCK_APPROVE"
)

// 型別邊界錯誤（契約違規 ⇒ 呼叫端映成 InvalidArgument／400，不借業務碼）。
var (
	ErrUnlockEvidenceRequired  = errors.New("invoice: provider_check_evidence is required")
	ErrUnlockEvidenceTooLong   = errors.New("invoice: provider_check_evidence exceeds 1000 characters")
	ErrUnlockCheckedAtRequired = errors.New("invoice: provider_checked_at is required")
	ErrUnlockCheckedAtFuture   = errors.New("invoice: provider_checked_at must not be in the future")

	// J28 證據（#3089 A 組第 2 項）。
	ErrUnlockEvidenceKindRequired = errors.New("invoice: cancel_evidence_kind is required when the job is CANCEL_BLOCKED_UNKNOWN")
	ErrUnlockEvidenceKindInvalid  = errors.New("invoice: cancel_evidence_kind must be MOF_PLATFORM_SCREENSHOT or AMEGO_RECONCILE_FILE")
	ErrUnlockEvidenceKindUnwanted = errors.New("invoice: cancel_evidence_kind is only accepted when the job is CANCEL_BLOCKED_UNKNOWN")
	ErrUnlockEvidenceKeyOrphan    = errors.New("invoice: cancel_evidence_object_key requires cancel_evidence_kind")
	ErrUnlockEvidenceKeyConflict  = errors.New("invoice: cancel_evidence_object_key and cancel_evidence_staged_token are mutually exclusive")
)

var j28EvidenceKinds = map[string]bool{
	codemeta.InvoiceCancelEvidenceKindMofPlatformScreenshot.Value: true,
	codemeta.InvoiceCancelEvidenceKindAmegoReconcileFile.Value:    true,
}

// ManualUnlockView 是端點 54／55 的回應（proto ManualUnlockInfo）。
//
// Status 已經過 domain.ProjectManualUnlockStatus 投影：DB 上仍掛著但已逾期的那一列
// 在這裡就是 EXPIRED。前端靠它決定顯示「等待覆核」還是「已逾期／可重新發起」。
type ManualUnlockView struct {
	UnlockID              int64
	Status                string
	RequestedByAccountID  int64
	ApprovedByAccountID   *int64
	ProviderCheckEvidence string
	ProviderCheckedAt     time.Time
	RequestedAt           time.Time
	ExpiresAt             time.Time

	// CancelEvidence* 只有 J28 那條路徑有值。**覆核者要看得到它**——沒有它，
	// 第二個人是在對一份看不見證據的請求按同意，雙人確認就只剩儀式。
	CancelEvidenceKind      *string
	CancelEvidenceObjectKey *string

	// NeedsSecondApproval=true ⇒ 呼叫端回 **200 ＋ 9344 ＋ preserveData**（不是錯誤）。
	// 它只在端點 54 撞到「已經有一個等待覆核的請求」時為真，data 帶的是**既有那一列**
	// 的 unlockId，讓 ADM 前端直接導去覆核對話框而不是再開一個新的請求。
	NeedsSecondApproval bool
}

// RequestManualUnlockInput 是端點 54 的輸入。
type RequestManualUnlockInput struct {
	ActorAccountID        int64
	JobID                 int64
	ProviderCheckEvidence string
	ProviderCheckedAt     time.Time
	Reason                string
	ExpectedVersion       int64
	IdempotencyKey        string

	// CancelEvidence* 只在 job 是 CANCEL_BLOCKED_UNKNOWN 時填（J28）。
	// 從 CANCELLED_BY_ADMIN 發起時帶值 ⇒ 400（那條路徑的證據已經在 operation 上，
	// 收下一份不會被用到的證據就是靜默失效）。
	CancelEvidenceKind      string
	CancelEvidenceObjectKey string
	// CancelEvidenceStagedToken 是 ADM 上傳證據檔後拿到的 token（#3108）。
	// 與 ObjectKey 二擇一：有 token 就換發成 objectKey，兩個都給 ⇒ 400。
	CancelEvidenceStagedToken string
}

// RequestManualUnlock 發起一次人工出口解鎖（端點 54）。
//
// allowed-from ＝ **兩個值**：
//
//	CANCELLED_BY_ADMIN      → 覆核時只走 J29（證據已在 operation 上）
//	CANCEL_BLOCKED_UNKNOWN  → 覆核時同一交易內先走 **J28** 再走 J29，發起時必帶證據
//
// 🔴 第二個值是 2026-09-04 補的（#3089 A 組第 2 項）。在那之前 J28 **零呼叫端**，
// 而設計書 §7.5.6 逐字寫著 CANCEL_BLOCKED_UNKNOWN「永不自動終結，唯一出邊是 ADM
// 雙人確認（J28）」⇒ 落到那個狀態的 job 會在 J26 ↔ J27 之間無限循環，端點 54／55／62
// 對它整條不可用。allowed-from 少一個值，症狀是**一個狀態沒有出口**，不是錯誤訊息。
//
// 在這裡先擋一次不是重複防線，是為了不讓 ADM 對一個永遠解不了的 job 造出一列
// 解鎖請求——那一列會一直掛著等一個不可能成功的覆核。
func (u *AdminJob) RequestManualUnlock(ctx context.Context, in RequestManualUnlockInput) (ManualUnlockView, error) {
	if err := requireAdminReason(in.Reason); err != nil {
		return ManualUnlockView{}, err
	}
	if err := validateUnlockEvidence(in.ProviderCheckEvidence); err != nil {
		return ManualUnlockView{}, err
	}

	var out ManualUnlockView
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		op, err := u.ops.GetByID(ctx, in.JobID)
		if err != nil {
			return err
		}
		if op.Version != in.ExpectedVersion {
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}
		blocked := op.Status == codemeta.InvoiceOperationStatusCancelBlockedUnknown.Value
		if op.Status != codemeta.InvoiceOperationStatusCancelledByAdmin.Value && !blocked {
			return pkgerrors.NewBusiness(bizcode.InvoiceJobVersionConflict,
				"本作業目前不是「已由管理員取消」或「取消受阻（狀態不明）」，無法申請人工出口解鎖")
		}
		if in.CancelEvidenceObjectKey != "" && in.CancelEvidenceStagedToken != "" {
			return ErrUnlockEvidenceKeyConflict
		}
		if err := validateUnlockEvidenceKind(blocked, in.CancelEvidenceKind,
			in.CancelEvidenceObjectKey+in.CancelEvidenceStagedToken); err != nil {
			return err
		}
		if !isIssuanceOperation(op.OperationType) {
			return pkgerrors.NewBusiness(bizcode.InvoiceExternalIssuanceNotAllowed,
				"這件工作不是開立類（作廢／折讓／查詢），不需要人工出口解鎖；"+
					"受控取消完成後即為結案")
		}

		now := u.now()
		if err := validateUnlockCheckedAt(in.ProviderCheckedAt, now); err != nil {
			return err
		}

		if _, err := u.unlocks.ExpireStale(ctx, op.ID, now); err != nil {
			return err
		}

		if existing, found, err := u.unlocks.GetOpen(ctx, op.ID, now); err != nil {
			return err
		} else if found {
			out = toManualUnlockView(existing, now)
			out.NeedsSecondApproval = true
			return nil
		}

		evidenceKey := in.CancelEvidenceObjectKey
		if in.CancelEvidenceStagedToken != "" {
			if u.unlockEvidence == nil {
				return pkgerrors.NewBusiness(bizcode.InvoiceExternalIssuanceNotAllowed,
					"證據檔上傳功能尚未就緒，請改用文字說明")
			}
			key, ferr := u.unlockEvidence.FinalizeUnlockEvidence(ctx, in.CancelEvidenceStagedToken, op.ID)
			if ferr != nil {
				return ferr
			}
			evidenceKey = key
		}

		created, taken, err := u.unlocks.Create(ctx, domain.NewManualUnlock{
			ID:                    u.newID(),
			OperationID:           op.ID,
			RequestedByAccountID:  in.ActorAccountID,
			RequestReason:         in.Reason,
			ProviderCheckEvidence: in.ProviderCheckEvidence,
			ProviderCheckedAt:     in.ProviderCheckedAt,
			ExpiresAt:             now.Add(domain.ManualUnlockExpiry),
			Now:                   now,
			CancelEvidenceKind:      nilIfBlank(in.CancelEvidenceKind),
			CancelEvidenceObjectKey: nilIfBlank(evidenceKey),
		})
		if err != nil {
			return err
		}
		if taken {
			return pkgerrors.NewBusiness(bizcode.InvoiceJobVersionConflict,
				"另一位管理員剛剛已發起解鎖請求，請重新整理後改為覆核")
		}

		if _, err := u.appendAudit(ctx, adminAuditInput{
			ActorAccountID: in.ActorAccountID,
			Action:         unlockAuditActionRequest,
			ResourceType:   jobResourceType,
			ResourceID:     op.ID,
			Before:         jobAuditState(op.Status, map[string]string{"manualUnlock": "NONE"}),
			After: jobAuditState(op.Status, map[string]string{
				"manualUnlock": domain.ManualUnlockAwaitingSecondApproval,
				"unlockId":     strconv.FormatInt(created.ID, 10),
			}),
			ReasonCode:     unlockReasonRequested,
			ReasonDetail:   in.Reason,
			IdempotencyKey: in.IdempotencyKey,
			Now:            now,
		}); err != nil {
			return err
		}

		out = toManualUnlockView(created, now)
		return nil
	})
	if err != nil {
		return ManualUnlockView{}, err
	}
	return out, nil
}

// ApproveManualUnlockInput 是端點 55 的輸入。
type ApproveManualUnlockInput struct {
	ActorAccountID  int64
	JobID           int64
	UnlockID        int64
	Approved        bool
	Reason          string
	ExpectedVersion int64
	IdempotencyKey  string
}

// ApproveManualUnlock 覆核一次人工出口解鎖（端點 55）。
//
// 🔴 approved=true 時**兩段必須同一交易**：解鎖列轉 APPROVED ＋ job 走 J29 進
// MANUAL_PROCESS_ALLOWED。只轉前者，ADM 會看到「已核准」而端點 56／62 仍回 9349
// （本作業尚未開放自行開立回報）——而那個組合沒有任何人會發現，因為兩邊各自都自洽。
//
// approved=false ⇒ 解鎖列轉 REJECTED，**job 狀態不動**（它留在 CANCELLED_BY_ADMIN，
// 之後仍可重新發起）。
func (u *AdminJob) ApproveManualUnlock(ctx context.Context, in ApproveManualUnlockInput) (ManualUnlockView, error) {
	if err := requireAdminReason(in.Reason); err != nil {
		return ManualUnlockView{}, err
	}

	var out ManualUnlockView
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		op, err := u.ops.GetByID(ctx, in.JobID)
		if err != nil {
			return err
		}
		if op.Version != in.ExpectedVersion {
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}
		unlock, err := u.unlocks.GetByID(ctx, in.UnlockID)
		if err != nil {
			return err
		}
		if unlock.OperationID != op.ID {
			return jobBiz(bizcode.InvoiceJobNotFound)
		}

		now := u.now()

		if unlock.RequestedByAccountID == in.ActorAccountID {
			return jobBiz(bizcode.InvoiceManualUnlockSameActor)
		}
		if op.CancelRequestedBy != nil && *op.CancelRequestedBy == in.ActorAccountID {
			return jobBiz(bizcode.InvoiceManualUnlockSameActor)
		}
		if unlock.Status != domain.ManualUnlockAwaitingSecondApproval {
			return pkgerrors.NewBusiness(bizcode.InvoiceJobVersionConflict,
				"本解鎖請求已被覆核過，請重新整理")
		}
		if !now.Before(unlock.ExpiresAt) {
			return pkgerrors.NewBusiness(bizcode.InvoiceJobVersionConflict,
				"本解鎖請求已逾期（發起後 24 小時失效），請重新發起")
		}

		nextUnlockStatus := domain.ManualUnlockApproved
		reasonCode := unlockReasonApproved
		if !in.Approved {
			nextUnlockStatus = domain.ManualUnlockRejected
			reasonCode = unlockReasonRejected
		}

		ok, err := u.unlocks.Decide(ctx, domain.DecideManualUnlock{
			ID:                  unlock.ID,
			OperationID:         op.ID,
			NextStatus:          nextUnlockStatus,
			ApprovedByAccountID: in.ActorAccountID,
			DecisionReason:      in.Reason,
			ExpectedVersion:     unlock.Version,
			Now:                 now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}

		jobStatusAfter := op.Status
		if in.Approved {
			jobVersion := op.Version
			if op.Status == codemeta.InvoiceOperationStatusCancelBlockedUnknown.Value {
				kind := ""
				if unlock.CancelEvidenceKind != nil {
					kind = *unlock.CancelEvidenceKind
				}
				if !j28EvidenceKinds[kind] {
					return pkgerrors.NewBusiness(bizcode.InvoiceJobVersionConflict,
						"本解鎖請求沒有可採信的取消證據，請重新發起並附上證據")
				}
				confirmed, err := u.ops.ConfirmCancel(ctx, domain.ConfirmOperationCancel{
					ID:                op.ID,
					ExpectedVersion:   jobVersion,
					ConfirmedBy:       in.ActorAccountID,
					EvidenceKind:      kind,
					EvidenceObjectKey: unlock.CancelEvidenceObjectKey,
					Now:               now,
				})
				if err != nil {
					return err
				}
				if !confirmed {
					return jobBiz(bizcode.InvoiceJobVersionConflict)
				}
				cancelled := codemeta.InvoiceOperationStatusCancelledByAdmin.Value
				if err := u.appendHistory(ctx, subjectOperation, op.ID, op.Status, cancelled,
					unlockReasonCancelConfirmed, in.Reason, in.ActorAccountID, now); err != nil {
					return err
				}
				jobVersion++
				op.Status = cancelled
			}

			allowed, err := u.ops.AllowManualProcess(ctx, op.ID, jobVersion, in.ActorAccountID, now)
			if err != nil {
				return err
			}
			if !allowed {
				return jobBiz(bizcode.InvoiceJobVersionConflict)
			}
			jobStatusAfter = codemeta.InvoiceOperationStatusManualProcessAllowed.Value
			if err := u.appendHistory(ctx, subjectOperation, op.ID, op.Status, jobStatusAfter,
				unlockReasonApproved, in.Reason, in.ActorAccountID, now); err != nil {
				return err
			}
		}

		if _, err := u.appendAudit(ctx, adminAuditInput{
			ActorAccountID: in.ActorAccountID,
			Action:         unlockAuditActionApprove,
			ResourceType:   jobResourceType,
			ResourceID:     op.ID,
			Before: jobAuditState(op.Status, map[string]string{
				"manualUnlock": domain.ManualUnlockAwaitingSecondApproval,
				"unlockId":     strconv.FormatInt(unlock.ID, 10),
			}),
			After: jobAuditState(jobStatusAfter, map[string]string{
				"manualUnlock": nextUnlockStatus,
				"unlockId":     strconv.FormatInt(unlock.ID, 10),
			}),
			ReasonCode:     reasonCode,
			ReasonDetail:   in.Reason,
			IdempotencyKey: in.IdempotencyKey,
			Now:            now,
			ApprovalActorAccountID: &unlock.RequestedByAccountID,
		}); err != nil {
			return err
		}

		updated, err := u.unlocks.GetByID(ctx, unlock.ID)
		if err != nil {
			return err
		}
		out = toManualUnlockView(updated, now)
		return nil
	})
	if err != nil {
		return ManualUnlockView{}, err
	}
	return out, nil
}

// LatestManualUnlock 回一個 job 最近一次的解鎖請求（端點 51 的 manualUnlock 卡）。
//
// found=false ＝ 從未發起過 ⇒ proto 的 manual_unlock 回 nil。**不要回一個空物件**：
// 58A §A06 用 `manualUnlock === null` 決定「可發起」按鈕的可見性，空物件會讓那顆按鈕
// 消失而 ADM 沒有任何路徑走完 A4。
func (u *AdminJob) LatestManualUnlock(ctx context.Context, operationID int64) (ManualUnlockView, bool, error) {
	row, found, err := u.unlocks.GetLatest(ctx, operationID)
	if err != nil || !found {
		return ManualUnlockView{}, false, err
	}
	return toManualUnlockView(row, u.now()), true, nil
}

func toManualUnlockView(u domain.ManualUnlock, now time.Time) ManualUnlockView {
	return ManualUnlockView{
		UnlockID:                u.ID,
		Status:                  domain.ProjectManualUnlockStatus(u.Status, u.ExpiresAt, now),
		RequestedByAccountID:    u.RequestedByAccountID,
		ApprovedByAccountID:     u.ApprovedByAccountID,
		ProviderCheckEvidence:   u.ProviderCheckEvidence,
		ProviderCheckedAt:       u.ProviderCheckedAt,
		RequestedAt:             u.RequestedAt,
		ExpiresAt:               u.ExpiresAt,
		CancelEvidenceKind:      u.CancelEvidenceKind,
		CancelEvidenceObjectKey: u.CancelEvidenceObjectKey,
	}
}

func validateUnlockEvidence(s string) error {
	if isBlank(s) {
		return ErrUnlockEvidenceRequired
	}
	if utf8.RuneCountInString(s) > unlockEvidenceMaxRunes {
		return ErrUnlockEvidenceTooLong
	}
	return nil
}

func validateUnlockEvidenceKind(blocked bool, kind, objectKey string) error {
	if !blocked {
		if !isBlank(kind) || !isBlank(objectKey) {
			return ErrUnlockEvidenceKindUnwanted
		}
		return nil
	}
	if isBlank(kind) {
		return ErrUnlockEvidenceKindRequired
	}
	if !j28EvidenceKinds[kind] {
		return ErrUnlockEvidenceKindInvalid
	}
	return nil
}

func nilIfBlank(s string) *string {
	if isBlank(s) {
		return nil
	}
	return &s
}

func validateUnlockCheckedAt(t, now time.Time) error {
	if t.IsZero() {
		return ErrUnlockCheckedAtRequired
	}
	if t.After(now) {
		return ErrUnlockCheckedAtFuture
	}
	return nil
}

// UnlockEvidenceFile 是端點「解鎖證據下載」的回應（#3108 第 4 件）。
type UnlockEvidenceFile struct {
	Data        []byte
	ContentType string
	FileName    string
}

// ReadUnlockEvidence 讀回一筆解鎖請求的取消證據檔。
//
// 🔴 **三道守衛缺一不可**，而且順序就是述詞的順序：
//   - unlock 存在（repository 把查無映成 9340）
//   - unlock.OperationID == jobID（不屬於這個 job 的 unlock_id 同回 9340——
//     分開回會洩漏「這個 id 存在但掛在別的 job 上」，與 ApproveManualUnlock 同一條規則）
//   - 該列真的有證據檔（沒有就是 9340，不是空檔案：回一個 0 bytes 的附件
//     會讓覆核者以為檔案壞了，而實際上是根本沒附）
//
// ADM 沒有 ownership 概念，授權在 gateway 的 admin group（同 FileActorAdmin）。
func (u *AdminJob) ReadUnlockEvidence(ctx context.Context, jobID, unlockID int64) (UnlockEvidenceFile, error) {
	if u.unlockEvidence == nil {
		return UnlockEvidenceFile{}, jobBiz(bizcode.InvoiceJobNotFound)
	}
	unlock, err := u.unlocks.GetByID(ctx, unlockID)
	if err != nil {
		return UnlockEvidenceFile{}, err
	}
	if unlock.OperationID != jobID {
		return UnlockEvidenceFile{}, jobBiz(bizcode.InvoiceJobNotFound)
	}
	if unlock.CancelEvidenceObjectKey == nil || *unlock.CancelEvidenceObjectKey == "" {
		return UnlockEvidenceFile{}, jobBiz(bizcode.InvoiceJobNotFound)
	}

	data, contentType, fileName, err := u.unlockEvidence.ReadUnlockEvidence(
		ctx, *unlock.CancelEvidenceObjectKey, unlock.OperationID)
	if err != nil {
		return UnlockEvidenceFile{}, err
	}
	if len(data) == 0 {
		return UnlockEvidenceFile{}, jobBiz(bizcode.InvoiceJobNotFound)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if fileName == "" {
		fileName = "unlock-evidence"
	}
	return UnlockEvidenceFile{Data: data, ContentType: contentType, FileName: fileName}, nil
}
