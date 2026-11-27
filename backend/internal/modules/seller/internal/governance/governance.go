// Package governance 是商家治理（ADR 2026-10-18-seller-governance-consolidated、step10e-reports-brief R2、R3、R7）的通報與案件：通報（檢舉與食安通報同一張表，一位檢舉人
// 的一次送出，含之後的補充）與它的佐證、分案、設定結果、移動，案件（同一家店、同一個平台判定類別，同時最多一個進行中的案件）的決定、改嚴重度與內部註記，
// 以及通報與措施上請求的佐證讀取。措施（警告、停權與終止合作）、認定與申覆在 measure 套件；治理的狀態改變在寫入的 transaction 內發模組事件，
// 送達由 A8 的 notification 負責。
//
// 鎖順序（單體規則 4 與 D3／D4 修訂 (c) 第 3 層的細化）：seller 內的全序是 identity 帳號列（需要時）→ 店家列（帳號全部的 stint，依 store_id，stint.LockAccountOf）→
// 通報列（依 id）→ 案件列（依 id）→ 措施列。改變案件成員或通報結果的指令都先鎖店家，所以同一家店的分案、移動、結案彼此串行（cases_one_open 只是保底）；
// 補充只鎖原通報、不鎖店家與案件（補充不改案件的版本）。
package governance

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Jobs   jobs.Enqueuer
	Media  model.Media
	// Identity 只在登記主管機關來函時驗證被指名的帳號存在。
	Identity Accounts
	Now      func() time.Time
	Logger   *slog.Logger
}

// Service 是治理的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service {
	if d.Logger == nil {
		d.Logger = slog.New(slog.DiscardHandler)
	}
	return &Service{d: d}
}

func (s *Service) now() time.Time { return s.d.Now().UTC().Truncate(time.Microsecond) }

func pageSize(size int32) (int32, error) {
	return pagination.PageSize(size, model.ReportDefaultPageSize, model.ReportMaxPageSize)
}

func inquiryPageSize(size int32) (int32, error) { return pagination.PageSize(size, 50, 100) }

func errReportNotFound() error { return apperr.NotFound("%s", text.ReportNotFound().Text) }

func errCaseNotFound() error { return apperr.NotFound("%s", text.CaseNotFound().Text) }

// 版本衝突的 ErrorInfo.metadata：哪一個物件過時（report 是請求帶的通報 etag，case 是案件指令帶的案件 etag，target_case 是分案或移動要加入的案件
// 版本前進或已關閉，category_case_opened 是要開新案件的類別剛有了進行中的案件）。client 依它顯示是通報、案件、要移入的案件剛被更新，還是類別剛有了案件
// （error_reason.proto 的 VERSION_CONFLICT、governance.proto 檔頭）。
const (
	ConflictTargetKey    = "conflictTarget"
	ConflictTargetReport = "report"
	ConflictTargetCase   = "case"
	// ConflictTargetTargetCase 是分案或移動要加入的案件不成立（它不是請求要操作的案件，而是通報要移入的案件）：版本前進或已關閉。
	ConflictTargetTargetCase = "target_case"
	// ConflictTargetCategoryCaseOpened 是分案或移動要開新案件，而這家店這個類別剛有了進行中的案件（管理員沒看過那個案件）。
	ConflictTargetCategoryCaseOpened = "category_case_opened"
)

func errReportVersion() error {
	return apperr.VersionConflict("%s", text.ReportVersionConflict().Text).WithMetadata(ConflictTargetKey, ConflictTargetReport)
}

func errCaseVersion() error {
	return apperr.VersionConflict("%s", text.CaseVersionConflict().Text).WithMetadata(ConflictTargetKey, ConflictTargetCase)
}

func errTargetCaseVersion() error {
	return apperr.VersionConflict("%s", text.CaseTargetVersionConflict().Text).WithMetadata(ConflictTargetKey, ConflictTargetTargetCase)
}

func errCategoryCaseOpened() error {
	return apperr.VersionConflict("%s", text.CaseCategoryCaseOpened().Text).WithMetadata(ConflictTargetKey, ConflictTargetCategoryCaseOpened)
}

func reportBlocked(blocks []model.ReportBlock) error {
	if len(blocks) == 0 {
		return nil
	}
	return apperr.FailedPrecondition("%s", blocks[0].Text().Text)
}

func caseBlocked(blocks []model.CaseBlock, action model.CaseAction) error {
	if len(blocks) == 0 {
		return nil
	}
	return apperr.FailedPrecondition("%s", blocks[0].Text(action).Text)
}

func invalidOn(field string, m interface{ String() string }) error {
	return apperr.InvalidInputOn([]string{field}, "%s", m.String())
}

func toReport(row store.SellerReport) model.Report {
	r := model.Report{
		ID: row.ID, StoreID: row.StoreID, ProductID: row.ProductID, OrderNo: row.OrderNo,
		Category: policy.Category(row.Category), Description: row.Description, ReporterBuyerID: row.ReporterBuyerID,
		CaseID: row.CaseID, TriagedAt: row.TriagedAt, TriagedByAdminID: row.TriagedByAdminID,
		ResolvedAt: row.ResolvedAt, ResolvedByAdminID: row.ResolvedByAdminID, ResolutionNote: row.ResolutionNote,
		SupplementsSeenSeq: int(row.SupplementsSeenSeq), Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.AssessedCategory != nil {
		category := policy.Category(*row.AssessedCategory)
		r.AssessedCategory = &category
	}
	if row.Outcome != nil {
		outcome := model.Outcome(*row.Outcome)
		r.Outcome = &outcome
	}
	return r
}

type reportRow struct {
	report              store.SellerReport
	storeName           *string
	storeClosed         bool
	supplementCount     int32
	latestSupplementSeq int32
	latestSupplementAt  *time.Time
}

func timeOf(v any) *time.Time {
	if t, ok := v.(time.Time); ok {
		return &t
	}
	return nil
}

func (r reportRow) toReport() model.Report {
	out := toReport(r.report)
	out.StoreName, out.SupplementCount, out.LatestSupplementSeq, out.LatestSupplementAt = r.storeName, int(r.supplementCount), int(r.latestSupplementSeq), r.latestSupplementAt
	return out
}

func toCase(row store.SellerCase, storeName *string) model.Case {
	c := model.Case{
		ID: row.ID, StoreID: row.StoreID, StoreName: storeName, Category: policy.Category(row.Category), Severity: model.Severity(row.Severity),
		Version: row.Version, OpenedAt: row.OpenedAt, OpenedByAdminID: row.OpenedByAdminID, ClosedAt: row.ClosedAt, ClosedByAdminID: row.ClosedByAdminID,
		DecisionNote: row.DecisionNote, UpdatedAt: row.UpdatedAt,
	}
	if row.CloseReason != nil {
		reason := model.CaseCloseReason(*row.CloseReason)
		c.CloseReason = &reason
	}
	if row.Decision != nil {
		decision := model.Decision(*row.Decision)
		c.Decision = &decision
	}
	return c
}

type caseRow struct {
	row         store.SellerCase
	storeName   *string
	storeClosed bool
	storePurged bool
}

func caseFacts(ctx context.Context, q *store.Queries, rows []caseRow) (map[uuid.UUID]model.CaseFacts, error) {
	out := make(map[uuid.UUID]model.CaseFacts, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.row.ID
		out[r.row.ID] = model.CaseFacts{Case: toCase(r.row, r.storeName), StoreClosed: r.storeClosed, StorePurged: r.storePurged}
	}
	counts, err := q.CaseReportCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, c := range counts {
		f := out[c.CaseID]
		f.Case.ReportCount, f.Case.UnresolvedReportCount, f.Case.UnseenSupplementReports = int(c.ReportCount), int(c.UnresolvedCount), int(c.UnseenSupplementReports)
		f.ActionTakenReports = int(c.ActionTakenCount)
		out[c.CaseID] = f
	}
	measures, err := q.CaseMeasureCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, m := range measures {
		if m.CaseID == nil {
			continue
		}
		f := out[*m.CaseID]
		f.ResultingMeasures, f.UnrevokedMeasures = int(m.ResultingMeasures), int(m.UnrevokedMeasures)
		out[*m.CaseID] = f
	}
	open, err := q.OpenCaseInquiries(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, i := range open {
		f := out[i.CaseID]
		id := i.ID
		f.Case.OpenInquiryID = &id
		out[i.CaseID] = f
	}
	return out, nil
}

func caseFactsByIDs(ctx context.Context, q *store.Queries, ids []uuid.UUID) (map[uuid.UUID]model.CaseFacts, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]model.CaseFacts{}, nil
	}
	rows, err := q.CasesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	cases := make([]caseRow, len(rows))
	for i, r := range rows {
		cases[i] = caseRow{row: r.SellerCase, storeName: r.StoreName, storeClosed: r.StoreClosed, storePurged: r.StorePurged}
	}
	return caseFacts(ctx, q, cases)
}

func adminCases(rows []caseRow, facts map[uuid.UUID]model.CaseFacts, now time.Time) []model.AdminCase {
	out := make([]model.AdminCase, len(rows))
	for i, r := range rows {
		f := facts[r.row.ID]
		out[i] = model.AdminCase{Case: f.Case, Availability: f.Availability()}
	}
	return out
}

func adminReports(ctx context.Context, q *store.Queries, rows []reportRow, detail bool, now time.Time) ([]model.AdminReport, error) {
	reports := make([]model.Report, len(rows))
	reportIDs := make([]uuid.UUID, len(rows))
	var caseIDs []uuid.UUID
	for i, r := range rows {
		reports[i] = r.toReport()
		reportIDs[i] = r.report.ID
		if r.report.CaseID != nil && !slices.Contains(caseIDs, *r.report.CaseID) {
			caseIDs = append(caseIDs, *r.report.CaseID)
		}
	}
	cases, err := caseFactsByIDs(ctx, q, caseIDs)
	if err != nil {
		return nil, err
	}
	sources := map[uuid.UUID]bool{}
	if len(reportIDs) > 0 {
		ids, err := q.ReportFindingSources(ctx, reportIDs)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			sources[id] = true
		}
	}
	if detail {
		if err := loadReportDetail(ctx, q, reportIDs, reports); err != nil {
			return nil, err
		}
	}
	if err := loadTriageDeadlines(ctx, q, now, reportIDs, reports); err != nil {
		return nil, err
	}
	out := make([]model.AdminReport, len(rows))
	for i, row := range rows {
		r := reports[i]
		facts := model.ReportFacts{Report: r, StoreClosed: row.storeClosed, FindingSource: sources[r.ID]}
		if r.CaseID != nil {
			if c, ok := cases[*r.CaseID]; ok {
				facts.Case = &c
			}
		}
		out[i] = model.AdminReport{Report: r, Availability: facts.Availability()}
	}
	return out, nil
}

func loadTriageDeadlines(ctx context.Context, q *store.Queries, now time.Time, ids []uuid.UUID, reports []model.Report) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := q.ReviewReportTriageByIDs(ctx, store.ReviewReportTriageByIDsParams{At: now, Ids: ids})
	if err != nil {
		return err
	}
	byID := make(map[uuid.UUID]*model.Deadline, len(rows))
	for _, r := range rows {
		byID[r.ReportID] = &model.Deadline{Start: r.StartAt, Due: r.DueAt}
	}
	for i := range reports {
		reports[i].TriageDeadline = byID[reports[i].ID]
	}
	return nil
}

func loadCaseDeadlines(ctx context.Context, q *store.Queries, now time.Time, facts map[uuid.UUID]model.CaseFacts) error {
	if len(facts) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(facts))
	for id := range facts {
		ids = append(ids, id)
	}
	decide, err := q.ReviewCaseDecisionByIDs(ctx, store.ReviewCaseDecisionByIDsParams{At: now, Ids: ids})
	if err != nil {
		return err
	}
	for _, r := range decide {
		f := facts[r.CaseID]
		f.Case.DecideDeadline = &model.Deadline{Start: r.StartAt, Due: r.DueAt}
		facts[r.CaseID] = f
	}
	return nil
}

func loadReportDetail(ctx context.Context, q *store.Queries, ids []uuid.UUID, reports []model.Report) error {
	if len(ids) == 0 {
		return nil
	}
	supplements, err := q.ReportSupplements(ctx, ids)
	if err != nil {
		return err
	}
	evidence, err := q.ReportEvidenceOf(ctx, ids)
	if err != nil {
		return err
	}
	byReport := make(map[uuid.UUID]int, len(ids))
	for i, id := range ids {
		byReport[id] = i
	}
	for _, row := range supplements {
		i := byReport[row.ReportID]
		supplement := model.Supplement{ID: row.ID, Description: row.Description, CreatedAt: row.CreatedAt}
		reports[i].Supplements = append(reports[i].Supplements, supplement)
	}
	for _, row := range evidence {
		i := byReport[row.ReportID]
		reports[i].Evidence = append(reports[i].Evidence, model.Evidence{
			AssetID: row.AssetID, ObjectKey: row.ObjectKey, Position: len(reports[i].Evidence), SupplementID: row.SupplementID,
		})
	}
	for i := range reports {
		r := &reports[i]
		index := make(map[uuid.UUID]int, len(r.Supplements))
		for j, supplement := range r.Supplements {
			index[supplement.ID] = j
		}
		for _, e := range r.Evidence {
			if e.SupplementID != nil {
				if j, ok := index[*e.SupplementID]; ok {
					r.Supplements[j].EvidencePositions = append(r.Supplements[j].EvidencePositions, e.Position)
				}
			}
		}
	}
	return nil
}

func lockReport(ctx context.Context, q *store.Queries, reportID uuid.UUID, expected int64) (store.SellerReport, error) {
	row, err := q.LockReport(ctx, reportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, errReportNotFound()
	}
	if err != nil {
		return row, err
	}
	if row.Version != expected {
		return row, errReportVersion()
	}
	return row, nil
}

func readReport(ctx context.Context, q *store.Queries, reportID uuid.UUID, now time.Time) (model.AdminReport, error) {
	rows, err := q.ReportDetails(ctx, []uuid.UUID{reportID})
	if err != nil {
		return model.AdminReport{}, err
	}
	if len(rows) == 0 {
		return model.AdminReport{}, errReportNotFound()
	}
	d := rows[0]
	views, err := adminReports(ctx, q, []reportRow{{
		report: d.SellerReport, storeName: d.StoreName, storeClosed: d.StoreClosed, supplementCount: d.SupplementCount, latestSupplementSeq: d.LatestSupplementSeq, latestSupplementAt: timeOf(d.LatestSupplementAt),
	}}, true, now)
	if err != nil {
		return model.AdminReport{}, err
	}
	view := views[0]
	if len(view.Availability.Triage) == 0 || len(view.Availability.Move) == 0 {
		if view.Targets, err = triageTargets(ctx, q, view.Report, len(view.Availability.Triage) == 0); err != nil {
			return model.AdminReport{}, err
		}
	}
	return view, nil
}

func triageTargets(ctx context.Context, q *store.Queries, r model.Report, triage bool) ([]model.TriageTarget, error) {
	open, err := q.OpenCasesOfStore(ctx, r.StoreID)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(open))
	for i, c := range open {
		ids[i] = c.ID
	}
	facts, err := caseFactsByIDs(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	byCategory := make(map[string]model.Case, len(open))
	for _, c := range open {
		byCategory[c.Category] = facts[c.ID].Case
	}
	var out []model.TriageTarget
	for _, category := range policy.Assessable() {
		if !triage && r.AssessedCategory != nil && *r.AssessedCategory == category {
			continue
		}
		target := model.TriageTarget{Category: category}
		if c, ok := byCategory[string(category)]; ok {
			target.Case = &c
		}
		out = append(out, target)
	}
	return out, nil
}

func readCase(ctx context.Context, q *store.Queries, caseID uuid.UUID, now time.Time) (model.AdminCase, error) {
	facts, err := caseFactsByIDs(ctx, q, []uuid.UUID{caseID})
	if err != nil {
		return model.AdminCase{}, err
	}
	f, ok := facts[caseID]
	if !ok {
		return model.AdminCase{}, errCaseNotFound()
	}
	if err := loadCaseDeadlines(ctx, q, now, facts); err != nil {
		return model.AdminCase{}, err
	}
	f = facts[caseID]
	moves, err := q.CaseMoves(ctx, caseID)
	if err != nil {
		return model.AdminCase{}, err
	}
	f.Case.Moves = make([]model.ReportMove, len(moves))
	for i, m := range moves {
		f.Case.Moves[i] = model.ReportMove{ReportID: m.ReportID, FromCaseID: m.FromCaseID, ToCaseID: m.ToCaseID, AdminID: m.AdminID, MovedAt: m.MovedAt}
	}
	return model.AdminCase{Case: f.Case, Availability: f.Availability()}, nil
}

func reportFactsOf(ctx context.Context, q *store.Queries, report store.SellerReport, storeClosed bool) (model.ReportFacts, error) {
	facts := model.ReportFacts{Report: toReport(report), StoreClosed: storeClosed}
	sources, err := q.ReportFindingSources(ctx, []uuid.UUID{report.ID})
	if err != nil {
		return facts, err
	}
	facts.FindingSource = len(sources) > 0
	if report.CaseID != nil {
		cases, err := caseFactsByIDs(ctx, q, []uuid.UUID{*report.CaseID})
		if err != nil {
			return facts, err
		}
		if c, ok := cases[*report.CaseID]; ok {
			facts.Case = &c
		}
	}
	return facts, nil
}

func validAssessed(category policy.Category) error {
	switch {
	case category == policy.Other:
		return invalidOn("AssessedCategory", text.TriageCategoryNotAssessable())
	case !category.Assessable():
		return invalidOn("AssessedCategory", text.TriageCategoryRequired())
	}
	return nil
}

func validSeverity(severity model.Severity) error {
	if !slices.Contains(model.SeverityValues(), severity) {
		return invalidOn("Severity", text.SeverityRequired())
	}
	return nil
}
