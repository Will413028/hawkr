package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
)

// fakeReportRepo implements domain.ReportRepository for tests across this
// package. Zero-value fields behave as "no data" (ErrNotFound / empty list);
// set report/countSince/recentActionCount per-test as needed.
type fakeReportRepo struct {
	report            *gen.Report
	countSince        int64
	recentActionCount int64

	// existingReport, when set, is returned by GetByReporterAndTarget to
	// simulate a duplicate report within the 24h dedup window. Falls back
	// to ErrNotFound (no existing record) when nil.
	existingReport *gen.Report
	// created is set true if CreateGenericReport is invoked, so tests can
	// assert the dedup path short-circuits before creating a new report.
	created bool
	// resolveCalled is set true if AdminResolve reaches the repo, so tests can
	// assert a failed side-effect dispatch left the report un-transitioned
	// (AdminResolve runs side effects BEFORE the status UPDATE precisely so a
	// failure is retryable).
	resolveCalled bool
	// lastParams captures the params passed to CreateGenericReport so tests
	// can assert on fields (e.g. Evidence) the returned gen.Report doesn't echo back.
	lastParams domain.CreateReportParams
}

func (f *fakeReportRepo) GetReport(ctx context.Context, postID, reporterID int64) (*gen.Report, error) {
	return nil, ErrNotFound
}

func (f *fakeReportRepo) CreateReport(ctx context.Context, id, postID, reporterID int64, reasonCode, description string) (*gen.Report, error) {
	return &gen.Report{ID: id, ReporterID: reporterID, TargetType: codemeta.ReportTargetTypePost.Value, TargetID: postID, ReasonCode: reasonCode, Description: nullableTestText(description), Status: codemeta.ReportStatusPending.Value}, nil
}

func (f *fakeReportRepo) GetByReporterAndTarget(ctx context.Context, reporterID int64, targetType string, targetID int64) (*gen.Report, error) {
	if f.existingReport != nil {
		return f.existingReport, nil
	}
	return nil, ErrNotFound // no existing 24h record
}

func (f *fakeReportRepo) CreateGenericReport(ctx context.Context, p domain.CreateReportParams) (*gen.Report, error) {
	f.created = true
	f.lastParams = p
	return &gen.Report{
		ID: p.ID, ReporterID: p.ReporterID, TargetType: p.TargetType, TargetID: p.TargetID,
		TargetOwnerID: nullableTestInt64(p.TargetOwnerID),
		ReasonCode:    p.ReasonCode,
		Description:   nullableTestText(p.Description),
		Status:        codemeta.ReportStatusPending.Value,
	}, nil
}

func (f *fakeReportRepo) ListByReporter(ctx context.Context, reporterID, limit, offset int64) ([]*gen.Report, error) {
	return nil, nil
}

func (f *fakeReportRepo) AdminList(ctx context.Context, status, targetType, reasonCode string, limit, offset int64) ([]*gen.Report, error) {
	return nil, nil
}

func (f *fakeReportRepo) AdminGet(ctx context.Context, id int64) (*gen.Report, error) {
	if f.report != nil {
		return f.report, nil
	}
	return nil, ErrNotFound
}

func (f *fakeReportRepo) AdminResolve(ctx context.Context, id int64, status, action string, resolvedBy int64) (*gen.Report, error) {
	f.resolveCalled = true
	if f.report == nil {
		return nil, ErrNotFound
	}
	updated := *f.report
	updated.Status = status
	updated.ResolvedAction = nullableTestText(action)
	updated.ResolvedBy = nullableTestInt64(resolvedBy)
	return &updated, nil
}

func (f *fakeReportRepo) CountByReporterSince(ctx context.Context, reporterID int64, since time.Time) (int64, error) {
	return f.countSince, nil
}

func (f *fakeReportRepo) Reopen(ctx context.Context, id int64) (*gen.Report, error) {
	if f.report == nil {
		return nil, ErrNotFound
	}
	updated := *f.report
	// 這是 fake 自己挑的值，不是產品碼挑的——所以「Reopen 之後是不是 APPEALED」在本
	// 層是同義反覆。真正的斷言在 postgres 套件的 TestReopenSendsAppealedNotPending
	// （攔 DBTX 看 repository 送出什麼），這裡只是讓 fake 不要說謊。
	updated.Status = codemeta.ReportStatusAppealed.Value
	return &updated, nil
}

func (f *fakeReportRepo) CountRecentActionsByOwner(ctx context.Context, ownerID int64) (int64, error) {
	return f.recentActionCount, nil
}

type fakeAuditRepo struct {
	lastParams domain.ModerationActionParams
}

func (f *fakeAuditRepo) Create(ctx context.Context, p domain.ModerationActionParams) error {
	f.lastParams = p
	return nil
}

type fakeHider struct {
	lastReason string
	err        error
}

func (f *fakeHider) HideTarget(ctx context.Context, targetType string, targetID int64, reason string) error {
	f.lastReason = reason
	return f.err
}

// fakeBanner records exactly what AdminResolve handed to BAN_USER. The whole
// point of these tests is that this is the content OWNER's id, never the
// reported content's own id.
type fakeBanner struct {
	called     bool
	lastUserID int64
	lastReason string
	err        error
}

func (f *fakeBanner) BanUser(ctx context.Context, ownerID int64, reason string) error {
	f.called = true
	f.lastUserID = ownerID
	f.lastReason = reason
	return f.err
}

func TestCountRecentActionsByOwner_DelegatesToRepo(t *testing.T) {
	reportRepo := &fakeReportRepo{recentActionCount: 3}
	uc := NewReportUseCase(reportRepo, nil, nil, nil, nil, func() int64 { return 1 })
	count, err := uc.CountRecentActionsByOwner(context.Background(), 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestAdminResolve_HideTarget_AuditsCorrectOwner(t *testing.T) {
	auditRepo := &fakeAuditRepo{}
	reportRepo := &fakeReportRepo{
		report: &gen.Report{ID: 1, Status: codemeta.ReportStatusPending.Value, TargetType: codemeta.ReportTargetTypePost.Value, TargetID: 100, TargetOwnerID: nullableTestInt64(999)},
	}
	hider := &fakeHider{}
	uc := NewReportUseCase(reportRepo, auditRepo, hider, nil, nil, func() int64 { return 42 })

	_, err := uc.AdminResolve(context.Background(), 1, codemeta.ReportResolveActionHideTarget.Value, 7, "spam content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auditRepo.lastParams.AffectedUserID != 999 {
		t.Errorf("expected AffectedUserID=999 (target owner), got %d", auditRepo.lastParams.AffectedUserID)
	}
	if hider.lastReason != "spam content" {
		t.Errorf("expected raw reason passthrough, got %q", hider.lastReason)
	}
}

func TestAdminResolve_HideTarget_TargetAlreadyGone_ResolvesHidden(t *testing.T) {
	reportRepo := &fakeReportRepo{
		report: &gen.Report{ID: 1, Status: codemeta.ReportStatusPending.Value, TargetType: codemeta.ReportTargetTypePost.Value, TargetID: 100},
	}
	hider := &fakeHider{err: ErrNotFound}
	uc := NewReportUseCase(reportRepo, nil, hider, nil, nil, func() int64 { return 42 })

	row, err := uc.AdminResolve(context.Background(), 1, codemeta.ReportResolveActionHideTarget.Value, 7, "gone")
	if err != nil {
		t.Fatalf("target-already-gone should resolve, not error: %v", err)
	}
	if row == nil {
		t.Fatal("expected the report to transition (non-nil row), got nil")
	}
	if row.Status != codemeta.ReportStatusResolvedHidden.Value {
		t.Errorf("expected status %s, got %q", codemeta.ReportStatusResolvedHidden.Value, row.Status)
	}
}

func TestAdminResolve_HideTarget_RetryableError_Blocks(t *testing.T) {
	reportRepo := &fakeReportRepo{
		report: &gen.Report{ID: 1, Status: codemeta.ReportStatusPending.Value, TargetType: codemeta.ReportTargetTypePost.Value, TargetID: 100},
	}
	sentinel := errors.New("downstream unavailable")
	hider := &fakeHider{err: sentinel}
	uc := NewReportUseCase(reportRepo, nil, hider, nil, nil, func() int64 { return 42 })

	if _, err := uc.AdminResolve(context.Background(), 1, codemeta.ReportResolveActionHideTarget.Value, 7, "x"); !errors.Is(err, sentinel) {
		t.Errorf("a non-ErrNotFound (retryable) error must block resolution, got %v", err)
	}
}

func TestAdminResolve_AllowsAppealedPrecondition(t *testing.T) {
	reportRepo := &fakeReportRepo{report: &gen.Report{ID: 1, Status: codemeta.ReportStatusAppealed.Value, TargetType: codemeta.ReportTargetTypePost.Value, TargetID: 100}}
	uc := NewReportUseCase(reportRepo, nil, &fakeHider{}, nil, nil, func() int64 { return 42 })
	if _, err := uc.AdminResolve(context.Background(), 1, codemeta.ReportResolveActionDismiss.Value, 7, "reviewed, no issue"); err != nil {
		t.Fatalf("expected APPEALED to be resolvable, got error: %v", err)
	}
}

// TestAdminResolve_BanUser_BansContentOwnerNotContentID is the regression test
// for the 2026-08-05 T&S bug: AdminResolve used to hand report.TargetID (the
// reported CONTENT's TSID) to BanUser, so BAN_USER only ever aimed at a real
// person when target_type happened to be "USER".
//
// Every row deliberately keeps TargetID != TargetOwnerID — including the USER
// rows, where a production row would have them equal. Equal values would make
// the assertion unable to tell the two code paths apart, which is exactly how
// this bug survived: the one target type that works is the one where the wrong
// field holds the right value.
func TestAdminResolve_BanUser_BansContentOwnerNotContentID(t *testing.T) {
	const (
		contentID    = int64(700001)
		buyerOwnerID = int64(8001)
		storeOwnerID = int64(9001)
	)
	cases := []struct {
		name       string
		targetType string
		ownerID    int64
	}{
		{"POST_seller_owned", codemeta.ReportTargetTypePost.Value, storeOwnerID},
		{"POST_COMMENT_buyer_owned", codemeta.ReportTargetTypePostComment.Value, buyerOwnerID},
		{"POST_COMMENT_seller_owned", codemeta.ReportTargetTypePostComment.Value, storeOwnerID},
		{"SELLER_FORUM_buyer_owned", codemeta.ReportTargetTypeSellerForum.Value, buyerOwnerID},
		{"SELLER_FORUM_seller_owned", codemeta.ReportTargetTypeSellerForum.Value, storeOwnerID},
		{"REVIEW_buyer_owned", codemeta.ReportTargetTypeReview.Value, buyerOwnerID},
		{"REVIEW_seller_owned", codemeta.ReportTargetTypeReview.Value, storeOwnerID},
		{"CHAT_MESSAGE_buyer_owned", codemeta.ReportTargetTypeChatMessage.Value, buyerOwnerID},
		{"CHAT_MESSAGE_seller_owned", codemeta.ReportTargetTypeChatMessage.Value, storeOwnerID},
		{"USER_buyer", codemeta.ReportTargetTypeUser.Value, buyerOwnerID},
		{"USER_seller", codemeta.ReportTargetTypeUser.Value, storeOwnerID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reportRepo := &fakeReportRepo{report: &gen.Report{
				ID: 1, Status: codemeta.ReportStatusPending.Value,
				TargetType:    tc.targetType,
				TargetID:      contentID,
				TargetOwnerID: nullableTestInt64(tc.ownerID),
			}}
			banner := &fakeBanner{}
			uc := NewReportUseCase(reportRepo, nil, nil, banner, nil, func() int64 { return 42 })

			row, err := uc.AdminResolve(context.Background(), 1, codemeta.ReportResolveActionBanUser.Value, 7, "repeat offender")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !banner.called {
				t.Fatal("expected BanUser to be dispatched")
			}
			if banner.lastUserID != tc.ownerID {
				t.Errorf("BanUser got %d, want the content owner %d (content id is %d)", banner.lastUserID, tc.ownerID, contentID)
			}
			if banner.lastUserID == contentID {
				t.Errorf("BanUser was handed the reported CONTENT id %d — this is the exact 2026-08-05 bug", contentID)
			}
			if banner.lastReason != "repeat offender" {
				t.Errorf("expected raw note passthrough, got %q", banner.lastReason)
			}
			if row.Status != codemeta.ReportStatusResolvedBanned.Value {
				t.Errorf("expected status %s, got %q", codemeta.ReportStatusResolvedBanned.Value, row.Status)
			}
		})
	}
}

// TestAdminResolve_BanUser_NoResolvedOwner_FailsClosed: REVIEW / CHAT_MESSAGE
// reports get target_owner_id = NULL (ResolveOwner returns 0 for both — no
// cross-service owner-lookup RPC exists yet), and so do rows predating the
// column. Banning "nobody" must error and must leave the report PENDING, not
// silently report success.
func TestAdminResolve_BanUser_NoResolvedOwner_FailsClosed(t *testing.T) {
	for _, targetType := range []string{
		codemeta.ReportTargetTypeReview.Value,
		codemeta.ReportTargetTypeChatMessage.Value,
		codemeta.ReportTargetTypePost.Value,
		codemeta.ReportTargetTypePostComment.Value,
		codemeta.ReportTargetTypeSellerForum.Value,
	} {
		t.Run(targetType, func(t *testing.T) {
			reportRepo := &fakeReportRepo{report: &gen.Report{
				ID: 1, Status: codemeta.ReportStatusPending.Value, TargetType: targetType, TargetID: 700001,
			}}
			banner := &fakeBanner{}
			uc := NewReportUseCase(reportRepo, nil, nil, banner, nil, func() int64 { return 42 })

			if _, err := uc.AdminResolve(context.Background(), 1, codemeta.ReportResolveActionBanUser.Value, 7, "x"); !errors.Is(err, ErrUnknownTargetOwner) {
				t.Fatalf("expected ErrUnknownTargetOwner, got %v", err)
			}
			if banner.called {
				t.Error("BanUser must not be dispatched when the owner is unknown")
			}
			if reportRepo.resolveCalled {
				t.Error("the report must stay PENDING (no status transition) so the admin can retry or pick another action")
			}
		})
	}
}

// TestAdminResolve_BanUser_LegacyUserReportFallsBackToTargetID covers the one
// target type where an unset target_owner_id is still unambiguous: for USER,
// ResolveOwner is the identity function, so target_id IS the owner.
func TestAdminResolve_BanUser_LegacyUserReportFallsBackToTargetID(t *testing.T) {
	reportRepo := &fakeReportRepo{report: &gen.Report{
		ID: 1, Status: codemeta.ReportStatusPending.Value, TargetType: codemeta.ReportTargetTypeUser.Value, TargetID: 8001,
	}}
	banner := &fakeBanner{}
	uc := NewReportUseCase(reportRepo, nil, nil, banner, nil, func() int64 { return 42 })

	if _, err := uc.AdminResolve(context.Background(), 1, codemeta.ReportResolveActionBanUser.Value, 7, "legacy row"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !banner.called || banner.lastUserID != 8001 {
		t.Errorf("expected BanUser(8001) from the USER target_id fallback, got called=%v id=%d", banner.called, banner.lastUserID)
	}
}

// TestAdminResolve_BanUser_DispatchFailureBlocksTransition mirrors the existing
// HIDE_TARGET retryable-error test: a downstream failure must not leave a report
// marked RESOLVED_BANNED while nobody is actually banned.
func TestAdminResolve_BanUser_DispatchFailureBlocksTransition(t *testing.T) {
	reportRepo := &fakeReportRepo{report: &gen.Report{
		ID: 1, Status: codemeta.ReportStatusPending.Value, TargetType: codemeta.ReportTargetTypePost.Value, TargetID: 700001,
		TargetOwnerID: nullableTestInt64(9001),
	}}
	sentinel := errors.New("account-service unavailable")
	banner := &fakeBanner{err: sentinel}
	uc := NewReportUseCase(reportRepo, nil, nil, banner, nil, func() int64 { return 42 })

	if _, err := uc.AdminResolve(context.Background(), 1, codemeta.ReportResolveActionBanUser.Value, 7, "x"); !errors.Is(err, sentinel) {
		t.Fatalf("expected the dispatch error to propagate, got %v", err)
	}
	if reportRepo.resolveCalled {
		t.Error("a failed ban must not transition the report")
	}
}

// TestAdminResolve_BanUser_AuditAffectedUserMatchesBannedID closes the loop the
// bug opened: the audit row already recorded target_owner_id as AffectedUserID
// while the ban itself used target_id, so the audit trail and the action
// disagreed about who was punished. They must now be the same number.
func TestAdminResolve_BanUser_AuditAffectedUserMatchesBannedID(t *testing.T) {
	auditRepo := &fakeAuditRepo{}
	reportRepo := &fakeReportRepo{report: &gen.Report{
		ID: 1, Status: codemeta.ReportStatusPending.Value, TargetType: codemeta.ReportTargetTypePost.Value, TargetID: 700001,
		TargetOwnerID: nullableTestInt64(9001),
	}}
	banner := &fakeBanner{}
	uc := NewReportUseCase(reportRepo, auditRepo, nil, banner, nil, func() int64 { return 42 })

	if _, err := uc.AdminResolve(context.Background(), 1, codemeta.ReportResolveActionBanUser.Value, 7, "spam"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if auditRepo.lastParams.AffectedUserID != banner.lastUserID {
		t.Errorf("audit AffectedUserID=%d but BanUser got %d — the audit trail must name the person actually banned",
			auditRepo.lastParams.AffectedUserID, banner.lastUserID)
	}
}

// TestAdminResolve_BanUser_FallbackOwner_AuditAffectedUserMatchesBannedID
// covers the exact corner the fix above closed: a USER-type report whose
// TargetOwnerID is NULL (so banTargetOwnerID falls back to TargetID). Before
// the fix, the audit block independently re-read report.TargetOwnerID —
// !Valid here — and recorded AffectedUserID=0 even though a real ban had just
// gone out to TargetID. This is the combination
// TestAdminResolve_BanUser_AuditAffectedUserMatchesBannedID did not cover,
// since that test uses a row with TargetOwnerID set.
func TestAdminResolve_BanUser_FallbackOwner_AuditAffectedUserMatchesBannedID(t *testing.T) {
	auditRepo := &fakeAuditRepo{}
	reportRepo := &fakeReportRepo{report: &gen.Report{
		ID: 1, Status: codemeta.ReportStatusPending.Value, TargetType: codemeta.ReportTargetTypeUser.Value, TargetID: 8001,
		// TargetOwnerID intentionally left zero-value (!Valid) — the legacy/
		// fallback case.
	}}
	banner := &fakeBanner{}
	uc := NewReportUseCase(reportRepo, auditRepo, nil, banner, nil, func() int64 { return 42 })

	if _, err := uc.AdminResolve(context.Background(), 1, codemeta.ReportResolveActionBanUser.Value, 7, "spam"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if banner.lastUserID != 8001 {
		t.Fatalf("expected BanUser(8001) from the USER target_id fallback, got %d", banner.lastUserID)
	}
	if auditRepo.lastParams.AffectedUserID != 8001 {
		t.Errorf("audit AffectedUserID=%d, want 8001 (the banned id) — a ban that happened must not be logged as affecting nobody",
			auditRepo.lastParams.AffectedUserID)
	}
}

func TestCreateAdminReport_SentinelReporterID(t *testing.T) {
	reportRepo := &fakeReportRepo{}
	uc := NewReportUseCase(reportRepo, nil, nil, nil, nil, func() int64 { return 42 })
	// invalid/unknown reason code falls back to OTHER (a takedown must not fail on a bad code)
	report, err := uc.CreateAdminReport(context.Background(), codemeta.ReportTargetTypePost.Value, 100, "NP", "spam post")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.ReporterID != 0 {
		t.Errorf("expected ReporterID=0 (sentinel, no end-user reporter), got %d", report.ReporterID)
	}
	if report.ReasonCode != codemeta.ReportReasonCodeOther.Value || report.Description == nil || *report.Description != "spam post" {
		t.Errorf("expected invalid code to fall back ReasonCode=%s Description='spam post', got %s/%v",
			codemeta.ReportReasonCodeOther.Value, report.ReasonCode, report.Description)
	}
	if reportRepo.lastParams.Evidence == nil {
		t.Error("expected non-nil Evidence (reports.evidence is JSONB NOT NULL); nil []byte marshals to SQL NULL")
	}
	// valid admin-chosen category is preserved verbatim
	report, err = uc.CreateAdminReport(context.Background(), codemeta.ReportTargetTypePost.Value, 100, codemeta.ReportReasonCodeFoodSafety.Value, "expired")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.ReasonCode != codemeta.ReportReasonCodeFoodSafety.Value {
		t.Errorf("expected valid ReasonCode=%s preserved, got %s", codemeta.ReportReasonCodeFoodSafety.Value, report.ReasonCode)
	}
	if reportRepo.lastParams.Evidence == nil {
		t.Error("expected non-nil Evidence (reports.evidence is JSONB NOT NULL); nil []byte marshals to SQL NULL")
	}
}

func TestSubmitReport_RateLimited(t *testing.T) {
	repo := &fakeReportRepo{countSince: reportRateLimitPerDay}
	uc := NewReportUseCase(repo, nil, nil, nil, nil, func() int64 { return 1 })
	_, err := uc.SubmitReport(context.Background(), domain.CreateReportParams{
		ReporterID: 1, TargetType: codemeta.ReportTargetTypePost.Value, TargetID: 2, ReasonCode: codemeta.ReportReasonCodeSpam.Value,
	})
	if !errors.Is(err, ErrReportRateLimited) {
		t.Fatalf("expected ErrReportRateLimited, got %v", err)
	}
}

func TestSubmitReport_InvalidReasonCode(t *testing.T) {
	uc := NewReportUseCase(&fakeReportRepo{}, nil, nil, nil, nil, func() int64 { return 1 })
	_, err := uc.SubmitReport(context.Background(), domain.CreateReportParams{
		// "ADULT" 刻意保持字面值：它是 social.proto 註解曾宣告、CHECK 從未接受的死值，
		// CodeMeta 裡沒有、也不該有對應常數。改成合法碼值等於把這支測試殺掉。
		ReporterID: 1, TargetType: codemeta.ReportTargetTypePost.Value, TargetID: 2, ReasonCode: "ADULT",
	})
	if !errors.Is(err, ErrInvalidReason) {
		t.Fatalf("expected ErrInvalidReason, got %v", err)
	}
}

// TestErrInvalidReason_And_ErrInvalidTargetType_AreDomainErrors guards
// against regressing to plain errors.New values: handler.go's ReportPost
// relies on pkgerrors.GetCode(err) to surface a structured {Code, Message}
// response instead of an opaque internal error, which only works if these
// are proper pkgerrors domain errors (see errors.go).
func TestErrInvalidReason_And_ErrInvalidTargetType_AreDomainErrors(t *testing.T) {
	if code := pkgerrors.GetCode(ErrInvalidReason); code != pkgerrors.CodeInvalidInput {
		t.Errorf("ErrInvalidReason: GetCode = %v, want CodeInvalidInput", code)
	}
	if code := pkgerrors.GetCode(ErrInvalidTargetType); code != pkgerrors.CodeInvalidInput {
		t.Errorf("ErrInvalidTargetType: GetCode = %v, want CodeInvalidInput", code)
	}
}

func TestReopen_TransitionsToAppealed(t *testing.T) {
	reportRepo := &fakeReportRepo{report: &gen.Report{ID: 1, Status: codemeta.ReportStatusAppealed.Value, TargetOwnerID: nullableTestInt64(999)}}
	auditRepo := &fakeAuditRepo{}
	uc := NewReportUseCase(reportRepo, auditRepo, nil, nil, nil, func() int64 { return 42 })

	row, err := uc.Reopen(context.Background(), 1, 7, "賣家申訴下架不當")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.Status != codemeta.ReportStatusAppealed.Value {
		t.Errorf("expected %s, got %s", codemeta.ReportStatusAppealed.Value, row.Status)
	}
	// REOPEN 必須來自 ModerationActionType（稽核動詞），不是 ReportResolveAction
	// ——後者只裝終態裁決。哪天兩個 enum 被合併，report_codemeta_test.go 的
	// TestResolveActionDomainExcludesAuditOnlyVerbs 會先紅。
	if auditRepo.lastParams.Action != codemeta.ModerationActionTypeReopen.Value {
		t.Errorf("expected %s action logged, got %s", codemeta.ModerationActionTypeReopen.Value, auditRepo.lastParams.Action)
	}
}

// TestSubmitReport_ReturnsExistingWithin24h exercises the 24h sliding dedup
// window: when GetByReporterAndTarget finds a prior report for the same
// reporter+target, SubmitReport must return that existing report as-is and
// must NOT create a new one. This is the headline behavior change for Task 5
// (permanent dedup -> 24h sliding), and lives in SubmitReport (not
// ReportPost) since that's where the actual dedup check runs — ReportPost
// now only delegates here. Placed in report_test.go rather than post_test.go
// for that reason.
func TestSubmitReport_ReturnsExistingWithin24h(t *testing.T) {
	sentinel := &gen.Report{ID: 999, ReporterID: 1, TargetType: codemeta.ReportTargetTypePost.Value, TargetID: 2, ReasonCode: codemeta.ReportReasonCodeSpam.Value, Status: codemeta.ReportStatusPending.Value}
	repo := &fakeReportRepo{existingReport: sentinel}
	uc := NewReportUseCase(repo, nil, nil, nil, nil, func() int64 { return 42 })

	got, err := uc.SubmitReport(context.Background(), domain.CreateReportParams{
		ReporterID: 1, TargetType: codemeta.ReportTargetTypePost.Value, TargetID: 2, ReasonCode: codemeta.ReportReasonCodeSpam.Value,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.ID != sentinel.ID {
		t.Fatalf("expected existing report (ID=%d) to be returned, got ID=%d", sentinel.ID, got.ID)
	}
	if repo.created {
		t.Fatalf("expected CreateGenericReport NOT to be called on dedup hit, but it was")
	}
}
