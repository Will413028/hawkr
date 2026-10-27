package postgres

import (
	"context"
	"time"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/social-service/internal/domain"
	"github.com/Will413028/hawkr/social-service/internal/repository/postgres/gen"
	"github.com/jackc/pgx/v5/pgxpool"
)

// reports.sql 不寫碼值字面值（沿 posts.sql 的先例），值域由這裡帶入。三份清單各自
// 是一條**業務述詞**，不是某個 enum 的全量展開——照著 AllXxx 塞會靜默改變語意，見
// priorOffenceActions 的註解。三者與 CodeMeta 的關係由
// report_repository_codemeta_test.go 釘住。
var (
	// 可裁決的起始狀態：PENDING（還沒人看過）與 APPEALED（處置過但被申訴重開）。
	resolvableStatuses = []string{
		codemeta.ReportStatusPending.Value,
		codemeta.ReportStatusAppealed.Value,
	}

	// 可被重開的終態，即 actionToStatus 的四個輸出。
	reopenableStatuses = []string{
		codemeta.ReportStatusResolvedHidden.Value,
		codemeta.ReportStatusResolvedWarned.Value,
		codemeta.ReportStatusResolvedBanned.Value,
		codemeta.ReportStatusDismissed.Value,
	}

	// 「算不算前科」的述詞：ReportResolveAction 的真子集，刻意排除 DISMISS。駁回
	// 代表檢舉不成立，把它計入累犯次數會讓被誣告的人在管理員眼裡變成慣犯。
	priorOffenceActions = []string{
		codemeta.ReportResolveActionHideTarget.Value,
		codemeta.ReportResolveActionWarnUser.Value,
		codemeta.ReportResolveActionBanUser.Value,
	}
)

type ReportRepository struct {
	q *gen.Queries
}

func NewReportRepository(pool *pgxpool.Pool) *ReportRepository {
	return &ReportRepository{q: gen.New(pool)}
}

// GetReport keeps the legacy post-only signature used by ReportPost.
// It looks up a report on a POST target by reporter (24h idempotency is
// enforced by the unified usecase via GetReportByReporterAndTarget).
func (r *ReportRepository) GetReport(ctx context.Context, postID, reporterID int64) (*gen.Report, error) {
	row, err := r.q.GetPostReport(ctx, gen.GetPostReportParams{
		TargetID:   postID,
		ReporterID: reporterID,
		TargetType: codemeta.ReportTargetTypePost.Value,
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// CreateReport keeps the legacy post-only signature; it inserts into the
// unified `reports` table with target_type='POST'.
func (r *ReportRepository) CreateReport(ctx context.Context, id, postID, reporterID int64, reasonCode string, description string) (*gen.Report, error) {
	row, err := r.q.CreatePostReport(ctx, gen.CreatePostReportParams{
		ID:            id,
		ReporterID:    reporterID,
		TargetID:      postID,
		TargetOwnerID: nil,
		ReasonCode:    reasonCode,
		Description:   nullableText(description),
		TargetType:    codemeta.ReportTargetTypePost.Value,
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// GetByReporterAndTarget enforces 24h idempotency — returns most recent
// report by the same reporter on the same target within 24h, if any.
func (r *ReportRepository) GetByReporterAndTarget(ctx context.Context, reporterID int64, targetType string, targetID int64) (*gen.Report, error) {
	row, err := r.q.GetReportByReporterAndTarget(ctx, gen.GetReportByReporterAndTargetParams{
		ReporterID: reporterID,
		TargetType: targetType,
		TargetID:   targetID,
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *ReportRepository) CreateGenericReport(ctx context.Context, p domain.CreateReportParams) (*gen.Report, error) {
	row, err := r.q.CreateReport(ctx, gen.CreateReportParams{
		ID:             p.ID,
		ReporterID:     p.ReporterID,
		TargetType:     p.TargetType,
		TargetID:       p.TargetID,
		TargetOwnerID:  nullableInt64(p.TargetOwnerID),
		ReasonCode:     p.ReasonCode,
		Description:    nullableText(p.Description),
		Evidence:       p.Evidence,
		TargetSnapshot: p.TargetSnapshot,
	})
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *ReportRepository) ListByReporter(ctx context.Context, reporterID int64, limit, offset int64) ([]*gen.Report, error) {
	rows, err := r.q.ListReportsByReporter(ctx, gen.ListReportsByReporterParams{
		ReporterID: reporterID,
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*gen.Report, len(rows))
	for i := range rows {
		row := rows[i]
		out[i] = &row
	}
	return out, nil
}

func (r *ReportRepository) AdminList(ctx context.Context, status, targetType, reasonCode string, limit, offset int64) ([]*gen.Report, error) {
	rows, err := r.q.AdminListReports(ctx, gen.AdminListReportsParams{
		Column1: status,
		Column2: targetType,
		Column3: reasonCode,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*gen.Report, len(rows))
	for i := range rows {
		row := rows[i]
		out[i] = &row
	}
	return out, nil
}

func (r *ReportRepository) AdminGet(ctx context.Context, id int64) (*gen.Report, error) {
	row, err := r.q.AdminGetReport(ctx, id)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

func (r *ReportRepository) AdminResolve(ctx context.Context, id int64, status, action string, resolvedBy int64) (*gen.Report, error) {
	row, err := r.q.AdminResolveReport(ctx, gen.AdminResolveReportParams{
		ID:                 id,
		Status:             status,
		ResolvedAction:     nullableText(action),
		ResolvedBy:         nullableInt64(resolvedBy),
		ResolvableStatuses: resolvableStatuses,
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

// CountByReporterSince counts reports filed by reporterID since the given
// time — used by SubmitReport rate limiting (task 4).
func (r *ReportRepository) CountByReporterSince(ctx context.Context, reporterID int64, since time.Time) (int64, error) {
	return r.q.CountReportsByReporterSince(ctx, gen.CountReportsByReporterSinceParams{
		ReporterID: reporterID,
		CreateTime: timestamptzParam(since),
	})
}

// Reopen transitions a resolved report to APPEALED (admin appeal flow).
// APPEALED, not PENDING: the two are deliberately distinct states — PENDING is
// "nobody has looked at this yet", APPEALED is "somebody ruled and the ruling is
// being contested" — and collapsing them would bury reopened cases in the unread
// queue.
func (r *ReportRepository) Reopen(ctx context.Context, id int64) (*gen.Report, error) {
	row, err := r.q.ReopenReport(ctx, gen.ReopenReportParams{
		ID:                 id,
		AppealedStatus:     codemeta.ReportStatusAppealed.Value,
		ReopenableStatuses: reopenableStatuses,
	})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &row, nil
}

// CountRecentActionsByOwner counts resolved-with-action reports against
// ownerID in the trailing window (see CountRecentActionsByOwner SQL) — used
// for escalation heuristics (repeat-offender detection).
func (r *ReportRepository) CountRecentActionsByOwner(ctx context.Context, ownerID int64) (int64, error) {
	return r.q.CountRecentActionsByOwner(ctx, gen.CountRecentActionsByOwnerParams{
		TargetOwnerID:       nullableInt64(ownerID),
		PriorOffenceActions: priorOffenceActions,
	})
}
