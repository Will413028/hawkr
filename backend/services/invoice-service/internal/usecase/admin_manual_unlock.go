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

// 本檔是 A4 受控人工出口的**雙人解鎖**（§6.1.14 端點 54 發起／55 覆核）。
//
// 它與同檔案家族的 admin_job.go 共用同一組守衛形狀（必填理由 ＋ expectedVersion ＋
// Idempotency-Key 落稽核 request_id），但多了一條 admin_job.go 沒有的規則：
//
//	🔴 **雙人確認有兩道守衛，兩道都在 SQL 的 WHERE**
//	   ① invoice_manual_unlocks：`requested_by_account_id <> $approver` ⇒ 9345
//	   ② invoice_operations（J29）：`cancel_requested_by <> $approver` ⇒ 同樣是 9345
//	   兩個「第一人」未必是同一位 ADM（提出取消的與發起解鎖的可以不同），所以兩道都要。
//	   本層的 pre-read 只負責把 rows=0 翻成**可分辨**的碼，它不是防線——兩個分頁同時
//	   送出時，先讀後判的窗口擋不住任何東西。
//
//	🔴 **逾期不由排程器收斂**（§5.1.16 紅字）。端點 54 進來時在同一交易內先把過期那一列
//	   就地轉 EXPIRED，才騰得出 uq_invmu_open_per_operation 的名額；讀取面另以
//	   domain.ProjectManualUnlockStatus 投影。少了前者，一個 job 逾期一次之後就永遠
//	   再也發起不了第二次解鎖，而症狀是 23505 不是任何業務碼。

const (
	// unlockEvidenceMaxRunes 是 providerCheckEvidence 的上限（§6.2.1 逐字 1–1000）。
	// DB 端 chk_invmu_evidence_len 守同一件事；兩道都要，理由同 externalInvoiceNoPattern：
	// DB 那道回 23514（對 ADM 是一句 500），本層這道才回得出「查證說明過長」。
	unlockEvidenceMaxRunes = 1000

	// invoice_status_history.reason_code 與 invoice_admin_audit_logs.reason_code 的解鎖軸。
	unlockReasonRequested = "ADMIN_MANUAL_UNLOCK_REQUESTED"
	unlockReasonApproved  = "ADMIN_MANUAL_UNLOCK_APPROVED"
	unlockReasonRejected  = "ADMIN_MANUAL_UNLOCK_REJECTED"

	// J28（CANCEL_BLOCKED_UNKNOWN → CANCELLED_BY_ADMIN）那一列狀態歷程的 reason_code。
	// 與 unlockReasonApproved 分開：同一次覆核會寫兩列歷程（先 J28 再 J29），兩列用同一個
	// reason_code 的話事後看不出哪一列是「確認取消」、哪一列是「解鎖」。
	unlockReasonCancelConfirmed = "ADMIN_CANCEL_CONFIRMED"

	// invoice_admin_audit_logs.action。
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

// j28EvidenceKinds 是 **J28 收得下的證據種類**，比 CodeMeta InvoiceCancelEvidenceKind
// 的四值窄兩個——理由與 DB 的 chk_invmu_evidence_kind 同一條（見 schema.sql 紅字）：
// PROVIDER_QUERY_71／PROVIDER_LIST_ABSENT 是 provider 直接作證、由 J24 自動落地，
// 沒有第二個人；本層若收下它們，就是讓 ADM 替系統蓋一個系統自己蓋得了的章。
//
// 🔴 它不是 CodeMeta 的拷貝，是 CodeMeta 的**子集合**。所以它從 codemeta 的常數取值
// （拼錯字會編譯失敗），而不是自己寫兩個字串字面值。
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
			return err // repository 已把查無映成 9340
		}
		if op.Version != in.ExpectedVersion {
			return jobBiz(bizcode.InvoiceJobVersionConflict)
		}
		// allowed-from ＝ **兩個值**（見函式註解）。CANCEL_BLOCKED_UNKNOWN 那條要帶證據，
		// 它在同一次覆核裡會先走 J28 再走 J29。
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
		// 🔴 人工出口解鎖只對**開立類**有意義（#3112，2026-09-04）。
		//
		// A4 的完整鏈路是「查證未開立 → 受控取消 → 雙人解鎖 → **賣家自行開立並回報號碼**」，
		// 最後一步就是端點 56。而 VOID／ALLOWANCE／VOID_ALLOWANCE／QUERY 沒有
		// 「自行開立的發票號碼」這回事 ⇒ 解鎖之後**沒有任何出口**：
		// job 會永遠停在 MANUAL_PROCESS_ALLOWED，佔著積壓指標而沒有人推得走它
		// （dev 上實測到的正是這個，見 #3112）。
		//
		// ⚠️ 在這裡擋而不是只在端點 56 擋：端點 56 那道是「已經走進死路才告訴你」，
		// 而這一道讓 job 停在 CANCELLED_BY_ADMIN——對作廢類而言那**就是**終局
		// （這件作廢工作被 ADM 取消了），不需要再解鎖。
		if !isIssuanceOperation(op.OperationType) {
			return pkgerrors.NewBusiness(bizcode.InvoiceExternalIssuanceNotAllowed,
				"這件工作不是開立類（作廢／折讓／查詢），不需要人工出口解鎖；"+
					"受控取消完成後即為結案")
		}

		now := u.now()
		if err := validateUnlockCheckedAt(in.ProviderCheckedAt, now); err != nil {
			return err
		}

		// ① 名額釋放：把過期而仍掛著的那一列就地轉 EXPIRED（見檔頭紅字）。
		if _, err := u.unlocks.ExpireStale(ctx, op.ID, now); err != nil {
			return err
		}

		// ② 已經有人發起且尚未逾期 ⇒ 200 ＋ 9344 ＋ 既有那一列的 unlockId。
		//    回 code 0 的話 ADM 會以為自己剛剛發起了一個新請求，而實際上什麼都沒建立。
		if existing, found, err := u.unlocks.GetOpen(ctx, op.ID, now); err != nil {
			return err
		} else if found {
			out = toManualUnlockView(existing, now)
			out.NeedsSecondApproval = true
			return nil
		}

		// 證據檔換發（#3108）：ADM 上傳後拿到的 staged token 在這裡變成 objectKey。
		//
		// 🔴 放在交易**內**、Create 之前：換發會建立 asset_reference，而若 Create
		// 撞唯一鍵（另一位 ADM 搶先）整筆 rollback ⇒ 那個 reference 會變成孤兒。
		// 這是已知的取捨——media 的 GC 會回收沒有引用的資產，而反過來（先建列後換發）
		// 會讓一列已經送出的解鎖請求指向一個還沒轉正的 token，覆核者下載不到。
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
			// 空字串存 NULL：DB 的 chk_invmu_evidence_kind 用 IS NULL 當豁免，
			// 存進去一個空字串會直接違反它（23514 ⇒ 500）。
			CancelEvidenceKind:      nilIfBlank(in.CancelEvidenceKind),
			CancelEvidenceObjectKey: nilIfBlank(evidenceKey),
		})
		if err != nil {
			return err
		}
		if taken {
			// 撞 uq_invmu_open_per_operation：另一位 ADM 在 ② 與這一句之間搶先建了一列。
			// 🔴 交易此刻已 aborted（25P02）⇒ **不得再下任何一句 SQL**（包含「回去把那一列
			// 讀出來回 9344」）。整筆 rollback，讓 ADM 重新整理後看到對方那一列。
			return pkgerrors.NewBusiness(bizcode.InvoiceJobVersionConflict,
				"另一位管理員剛剛已發起解鎖請求，請重新整理後改為覆核")
		}

		// ③ 稽核。before/after 只放狀態相關欄位——providerCheckEvidence 是自由文字，
		//    可能寫進光貿客服姓名／電話，而本表保存 5 年（§9.5）。它的落點是
		//    invoice_manual_unlocks 那一欄本身，不是稽核列。
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
			return err // 查無 ⇒ 9340
		}
		if unlock.OperationID != op.ID {
			// 這把 unlock_id 不屬於這個 job。與「不存在」同回 9340：對 ADM 而言下一步
			// 相同，而分開回會洩漏「這個 id 存在但掛在別的 job 上」。
			return jobBiz(bizcode.InvoiceJobNotFound)
		}

		now := u.now()

		// ── 三個 pre-read 只為了回**可分辨**的碼；真正的守衛是底下那兩句 CAS ──
		if unlock.RequestedByAccountID == in.ActorAccountID {
			return jobBiz(bizcode.InvoiceManualUnlockSameActor)
		}
		if op.CancelRequestedBy != nil && *op.CancelRequestedBy == in.ActorAccountID {
			// J29 的第二道雙人守衛：覆核的人不得是當初提出取消的那一位。
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
			// ── J28（只有從 CANCEL_BLOCKED_UNKNOWN 發起的解鎖才有這一段）──
			//
			// 🔴 **版本會在這裡 +1**：J28 的 UPDATE 有 `version = version + 1`，所以下面
			// 那句 J29 必須用 op.Version+1。兩句都傳 op.Version 的話 J29 會 rows=0，
			// 而 rows=0 在這裡是**靜默**的——ADM 看到「已核准」、job 卻停在
			// CANCELLED_BY_ADMIN，端點 56／62 繼續回 9349，兩邊各自都自洽。
			jobVersion := op.Version
			if op.Status == codemeta.InvoiceOperationStatusCancelBlockedUnknown.Value {
				kind := ""
				if unlock.CancelEvidenceKind != nil {
					kind = *unlock.CancelEvidenceKind
				}
				if !j28EvidenceKinds[kind] {
					// 只可能發生在「發起時還允許空證據的舊列」上。回可分辨的碼而不是讓
					// chk_invop_admin_cancel_needs_confirm 吐 23514（對 ADM 是一句 500）。
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
				// allowed-from 不符或 J29 的雙人述詞不成立。上面的 pre-read 已把後者翻成
				// 9345，走到這裡的多半是狀態被搶先改走。
				return jobBiz(bizcode.InvoiceJobVersionConflict)
			}
			jobStatusAfter = codemeta.InvoiceOperationStatusManualProcessAllowed.Value
			if err := u.appendHistory(ctx, subjectOperation, op.ID, op.Status, jobStatusAfter,
				unlockReasonApproved, in.Reason, in.ActorAccountID, now); err != nil {
				return err
			}
		}

		// 稽核：approval_actor_account_id 是本表**唯一**為雙人確認保留的欄位
		// （chk_invadmaudit_two_person 保證它 ≠ actor）。這裡的 actor 是第二位，
		// 所以第二人欄填的是**第一位**（發起人）——反過來填會讓稽核上看不出誰先誰後。
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
			// 第二人欄填的是**第一位**（發起人）：本列的 actor 已經是第二位。
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

// toManualUnlockView 把 domain 列折成回應，並把逾期投影成 EXPIRED。
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

// validateUnlockEvidence 驗 1–1000 字。
//
// 用 RuneCount 不是 len：查證說明幾乎一定是中文，用 byte 數會讓 334 個中文字就被擋下來，
// 而 DB 那道 CHECK 用的是 length()（PostgreSQL 的字元數）——兩邊口徑必須一致，否則會出現
// 「應用層放行、DB 回 23514」的 500。
func validateUnlockEvidence(s string) error {
	if isBlank(s) {
		return ErrUnlockEvidenceRequired
	}
	if utf8.RuneCountInString(s) > unlockEvidenceMaxRunes {
		return ErrUnlockEvidenceTooLong
	}
	return nil
}

// validateUnlockEvidenceKind 驗 J28 的證據種類。
//
// 兩個方向都要擋，這是本函式存在的理由：
//   - blocked=true 而沒帶 ⇒ 那一列到了覆核時無法滿足 chk_invop_admin_cancel_needs_confirm，
//     而**第一個人早就送出了**，錯誤要等第二個人按核准才爆。
//   - blocked=false 卻帶了 ⇒ 那份證據不會被任何一句 SQL 讀到（CANCELLED_BY_ADMIN 的證據
//     在 operation 上），收下它就是靜默失效。
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
	// objectKey 是選填的（截圖／對帳檔才有），但**只收 objectKey 不收 URL**：
	// 49 號規範的 DB 只存 objectKey。這裡不再驗前綴——與端點 62 的證據上傳共用同一條
	// 路徑，驗證在那裡（media 的 upload_session 已經保證了 key 的形狀）。
	return nil
}

// nilIfBlank 把空白字串折成 NULL（DB 的 CHECK 用 IS NULL 當豁免，空字串不是豁免）。
func nilIfBlank(s string) *string {
	if isBlank(s) {
		return nil
	}
	return &s
}

// validateUnlockCheckedAt 驗查證時間。
//
// 未來時間一律 400：它是「我在某個時點向光貿確認過沒開出去」的事實紀錄，而一個還沒到的
// 時點不可能發生過。放行它等於讓一列 5 年期的證據自我矛盾。
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
		// media 回了空內容 ⇒ 當成查無，不要把 0 bytes 當附件送出去。
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
