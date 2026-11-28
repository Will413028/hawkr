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

const (
	complaintActionRemindSeller        = "REMIND_SELLER"
	complaintActionRequestRework       = "REQUEST_REWORK"
	complaintActionDecideResolved      = "DECIDE_RESOLVED"
	complaintActionDecideViolation     = "DECIDE_VIOLATION"
	complaintActionRecommendSuspension = "RECOMMEND_SUSPENSION"
)

const (
	complaintViolationWindowDays = 90
	complaintViolationThreshold  = 3
	complaintSuspendDays         = 14
	complaintReopenCooldown      = 24 * time.Hour

	complaintResolutionNoViolation = "NO_VIOLATION"
	complaintResolutionViolation   = "VIOLATION"

	complaintReasonAdminDecision = "ADMIN_COMPLAINT_DECISION"

	complaintAuditAction  = "INVOICE_COMPLAINT_ACT"
	complaintResourceType = "COMPLAINT"

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
	if in.Action == complaintActionRecommendSuspension && !in.Confirmed {
		return ComplaintActionResult{}, ErrComplaintConfirmRequired
	}

	var out ComplaintActionResult
	err := u.tx.WithTx(ctx, func(ctx context.Context) error {
		c, err := u.complaints.GetByID(ctx, in.ComplaintID)
		if err != nil {
			return err
		}
		if c.Version != in.ExpectedVersion {
			return complaintVersionConflict()
		}
		now := u.now()

		nextStatus, err := u.applyAction(ctx, in, c, now)
		if err != nil {
			return err
		}

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

func (u *AdminComplaint) applyAction(
	ctx context.Context, in ComplaintActionInputAdmin, c domain.Complaint, now time.Time,
) (string, error) {
	switch in.Action {
	case complaintActionRemindSeller, complaintActionRecommendSuspension:
		return c.Status, nil

	case complaintActionRequestRework:
		cal, err := u.calendarAround(ctx, now)
		if err != nil {
			return "", err
		}
		ok, err := u.complaints.ReturnToSeller(ctx, c.ID, c.Version, cal.ComplaintSlaDueAt(now), now)
		return codemeta.InvoiceComplaintStatusOpen.Value, complaintCASResult(ok, err)

	case complaintActionDecideResolved:
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
	return "", ErrComplaintActionUnknown
}

func (u *AdminComplaint) appendTimeline(
	ctx context.Context, in ComplaintActionInputAdmin, c domain.Complaint, now time.Time,
) error {
	action := timelineActionFor(in.Action)
	if action == "" {
		return nil
	}
	_, err := u.complaints.AppendEvent(ctx, domain.NewComplaintEvent{
		ComplaintID:    c.ID,
		EventType:      codemeta.InvoiceComplaintEventTypePlatformDecision.Value,
		Action:         action,
		ActorType:      complaintActorAdmin,
		ActorAccountID: in.ActorAccountID,
		Message:        in.Reason,
		Now:            now,
	})
	return err
}

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
	issuer, err := u.issuers.GetByID(ctx, c.IssuerID)
	if err != nil {
		return nil, err
	}
	reached := count >= complaintViolationThreshold
	if reached {
		u.logger().Info("invoice.admin.complaint.suspension_threshold_reached",
			zap.Int64("complaint_id", c.ID), zap.Int64("issuer_id", c.IssuerID),
			zap.Int64("violations_90d", count))
	}
	return &SuspensionRecommendation{
		ShouldSuspend:          reached || in.Action == complaintActionRecommendSuspension,
		StoreID:                issuer.IssuerRefID,
		ViolationCountIn90Days: count,
		ThresholdReached:       reached,
		SuggestedSuspendDays:   complaintSuspendDays,
		SuggestedReason: "發票未開立申訴判定違規（申訴案件 #" + strconv.FormatInt(c.ID, 10) +
			"；90 天內第 " + strconv.FormatInt(count, 10) + " 件）：" + in.Reason,
	}, nil
}

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

func (u *AdminComplaint) calendarAround(ctx context.Context, now time.Time) (domain.BusinessCalendar, error) {
	return loadBusinessCalendar(ctx, u.businessDay,
		now.AddDate(0, 0, -complaintCalendarLookbehind),
		now.AddDate(0, 0, complaintCalendarLookahead))
}

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
