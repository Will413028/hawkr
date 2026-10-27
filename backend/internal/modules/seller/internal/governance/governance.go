// Package governance 是商家治理（ADR 2026-10-18-seller-governance-consolidated、step10e-reports-brief R2、R3、R7）的通報與案件：通報（檢舉與食安通報同一張表，一位檢舉人
// 的一次送出，含之後的補充）與它的佐證、分案、設定結果、移動，案件（同一家店、同一個平台判定類別，同時最多一個進行中的案件）的決定、改嚴重度、轉報主管機關與內部註記，
// 匿名聯絡欄位的保留期限清理，以及通報與措施上請求的佐證讀取。措施（警告、停權與終止合作）、認定與申覆在 measure 套件；治理的狀態改變在寫入的 transaction 內發模組事件，
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

	"github.com/Will413028/hawkr/backend/internal/modules/media"
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
	Media  media.Media
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

// now 是寫入用的現在：截到微秒（Postgres 的 timestamptz 精度），之後與已存的時間比較才不會被奈秒的差異誤判。
func (s *Service) now() time.Time { return s.d.Now().UTC().Truncate(time.Microsecond) }

func pageSize(size int32) (int32, error) { return pagination.PageSize(size, 20, 100) }

// inquiryPageSize 是詢問清單的每頁筆數：預設 50，上限 100（一個案件的詢問不多，通常一頁讀完）。
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

// reportBlocked 是通報不能做動作的錯誤：訊息是第一個原因的句子（API 的 can_* 看同一份原因）；沒有原因回 nil。
func reportBlocked(blocks []model.ReportBlock) error {
	if len(blocks) == 0 {
		return nil
	}
	return apperr.FailedPrecondition("%s", blocks[0].Text().Text)
}

// caseBlocked 是案件不能做動作的錯誤：訊息是第一個原因的句子；沒有原因回 nil。
func caseBlocked(blocks []model.CaseBlock, action model.CaseAction) error {
	if len(blocks) == 0 {
		return nil
	}
	return apperr.FailedPrecondition("%s", blocks[0].Text(action).Text)
}

// invalidOn 是指向欄位 field 的 invalid_argument，訊息是模組文字。
func invalidOn(field string, m interface{ String() string }) error {
	return apperr.InvalidInputOn([]string{field}, "%s", m.String())
}

func toReport(row store.SellerReport) model.Report {
	r := model.Report{
		ID: row.ID, StoreID: row.StoreID, ProductID: row.ProductID, OrderNo: row.OrderNo,
		Category: policy.Category(row.Category), Description: row.Description, ReporterBuyerID: row.ReporterBuyerID,
		ContactErasedAt: row.ContactErasedAt, CaseID: row.CaseID, TriagedAt: row.TriagedAt, TriagedByAdminID: row.TriagedByAdminID,
		ResolvedAt: row.ResolvedAt, ResolvedByAdminID: row.ResolvedByAdminID, ResolutionNote: row.ResolutionNote,
		DiscardedSubmissions: int(row.DiscardedSubmissions), SupplementsSeenSeq: int(row.SupplementsSeenSeq), Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.AssessedCategory != nil {
		category := policy.Category(*row.AssessedCategory)
		r.AssessedCategory = &category
	}
	if row.Outcome != nil {
		outcome := model.Outcome(*row.Outcome)
		r.Outcome = &outcome
	}
	if row.ReporterBuyerID == nil && row.ContactErasedAt == nil {
		r.Contact = &model.ReporterContact{Phone: row.ReporterPhone, Email: row.ReporterEmail, Name: row.ReporterName}
	}
	return r
}

// reportRow 是一筆通報加上讀取時要的店家與補充摘要（ReportDetails、AdminReports、AdminReportsQueue 三支查詢的共同形狀）。
type reportRow struct {
	report              store.SellerReport
	storeName           *string
	storeClosed         bool
	supplementCount     int32
	latestSupplementSeq int32
	latestSupplementAt  *time.Time
}

// timeOf 把 sqlc 對 max(timestamptz) 的結果（interface{}：它不知道欄位會不會是 NULL）轉成時間；NULL 是 nil。
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

// toReferral 把轉報列轉成模型：已轉報帶管道、案號與追蹤日期（回覆由 caseFacts 另載入），不轉報帶原因。CHECK 保證 REFERRED 有管道、WAIVED 有原因。
func toReferral(row store.SellerCaseReferral) *model.CaseReferral {
	out := &model.CaseReferral{Outcome: model.ReferralOutcome(row.Outcome)}
	if out.Waived() {
		out.Waiver = &model.ReferralWaiver{Reason: deref(row.WaiverReason), WaivedByAdminID: row.RecordedByAdminID, WaivedAt: row.RecordedAt}
		return out
	}
	out.FDA = &model.FDAReferral{
		ReportedAt: row.RecordedAt, Channel: model.FDAChannel(deref(row.Channel)), ReportedByAdminID: row.RecordedByAdminID, FollowUpOn: row.FollowUpOn,
		FollowUpSetAt: row.FollowUpSetAt,
	}
	if row.ReferenceNo != nil {
		out.FDA.ReferenceNo = *row.ReferenceNo
	}
	return out
}

// caseRow 是一個案件加上店名、店家是否已關閉與個資是否已清理（CasesByIDs、AdminCases 兩支查詢的共同形狀）。
type caseRow struct {
	row         store.SellerCase
	storeName   *string
	storeClosed bool
	storePurged bool
}

// caseFacts 讀一批案件的事實：成員統計（成員數、沒有結果的、已採取措施的、有補充還沒看過的）、導出的措施數（案件導出的措施的定義在 CaseMeasureCounts）、轉報紀錄
// （Case.Referral：案件的可用性看它，通報的移動也看它——有轉報紀錄的案件，最後一筆通報不能移走）與詢問（還沒結束的那一筆、「待決定」重新起算的時間）。
// 每批固定七次查詢以內；同一個讀取快照或指令裡鎖住案件之後呼叫。
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
	referrals, err := q.CaseReferrals(ctx, ids)
	if err != nil {
		return nil, err
	}
	var referred []uuid.UUID
	for _, r := range referrals {
		f := out[r.CaseID]
		f.Case.Referral = toReferral(r)
		out[r.CaseID] = f
		if f.Case.Referral.Referred() {
			referred = append(referred, r.CaseID)
		}
	}
	if len(referred) > 0 {
		replies, err := q.CaseReferralReplies(ctx, referred)
		if err != nil {
			return nil, err
		}
		for _, r := range replies {
			fda := out[r.CaseID].Case.Referral.FDA
			fda.Replies = append(fda.Replies, model.FDAReferralReply{
				Body: r.Body, ReceivedOn: r.ReceivedOn, RecordedByAdminID: r.RecordedByAdminID, RecordedAt: r.RecordedAt,
			})
		}
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

// caseFactsByIDs 讀一批案件（CasesByIDs）的事實；不存在的 ID 不在結果裡。
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

// adminCases 把案件的事實變成後台的檢視（含此刻可以做的動作）。
func adminCases(rows []caseRow, facts map[uuid.UUID]model.CaseFacts, now time.Time) []model.AdminCase {
	out := make([]model.AdminCase, len(rows))
	for i, r := range rows {
		f := facts[r.row.ID]
		out[i] = model.AdminCase{Case: f.Case, Availability: f.Availability(), ReadAt: now}
	}
	return out
}

// adminReports 把一批通報變成後台的檢視（含此刻可以做的動作）。事實固定五次查詢以內（所在案件的事實、認定的來源），與清單的大小無關；detail 時另載入補充與佐證（單筆讀取與匯出）。
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

// loadTriageDeadlines 一次讀出這批通報的分案期限（TriageDeadline）。成員、開始與期限只在 seller.review_report_triage(at)（db/objects.sql）；
// 不是待分案的通報不在函式的結果裡，維持 nil。reports 與 ids 同序。
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

// loadCaseDeadlines 一次讀出這批案件的決定期限與轉報期限（DecideDeadline、FDAReferralDeadline），填進 facts。成員、開始與期限只在
// seller.review_case_decision(at)、seller.review_case_fda_referral(at)（db/objects.sql）；義務不在時維持 nil。
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
	referral, err := q.ReviewCaseFDAReferralByIDs(ctx, store.ReviewCaseFDAReferralByIDsParams{At: now, Ids: ids})
	if err != nil {
		return err
	}
	for _, r := range referral {
		f := facts[r.CaseID]
		f.Case.FDAReferralDeadline = &model.Deadline{Start: r.StartAt, Due: r.DueAt}
		facts[r.CaseID] = f
	}
	return nil
}

// loadReportDetail 載入一批通報的補充與佐證（原通報與補充的佐證依送出的順序編號；每則補充帶它的佐證序號）。reports 與 ids 同序。
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
	bySupplement := map[uuid.UUID]*model.Supplement{}
	for _, row := range supplements {
		i := byReport[row.ReportID]
		supplement := model.Supplement{ID: row.ID, Description: row.Description, CreatedAt: row.CreatedAt}
		if row.MatchedBy != nil {
			matched := model.SupplementMatch(*row.MatchedBy)
			supplement.MatchedBy = &matched
		}
		reports[i].Supplements = append(reports[i].Supplements, supplement)
		bySupplement[row.ID] = &reports[i].Supplements[len(reports[i].Supplements)-1]
	}
	for _, row := range evidence {
		// 「第 n 張」依送出的順序與次內位置編號（查詢已排序），是讀取時算的序號，不存。
		i := byReport[row.ReportID]
		reports[i].Evidence = append(reports[i].Evidence, model.Evidence{
			AssetID: row.AssetID, ObjectKey: row.ObjectKey, Position: len(reports[i].Evidence), SupplementID: row.SupplementID,
		})
	}
	// 補充的佐證位置要等 Supplements 的切片不再成長之後才能取指標，所以分兩輪。
	for i := range reports {
		index := make(map[uuid.UUID]int, len(reports[i].Supplements))
		for j, supplement := range reports[i].Supplements {
			index[supplement.ID] = j
		}
		for _, e := range reports[i].Evidence {
			if e.SupplementID != nil {
				if j, ok := index[*e.SupplementID]; ok {
					reports[i].Supplements[j].EvidencePositions = append(reports[i].Supplements[j].EvidencePositions, e.Position)
				}
			}
		}
	}
	return nil
}

// lockReport 鎖住通報並比對版本：版本不符回版本衝突（版本與狀態都不符時先回版本衝突）。
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

// readReport 讀一筆通報的後台檢視（含補充、佐證與此刻可以做的動作）；在 q 的連線上執行，所以可以在讀取快照裡，也可以在指令的 transaction 裡讀自己寫的。
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

// triageTargets 是通報分案或移動時每個平台判定類別的去處：這家店那個類別進行中的案件（加入它），沒有就是開新案件。每個可判定的類別一筆（順序同政策分類），
// 移動時不含目前的類別。不鎖就讀，client 把它放進請求的意圖，由指令在鎖內驗證仍然成立（joinCase）。
func triageTargets(ctx context.Context, q *store.Queries, r model.Report, triage bool) ([]model.TriageTarget, error) {
	open, err := q.OpenCasesOfStore(ctx, r.StoreID)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(open))
	for i, c := range open {
		ids[i] = c.ID
	}
	// 帶成員數的案件（說明「會加入哪個案件」）；不鎖。
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

// readCase 讀一個案件的後台檢視（含移動紀錄、轉報紀錄與此刻可以做的動作）；在 q 的連線上執行，同 readReport。
// now 是讀取的時間：讀取時推導的欄位（轉報的追蹤逾期）以它判斷。
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
	return model.AdminCase{Case: f.Case, Availability: f.Availability(), ReadAt: now}, nil
}

// facts 組出一筆已鎖住的通報的事實：店家是否已關閉、是不是認定的來源、所在案件的事實（案件已鎖住）。
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

// validAssessed 驗證平台判定類別：必須選、而且在 Assessable 的子集（OTHER 只給檢舉人選，分案時要改判成有依據的類別，或直接設定結果）。
func validAssessed(category policy.Category) error {
	switch {
	case category == policy.Other:
		return invalidOn("AssessedCategory", text.TriageCategoryNotAssessable())
	case !category.Assessable():
		return invalidOn("AssessedCategory", text.TriageCategoryRequired())
	}
	return nil
}

// validSeverity 驗證嚴重度是合法的值。
func validSeverity(severity model.Severity) error {
	if !slices.Contains(model.SeverityValues(), severity) {
		return invalidOn("Severity", text.SeverityRequired())
	}
	return nil
}
