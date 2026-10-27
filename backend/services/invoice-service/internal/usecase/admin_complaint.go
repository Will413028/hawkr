package usecase

import (
	"context"
	"errors"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	"github.com/Will413028/hawkr/invoice-service/internal/domain"
	"go.uber.org/zap"
)

// 本檔是 A08 申訴裁決的**寫入面**（§6.1.16 端點 60 ＝ §7.3.5 的 K08／K09／K10）。
// 讀取面（端點 58／59）在 query_read.go。
//
// 四條貫穿全檔的約束：
//
//	① **allowed-from 與 expectedVersion 都在 SQL 的 WHERE**。本層先讀一次是為了回
//	   可分辨的碼（9309／9322／9336），不是正確性防線；rows=0 一律回 9336。
//	② **一次動作 ＝ 一個交易**：狀態 CAS ＋ 時間軸事件 ＋ 狀態歷程 ＋ ADM 稽核同進同退。
//	③ 🔴 **發票模組只提建議、不執行處分**（§6.1.16、I-57）。DECIDE_VIOLATION 只做兩件事：
//	   寫一列帶「當下窗口計數與門檻」的歷程，以及回一個 recommendation 物件。
//	   **沒有第三件**：不蓋 suspended_at、不發停權事件、不排恢復 job、不對停權倒數。
//	   處分走 seller-service 既有治理端點，由 ADM 在治理頁確認後送出。
//	④ **違規計數是滑動窗即時算**，不維護計數器欄。所以它必須被寫進歷程的 reason_detail：
//	   一個月後重跑同一句 SQL 會得到不同答案，那列歷程是事後重建「當時為什麼建議停權」
//	   的唯一依據。

// 端點 60 的五個 action（§6.1.16）。
//
// ⚠️ 它們**不是** CodeMeta InvoiceComplaintAction 的值：那張碼表是「時間軸上記了哪一顆
// 按鈕」，只有 6 值且不含 REQUEST_REWORK／RECOMMEND_SUSPENSION（那兩個刻意不寫時間軸）。
// 本組是 API 的 action 值域，兩者交集非空但不相等，不得互相代入。
const (
	complaintActionRemindSeller        = "REMIND_SELLER"
	complaintActionRequestRework       = "REQUEST_REWORK"
	complaintActionDecideResolved      = "DECIDE_RESOLVED"
	complaintActionDecideViolation     = "DECIDE_VIOLATION"
	complaintActionRecommendSuspension = "RECOMMEND_SUSPENSION"
)

const (
	// complaintViolationWindowDays 是違規計數窗口（§6.1.16「沿用申訴佇列既有窗口，
	// 不另造第二個時間窗」）。
	complaintViolationWindowDays = 90
	// complaintViolationThreshold 是建議停權的門檻（C1：90 天內 3 次）。
	complaintViolationThreshold = 3
	// complaintSuspendDays 是建議停權天數（C2）。**只是建議**——發票模組不寫停權狀態、
	// 不倒數、不排恢復 job；14 天到期自動恢復由 seller-service 既有 cron 負責。
	complaintSuspendDays = 14
	// complaintReopenCooldown 是進 RESOLVED 之後買家可重開的冷卻（K13 的守衛讀它）。
	complaintReopenCooldown = 24 * time.Hour

	// invoice_complaints.resolution_code 的兩個 ADM 取值（欄位註解四值中的後兩個）。
	complaintResolutionNoViolation = "NO_VIOLATION"
	complaintResolutionViolation   = "VIOLATION"

	// invoice_status_history.reason_code 的申訴軸。
	complaintReasonAdminDecision = "ADMIN_COMPLAINT_DECISION"

	// invoice_admin_audit_logs.action / resource_type。
	complaintAuditAction  = "INVOICE_COMPLAINT_ACT"
	complaintResourceType = "COMPLAINT"

	// invoice_complaint_events.actor_type 的 ADM 值（chk_invcompev_actor 四值之一）。
	complaintActorAdmin = "ADMIN"
)

var (
	ErrComplaintActionUnknown       = errors.New("invoice: action is outside the defined value set")
	ErrComplaintConfirmRequired     = errors.New("invoice: this action requires confirmed=true")
	ErrComplaintAdminReasonRequired = errors.New("invoice: reason is required")
)

// AdminComplaint 是 A08 的寫入 usecase。
//
// 與 Complaint（買家面）分開的理由不是分層潔癖：兩者的授權形狀相反。買家面每一支 SQL
// 都帶 complainant_buyer_id（Tier A），ADM 面沒有那段述詞——放在同一個型別裡，很容易
// 有人「順手」把 ADM 的方法接到買家路由上，而那個錯誤在測試上完全看不出來。
type AdminComplaint struct {
	tx          database.TxManager
	complaints  domain.ComplaintRepository
	issuers     domain.IssuerRepository
	audit       domain.AuditRepository
	businessDay domain.BusinessDayRepository

	now func() time.Time
	log *zap.Logger
}

// NewAdminComplaint 組出 A08 的寫入 usecase。
func NewAdminComplaint(
	tx database.TxManager,
	complaints domain.ComplaintRepository,
	issuers domain.IssuerRepository,
	audit domain.AuditRepository,
	businessDay domain.BusinessDayRepository,
	log *zap.Logger,
) *AdminComplaint {
	return &AdminComplaint{
		tx: tx, complaints: complaints, issuers: issuers, audit: audit, businessDay: businessDay,
		now: func() time.Time { return time.Now().UTC() },
		log: orNop(log),
	}
}

// WithClock 注入時間來源（測試用）。
func (u *AdminComplaint) WithClock(now func() time.Time) *AdminComplaint {
	if now != nil {
		u.now = now
	}
	return u
}

// ComplaintActionInputAdmin 是端點 60 的輸入。
type ComplaintActionInputAdmin struct {
	ActorAccountID  int64
	ComplaintID     int64
	Action          string
	Reason          string
	Confirmed       bool
	ExpectedVersion int64
	IdempotencyKey  string
}

// SuspensionRecommendation 是回應裡的建議物件（proto SuspensionRecommendation）。
//
// 它只是建議：發票模組不寫停權狀態。管理員在 A08 的「依建議建立措施」開啟建立措施對話框，
// 措施由賣家治理模組的 CreateMeasure 建立，所以這裡不帶任何治理端點。
type SuspensionRecommendation struct {
	ShouldSuspend          bool
	StoreID                int64
	ViolationCountIn90Days int64
	ThresholdReached       bool
	SuggestedSuspendDays   int
	SuggestedReason        string
}

// ComplaintActionResult 是端點 60 的輸出（proto ActOnInvoiceComplaintForAdminData）。
type ComplaintActionResult struct {
	ComplaintID    int64
	Status         string
	Version        int64
	AuditLogID     int64
	Recommendation *SuspensionRecommendation
}

// Act 執行 ADM 的申訴處理動作（端點 60）。
//
// 五個 action 的落點（§6.1.16 對照表）：
//
//	REMIND_SELLER        不改狀態、不改 SLA；只寫時間軸（action=REMINDED）＋ 稽核
//	REQUEST_REWORK       K10 ESCALATED_TO_ADM → OPEN，**重置 sla_due_at ＋ 清 overdue_at**
//	DECIDE_RESOLVED      K08 → RESOLVED，reopen_allowed_after = now + 24h
//	DECIDE_VIOLATION     K09 → VIOLATION_CONFIRMED，**只有這個計入 90 天違規計數**
//	RECOMMEND_SUSPENSION 不改狀態；只寫建議與稽核
//
// **暫停接單／提前恢復／14 天自動恢復都不在本端點，也不在發票模組。**
func (u *AdminComplaint) Act(ctx context.Context, in ComplaintActionInputAdmin) (ComplaintActionResult, error) {
	if !isKnownComplaintAction(in.Action) {
		return ComplaintActionResult{}, ErrComplaintActionUnknown
	}
	if isBlank(in.Reason) {
		return ComplaintActionResult{}, ErrComplaintAdminReasonRequired
	}
	if utf8.RuneCountInString(in.Reason) > complaintTextMaxRunes {
		return ComplaintActionResult{}, ErrComplaintReasonTooLong
	}
	// 二次確認只有 RECOMMEND_SUSPENSION 要（§6.1 端點 60 的 400「含缺 confirmed」）。
	if in.Action == complaintActionRecommendSuspension && !in.Confirmed {
		return ComplaintActionResult{}, ErrComplaintConfirmRequired
	}

	var out ComplaintActionResult
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		c, err := u.complaints.GetByID(ctx, in.ComplaintID)
		if err != nil {
			return err // repository 已把查無映成 9309
		}
		if c.Version != in.ExpectedVersion {
			return complaintVersionConflict()
		}
		now := u.now()

		nextStatus, err := u.applyAction(ctx, in, c, now)
		if err != nil {
			return err
		}

		// I7 的 ADM 軸留痕在下面（`u.audit.AppendStatusHistory`，就在 recommendationFor
		// 之後）——**這裡刻意不再寫第二次**。
		//
		// 🔴 #3109 曾在這個位置補過一支 appendStatusHistoryDetail，理由是「I7 契約測試
		// 說這個函式沒有留痕」。但那道判準的寫入者名單只認 helper 名（小寫 a 的
		// `appendStatusHistory(`），認不得直接呼叫 repository 的
		// `u.audit.AppendStatusHistory(` ⇒ 它把一個**已經有留痕**的函式判成沒有，
		// 而照著它的建議修，結果是同一次 status 轉移寫**兩列**。
		// dev 實證（2026-09-05，complaint 7501677659433205760）：
		//   ESCALATED_TO_ADM → RESOLVED  DECIDE_RESOLVED           SYSTEM  NULL
		//   ESCALATED_TO_ADM → RESOLVED  ADMIN_COMPLAINT_DECISION  ADMIN   2600000022
		// 兩列同一個 from/to，稽核畫面會出現一次裁決兩筆紀錄，而且上面那筆的 actor 是錯的。
		// 判準本身已一併修正（i7HistCalls 加入直接呼叫的寫法）。
		//
		// 保留下面那一支而不是上面這一支的理由：它的 ReasonDetail 走
		// `historyDetailFor(in, rec)`，帶得出停權建議；而 rec 必須在 CAS 之後才算得出來。

		// 違規計數必須在 CAS **之後**算：K09 剛剛才把本案寫成 VIOLATION_CONFIRMED，
		// 而 CountConfirmedViolationOrders 的述詞正是那個狀態 ⇒ 先算會少算本案自己。
		rec, err := u.recommendationFor(ctx, in, c, now)
		if err != nil {
			return err
		}

		if err := u.appendTimeline(ctx, in, c, now); err != nil {
			return err
		}
		if nextStatus != c.Status {
			if err := u.audit.AppendStatusHistory(ctx, domain.NewStatusHistoryEntry{
				SubjectType:    complaintResourceType,
				SubjectID:      c.ID,
				FromStatus:     nilIfEmpty(c.Status),
				ToStatus:       nextStatus,
				ReasonCode:     nilIfEmpty(complaintReasonAdminDecision),
				ReasonDetail:   nilIfEmpty(historyDetailFor(in, rec)),
				ActorType:      jobActorAdmin,
				ActorAccountID: &in.ActorAccountID,
				Now:            now,
			}); err != nil {
				return err
			}
		}

		auditID, err := u.audit.AppendAdminAuditLog(ctx, domain.NewAdminAuditLogEntry{
			ActorAccountID: in.ActorAccountID,
			Action:         complaintAuditAction,
			ResourceType:   complaintResourceType,
			ResourceID:     c.ID,
			BeforeState:    jobAuditState(c.Status, nil),
			AfterState:     jobAuditState(nextStatus, auditExtraFor(in, rec)),
			ReasonCode:     in.Action,
			ReasonDetail:   in.Reason,
			RequestID:      nilIfEmpty(in.IdempotencyKey),
			Now:            now,
		})
		if err != nil {
			return err
		}

		updated, err := u.complaints.GetByID(ctx, c.ID)
		if err != nil {
			return err
		}
		out = ComplaintActionResult{
			ComplaintID: updated.ID, Status: updated.Status, Version: updated.Version,
			AuditLogID: auditID, Recommendation: rec,
		}
		return nil
	})
	if err != nil {
		return ComplaintActionResult{}, err
	}
	return out, nil
}

// applyAction 跑該 action 的狀態轉移，回「目標狀態」。
//
// 兩個不改狀態的 action 回原狀態（REMIND_SELLER／RECOMMEND_SUSPENSION）——它們的價值
// 全在時間軸與稽核，硬造一次自轉移只會讓 A07 的分頁計數莫名跳動。
func (u *AdminComplaint) applyAction(
	ctx context.Context, in ComplaintActionInputAdmin, c domain.Complaint, now time.Time,
) (string, error) {
	switch in.Action {
	case complaintActionRemindSeller, complaintActionRecommendSuspension:
		return c.Status, nil

	case complaintActionRequestRework:
		// K10。sla_due_at 以工作天曆重算後傳入：不重置的話這一列一進 OPEN 就落回逾期
		// 掃描的候選集合，賣家連看都還沒看到就被再標一次 OVERDUE。
		cal, err := u.calendarAround(ctx, now)
		if err != nil {
			return "", err
		}
		ok, err := u.complaints.ReturnToSeller(ctx, c.ID, c.Version, cal.ComplaintSlaDueAt(now), now)
		return codemeta.InvoiceComplaintStatusOpen.Value, complaintCASResult(ok, err)

	case complaintActionDecideResolved:
		// K08。reopen_allowed_after ＝ now + 24h（K13 的守衛讀它）；不寫它，買家永遠
		// 重開不了——而那條邊正是「RESOLVED 不是終態」的全部意義。
		reopenAfter := now.Add(complaintReopenCooldown)
		ok, err := u.complaints.DecideEscalated(ctx, domain.DecideComplaint{
			ID:                 c.ID,
			ExpectedVersion:    c.Version,
			NextStatus:         codemeta.InvoiceComplaintStatusResolved.Value,
			ResolutionCode:     complaintResolutionNoViolation,
			ResolutionNote:     in.Reason,
			ReopenAllowedAfter: &reopenAfter,
			DecidedByAccountID: in.ActorAccountID,
			Now:                now,
		})
		return codemeta.InvoiceComplaintStatusResolved.Value, complaintCASResult(ok, err)

	case complaintActionDecideViolation:
		// K09。reopen_allowed_after 傳 nil：VIOLATION_CONFIRMED 是終態、沒有重開邊，
		// 寫了也沒有讀者。
		ok, err := u.complaints.DecideEscalated(ctx, domain.DecideComplaint{
			ID:                 c.ID,
			ExpectedVersion:    c.Version,
			NextStatus:         codemeta.InvoiceComplaintStatusViolationConfirmed.Value,
			ResolutionCode:     complaintResolutionViolation,
			ResolutionNote:     in.Reason,
			DecidedByAccountID: in.ActorAccountID,
			Now:                now,
		})
		return codemeta.InvoiceComplaintStatusViolationConfirmed.Value, complaintCASResult(ok, err)
	}
	// isKnownComplaintAction 已在呼叫端驗過，走到這裡代表兩者漂移了。
	return "", ErrComplaintActionUnknown
}

// appendTimeline 寫一列 invoice_complaint_events。
//
// 🔴 只有三個 action 會寫（§C 的 InvoiceComplaintAction 逐字）：REQUEST_REWORK 的副作用
// 只有重置 SLA、RECOMMEND_SUSPENSION 只寫建議與稽核 ⇒ 那兩個**刻意不寫時間軸**，
// 所以那張碼表是 6 值不是 8 值。硬給它們一個 action 值會讓 S09／A07 兩份 UI 出現一個
// 碼表裡不存在的字串。
func (u *AdminComplaint) appendTimeline(
	ctx context.Context, in ComplaintActionInputAdmin, c domain.Complaint, now time.Time,
) error {
	action := timelineActionFor(in.Action)
	if action == "" {
		return nil
	}
	_, err := u.complaints.AppendEvent(ctx, domain.NewComplaintEvent{
		ComplaintID: c.ID,
		// 七個 event_type 裡只有 PLATFORM_DECISION 的 actor_type 是 ADMIN；
		// 「平台提醒賣家」沒有自己的 event_type，共用它。缺口登記在交付回報。
		EventType:      codemeta.InvoiceComplaintEventTypePlatformDecision.Value,
		Action:         action,
		ActorType:      complaintActorAdmin,
		ActorAccountID: in.ActorAccountID,
		Message:        in.Reason,
		Now:            now,
	})
	return err
}

// timelineActionFor 把 API 的 action 映成 CodeMeta InvoiceComplaintAction 的值；
// 空字串代表這個 action 不寫時間軸。
func timelineActionFor(apiAction string) string {
	switch apiAction {
	case complaintActionRemindSeller:
		return codemeta.InvoiceComplaintActionReminded.Value
	case complaintActionDecideResolved:
		return codemeta.InvoiceComplaintActionDecidedResolved.Value
	case complaintActionDecideViolation:
		return codemeta.InvoiceComplaintActionDecidedViolation.Value
	}
	return ""
}

// recommendationFor 算出停權建議。只有兩個 action 會回它。
//
// 🔴 計數口徑寫死（§6.1.16）：只計 DECIDE_VIOLATION（＝ status VIOLATION_CONFIRMED）、
// 窗口 90 天、**以去重後的訂單數計**（同一訂單多次申訴只算一次）、門檻 3 次。
func (u *AdminComplaint) recommendationFor(
	ctx context.Context, in ComplaintActionInputAdmin, c domain.Complaint, now time.Time,
) (*SuspensionRecommendation, error) {
	if in.Action != complaintActionDecideViolation && in.Action != complaintActionRecommendSuspension {
		return nil, nil
	}
	count, err := u.complaints.CountConfirmedViolationOrders(ctx, c.IssuerID,
		now.AddDate(0, 0, -complaintViolationWindowDays))
	if err != nil {
		return nil, err
	}
	// store_id 走 invoice_issuers.issuer_ref_id，不是 issuer_id：治理端點的 sellerId
	// 是 store surrogate，把 issuer 的 PK 送過去 seller-service 會查不到（或查到別人）。
	issuer, err := u.issuers.GetByID(ctx, c.IssuerID)
	if err != nil {
		return nil, err
	}
	reached := count >= complaintViolationThreshold
	if reached {
		// 跨過門檻是要有人去治理頁按下去的事：發票模組不執行處分，那條路上唯一
		// 會提醒人的東西就是這一行與回應裡的 recommendation。
		u.logger().Info("invoice.admin.complaint.suspension_threshold_reached",
			zap.Int64("complaint_id", c.ID), zap.Int64("issuer_id", c.IssuerID),
			zap.Int64("violations_90d", count))
	}
	return &SuspensionRecommendation{
		// RECOMMEND_SUSPENSION 是 ADM 主動按的，本來就是在建議停權 ⇒ shouldSuspend
		// 不再要求達到門檻；DECIDE_VIOLATION 則由門檻決定。
		ShouldSuspend:          reached || in.Action == complaintActionRecommendSuspension,
		StoreID:                issuer.IssuerRefID,
		ViolationCountIn90Days: count,
		ThresholdReached:       reached,
		SuggestedSuspendDays:   complaintSuspendDays,
		// 🔴 complaintId 必須寫進這段自由文字：seller_violations 的兩個 source FK 都不是
		// 申訴案件 ⇒ 從違規紀錄回查「這是哪一件申訴造成的」的唯一線索就是它。
		SuggestedReason: "發票未開立申訴判定違規（申訴案件 #" + strconv.FormatInt(c.ID, 10) +
			"；90 天內第 " + strconv.FormatInt(count, 10) + " 件）：" + in.Reason,
	}, nil
}

// historyDetail／auditExtra 把「當下的」窗口計數與門檻寫進留痕。
//
// 🔴 滑動窗即時算 ⇒ 一個月後重跑同一句 SQL 會得到不同答案。這兩處是事後重建
// 「當時為什麼建議停權」的唯一依據（§6.1.16 K09 逐字）。
func historyDetailFor(in ComplaintActionInputAdmin, rec *SuspensionRecommendation) string {
	if rec == nil {
		return in.Action + ": " + in.Reason
	}
	return in.Action + ": " + in.Reason +
		" [90d violations=" + strconv.FormatInt(rec.ViolationCountIn90Days, 10) +
		", threshold=" + strconv.Itoa(complaintViolationThreshold) + "]"
}

func auditExtraFor(in ComplaintActionInputAdmin, rec *SuspensionRecommendation) map[string]string {
	extra := map[string]string{"action": in.Action}
	if rec != nil {
		extra["violationCountIn90Days"] = strconv.FormatInt(rec.ViolationCountIn90Days, 10)
		extra["thresholdReached"] = strconv.FormatBool(rec.ThresholdReached)
		extra["storeId"] = strconv.FormatInt(rec.StoreID, 10)
	}
	return extra
}

// calendarAround 讀一次工作天曆鏡像（區間與買家側同寬）。
func (u *AdminComplaint) calendarAround(ctx context.Context, now time.Time) (domain.BusinessCalendar, error) {
	return loadBusinessCalendar(ctx, u.businessDay,
		now.AddDate(0, 0, -complaintCalendarLookbehind),
		now.AddDate(0, 0, complaintCalendarLookahead))
}

// complaintCASResult 把 CAS 的 rows=0 折成 9336。
//
// 🔴 rows=0 ＝ 守衛不成立（allowed-from 不符／版本被搶先），**不得當成功往下走**：
// ADM 按下去什麼事都不會發生，而且不會有任何錯誤訊號。
func complaintCASResult(ok bool, err error) error {
	if err != nil {
		return err
	}
	if !ok {
		return complaintBiz(bizcode.InvoiceComplaintActionNotAllowed)
	}
	return nil
}

func isKnownComplaintAction(a string) bool {
	switch a {
	case complaintActionRemindSeller, complaintActionRequestRework,
		complaintActionDecideResolved, complaintActionDecideViolation,
		complaintActionRecommendSuspension:
		return true
	}
	return false
}

func (u *AdminComplaint) logger() *zap.Logger {
	if u.log == nil {
		return zap.NewNop()
	}
	return u.log
}
