package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

// ErrInvalidTargetType and ErrInvalidReason are defined in errors.go as
// pkgerrors domain errors (see NewInvalidInput) so grpc handlers using
// pkgerrors.GetCode surface a structured client error instead of an opaque
// internal one.
var ErrAlreadyResolved = errors.New("report already resolved")

// ErrUnknownTargetOwner blocks a BAN_USER resolution whose report never got an
// owner identity resolved (target_owner_id NULL/0). That is the normal state
// for REVIEW / CHAT_MESSAGE reports — FanoutHideDispatcher.ResolveOwner returns
// (0, nil) for both, because review-service / im-service expose no owner-lookup
// RPC yet (dispatcher.go's ResolveOwner doc comment) — and it is also what a row
// predating target_owner_id looks like.
//
// Failing closed is deliberate: a ban is a write keyed on an identity, and the
// only alternatives to erroring are suspending id 0 (a no-op the admin would
// read as success) or suspending the target CONTENT id (the bug this whole
// change removes). AdminResolve dispatches side effects BEFORE transitioning
// status, so an error here leaves the report PENDING and the admin can retry or
// pick a different action — the same contract HIDE_TARGET's retryable branch has.
var ErrUnknownTargetOwner = errors.New("report has no resolved target owner; cannot ban")

const reportRateLimitPerDay = 20

// isValidTargetType / isValidReasonCode answer against the generated CodeMeta
// registry rather than a map transcribed here. The value domain also lives in
// the DB CHECK constraints (chk_reports_target_type / chk_reports_reason_code)
// and in the ADM filter dropdowns; a hand-copied third table is what let
// reason_code drift — social.proto's comment still advertised HATE / ADULT,
// two values no CHECK ever accepted, while omitting the four that exist.
func isValidTargetType(v string) bool {
	_, ok := codemeta.ReportTargetTypeFromValue(v)
	return ok
}

func isValidReasonCode(v string) bool {
	_, ok := codemeta.ReportReasonCodeFromValue(v)
	return ok
}

// HideTargetDispatcher abstracts the cross-service / in-process side-effect
// dispatch for HIDE_TARGET. Implementations live in usecase package and
// fan out to the right service based on targetType.
type HideTargetDispatcher interface {
	HideTarget(ctx context.Context, targetType string, targetID int64, reason string) error
}

type BanUserDispatcher interface {
	BanUser(ctx context.Context, ownerID int64, reason string) error
}

type ReportUseCase struct {
	reportRepo    domain.ReportRepository
	auditRepo     domain.ModerationActionRepository
	hider         HideTargetDispatcher
	banner        BanUserDispatcher
	ownerResolver domain.TargetOwnerResolver
	tsid          func() int64
}

func NewReportUseCase(
	reportRepo domain.ReportRepository,
	auditRepo domain.ModerationActionRepository,
	hider HideTargetDispatcher,
	banner BanUserDispatcher,
	ownerResolver domain.TargetOwnerResolver,
	tsid func() int64,
) *ReportUseCase {
	return &ReportUseCase{
		reportRepo:    reportRepo,
		auditRepo:     auditRepo,
		hider:         hider,
		banner:        banner,
		ownerResolver: ownerResolver,
		tsid:          tsid,
	}
}

// SubmitReport validates the request, enforces rate limit + 24h idempotency,
// resolves target_owner_id, and inserts into the unified `reports` table.
func (uc *ReportUseCase) SubmitReport(ctx context.Context, p domain.CreateReportParams) (*gen.Report, error) {
	if !isValidTargetType(p.TargetType) {
		return nil, ErrInvalidTargetType
	}
	if !isValidReasonCode(p.ReasonCode) {
		return nil, ErrInvalidReason
	}
	// 24h idempotency: if same reporter+target was reported within 24h,
	// return existing record.
	existing, err := uc.reportRepo.GetByReporterAndTarget(ctx, p.ReporterID, p.TargetType, p.TargetID)
	if err == nil && existing != nil {
		return existing, nil
	}
	count, err := uc.reportRepo.CountByReporterSince(ctx, p.ReporterID, time.Now().Add(-24*time.Hour))
	if err != nil {
		return nil, err
	}
	if count >= reportRateLimitPerDay {
		return nil, ErrReportRateLimited
	}
	if uc.ownerResolver != nil {
		if ownerID, err := uc.ownerResolver.ResolveOwner(ctx, p.TargetType, p.TargetID); err == nil {
			p.TargetOwnerID = ownerID
		}
	}
	p.ID = uc.tsid()
	return uc.reportRepo.CreateGenericReport(ctx, p)
}

// CreateAdminReport creates a synthetic PENDING report representing a direct
// admin action not originating from an end-user complaint (reporter_id=0,
// sentinel matching the existing "0 = system/no reporter" convention used by
// ModerationActionParams.AdminID/ReportID). Used by AdminModeratePost so every
// hide still produces a report + moderation_actions row — closing the gap
// where direct admin moderation bypassed the audit trail entirely.
//
// reasonCode is the admin-chosen category; unlike SubmitReport (which hard-fails
// ErrInvalidReason) an empty or invalid code gracefully falls back to "OTHER" so a
// takedown never fails just because the category is unset or malformed.
func (uc *ReportUseCase) CreateAdminReport(ctx context.Context, targetType string, targetID int64, reasonCode, reason string) (*gen.Report, error) {
	if !isValidReasonCode(reasonCode) {
		reasonCode = codemeta.ReportReasonCodeOther.Value
	}
	p := domain.CreateReportParams{
		ID:          uc.tsid(),
		ReporterID:  0,
		TargetType:  targetType,
		TargetID:    targetID,
		ReasonCode:  reasonCode,
		Description: reason,
		Evidence:    []byte("[]"), // admin-direct action has no evidence URLs; reports.evidence is NOT NULL
	}
	if uc.ownerResolver != nil {
		if ownerID, err := uc.ownerResolver.ResolveOwner(ctx, targetType, targetID); err == nil {
			p.TargetOwnerID = ownerID
		}
	}
	return uc.reportRepo.CreateGenericReport(ctx, p)
}

func (uc *ReportUseCase) ListMyReports(ctx context.Context, reporterID int64, limit, offset int64) ([]*gen.Report, error) {
	if limit <= 0 {
		limit = 20
	}
	return uc.reportRepo.ListByReporter(ctx, reporterID, limit, offset)
}

func (uc *ReportUseCase) AdminList(ctx context.Context, status, targetType, reasonCode string, limit, offset int64) ([]*gen.Report, error) {
	if limit <= 0 {
		limit = 20
	}
	return uc.reportRepo.AdminList(ctx, status, targetType, reasonCode, limit, offset)
}

func (uc *ReportUseCase) AdminGet(ctx context.Context, id int64) (*gen.Report, error) {
	return uc.reportRepo.AdminGet(ctx, id)
}

// CountRecentActionsByOwner exposes the repo's 90-day recent-action count for
// grpc layer response assembly (admin queue repeat-offender visibility).
func (uc *ReportUseCase) CountRecentActionsByOwner(ctx context.Context, ownerID int64) (int64, error) {
	return uc.reportRepo.CountRecentActionsByOwner(ctx, ownerID)
}

// AdminResolve transitions a PENDING/APPEALED report to a terminal state and
// runs the corresponding side-effect (HIDE_TARGET / BAN_USER / WARN_USER /
// DISMISS). Side effect runs FIRST: if it fails, status is not transitioned
// so admin can retry. The SQL UPDATE filters by status IN ('PENDING',
// 'APPEALED') so a no-op returns ErrAlreadyResolved.
func (uc *ReportUseCase) AdminResolve(ctx context.Context, id int64, action string, resolvedBy int64, note string) (*gen.Report, error) {
	status, err := actionToStatus(action)
	if err != nil {
		return nil, err
	}
	report, err := uc.reportRepo.AdminGet(ctx, id)
	if err != nil {
		return nil, err
	}
	if report.Status != codemeta.ReportStatusPending.Value && report.Status != codemeta.ReportStatusAppealed.Value {
		return nil, ErrAlreadyResolved
	}

	previousState, _ := json.Marshal(map[string]string{"status": report.Status})

	// banOwnerID is the resolved ban target for the BAN_USER path — computed
	// once here and reused below for AffectedUserID, so the audit trail names
	// the person actually banned instead of independently (and incorrectly)
	// re-deriving it from report.TargetOwnerID, which is exactly NULL in the
	// USER-fallback case banTargetOwnerID exists to handle.
	var banOwnerID int64

	// Side-effect dispatch.
	switch action {
	case codemeta.ReportResolveActionHideTarget.Value:
		if uc.hider != nil {
			// ErrNotFound = the target content is already gone (deleted/removed
			// between report time and resolution). The takedown goal (content no
			// longer visible) is already satisfied, so fall through and resolve
			// the report as RESOLVED_HIDDEN. Any other error is treated as
			// retryable: don't transition, so the admin can retry.
			if err := uc.hider.HideTarget(ctx, report.TargetType, report.TargetID, note); err != nil && !errors.Is(err, ErrNotFound) {
				return nil, err
			}
		}
	case codemeta.ReportResolveActionBanUser.Value:
		// report.TargetID is the reported CONTENT's id (post / comment /
		// forum entry / review / chat message). The person to ban is the
		// content's owner, already resolved and snapshotted at submission
		// time — the very value this function feeds to the audit trail's
		// AffectedUserID a few lines below. Resolved unconditionally (not
		// only when uc.banner != nil) so the audit trail stays correct even
		// when dispatch is disabled.
		ownerID, err := banTargetOwnerID(report)
		if err != nil {
			return nil, err
		}
		banOwnerID = ownerID
		if uc.banner != nil {
			if err := uc.banner.BanUser(ctx, ownerID, note); err != nil {
				return nil, err
			}
		}
	case codemeta.ReportResolveActionWarnUser.Value, codemeta.ReportResolveActionDismiss.Value:
		// No external dispatch — audit-only actions.
	}

	row, err := uc.reportRepo.AdminResolve(ctx, id, status, action, resolvedBy)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, ErrAlreadyResolved
	}

	// Audit (best-effort: failure is silently discarded — no logger available
	// at this layer — the resolution itself already succeeded, so this is not
	// rolled back).
	if uc.auditRepo != nil {
		newState, _ := json.Marshal(map[string]string{"status": row.Status})
		var affectedUserID int64
		if action == codemeta.ReportResolveActionBanUser.Value {
			// banOwnerID (via banTargetOwnerID) is the corrected source: it
			// covers the USER-fallback case where TargetOwnerID is NULL but
			// the owner is still knowable (target_id IS the owner for USER
			// reports).
			affectedUserID = banOwnerID
		} else if report.TargetOwnerID != nil {
			affectedUserID = *report.TargetOwnerID
		}
		_ = uc.auditRepo.Create(ctx, domain.ModerationActionParams{
			ID:             uc.tsid(),
			ReportID:       id,
			AdminID:        resolvedBy,
			AffectedUserID: affectedUserID,
			Action:         action,
			Note:           note,
			PreviousState:  previousState,
			NewState:       newState,
		})
	}
	return row, nil
}

// banTargetOwnerID picks the identity a BAN_USER resolution must act on.
//
// Primary source is the snapshotted target_owner_id: it was resolved at
// submission time from live content (post.seller_id / comment.user_id /
// forum.user_id — see FanoutHideDispatcher.ResolveOwner), so it still answers
// "who wrote this" even after the content itself has been deleted or hidden
// between the report and its resolution.
//
// The USER fallback covers rows whose target_owner_id was never populated (a
// nil ownerResolver, or a row predating the column) without weakening anything:
// for target_type=="USER" ResolveOwner is the identity function, so target_id
// IS the owner by definition. No other target type has such an identity — for
// them an unset owner means "unknown", and guessing is exactly the defect being
// fixed here.
func banTargetOwnerID(report *gen.Report) (int64, error) {
	if report.TargetOwnerID != nil && *report.TargetOwnerID != 0 {
		return *report.TargetOwnerID, nil
	}
	if report.TargetType == codemeta.ReportTargetTypeUser.Value && report.TargetID != 0 {
		return report.TargetID, nil
	}
	return 0, ErrUnknownTargetOwner
}

// Reopen transitions a terminal report (RESOLVED_*/DISMISSED) back to
// APPEALED so it can be re-resolved via AdminResolve. Does not overwrite the
// original resolved_action/resolved_by — those remain as history; the new
// resolution after re-review is a second AdminResolve call that updates them.
func (uc *ReportUseCase) Reopen(ctx context.Context, id int64, adminID int64, note string) (*gen.Report, error) {
	row, err := uc.reportRepo.Reopen(ctx, id)
	if err != nil {
		return nil, err
	}
	// Audit (best-effort: failure is silently discarded — no logger available
	// at this layer — the reopen itself already succeeded, so this is not
	// rolled back).
	if uc.auditRepo != nil {
		var affectedUserID int64
		if row.TargetOwnerID != nil {
			affectedUserID = *row.TargetOwnerID
		}
		_ = uc.auditRepo.Create(ctx, domain.ModerationActionParams{
			ID:             uc.tsid(),
			ReportID:       id,
			AdminID:        adminID,
			AffectedUserID: affectedUserID,
			// ModerationActionType, not ReportResolveAction: REOPEN is an audit
			// verb with no terminal state, so it exists only in the superset
			// enum. Writing it as a resolve action would put a non-terminal
			// value into reports.resolved_action.
			Action: codemeta.ModerationActionTypeReopen.Value,
			Note:   note,
		})
	}
	return row, nil
}

// actionToStatus is the one-to-one mapping from a ReportResolveAction (the
// admin's verdict) to the ReportStatus terminal state it lands the report in.
// Both sides read the generated CodeMeta registry: the pairing is the reason
// those two enums exist as a pair, and a hand-typed literal on either side
// drifts into a value the DB CHECK rejects at 23514 time.
//
// Only ReportResolveAction values appear here. ModerationActionType's extra
// members (AUTO_REJECT / REOPEN) are audit-log verbs with no terminal state —
// accepting one would write a non-terminal value into reports.resolved_action.
func actionToStatus(action string) (string, error) {
	switch action {
	case codemeta.ReportResolveActionHideTarget.Value:
		return codemeta.ReportStatusResolvedHidden.Value, nil
	case codemeta.ReportResolveActionWarnUser.Value:
		return codemeta.ReportStatusResolvedWarned.Value, nil
	case codemeta.ReportResolveActionBanUser.Value:
		return codemeta.ReportStatusResolvedBanned.Value, nil
	case codemeta.ReportResolveActionDismiss.Value:
		return codemeta.ReportStatusDismissed.Value, nil
	default:
		return "", errors.New("invalid resolve action")
	}
}
