package governance

import (
	"context"
	"errors"
	"slices"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

// lockCase 鎖住案件並比對版本（expected 為 0 表示不比對，給不帶 etag 的指令）：版本不符回版本衝突。
func lockCase(ctx context.Context, q *store.Queries, caseID uuid.UUID, expected int64) (store.SellerCase, error) {
	row, err := q.LockCase(ctx, caseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, errCaseNotFound()
	}
	if err != nil {
		return row, err
	}
	if expected != 0 && row.Version != expected {
		return row, errCaseVersion()
	}
	return row, nil
}

// caseFactsOf 組出一個已鎖住的案件的事實（成員統計、導出的措施、店家是否已關閉）。
func caseFactsOf(ctx context.Context, q *store.Queries, caseID uuid.UUID) (model.CaseFacts, error) {
	facts, err := caseFactsByIDs(ctx, q, []uuid.UUID{caseID})
	if err != nil {
		return model.CaseFacts{}, err
	}
	f, ok := facts[caseID]
	if !ok {
		return model.CaseFacts{}, errCaseNotFound()
	}
	return f, nil
}

// Decide 決定案件（採取措施或不採取措施），決定即結案，結案是終態：結案時還沒有結果的通報帶入決定的預設結果（採取措施是已採取措施，不採取措施是未違規）與給檢舉人的說明，
// 每筆帶入的通報各發一個 ReportResolved；已經有結果的通報不受影響；還沒結束的詢問由系統撤回（CASE_CLOSED）。決定必須在 CaseAvailability.Decide 裡可選：「採取措施」要有導出的措施（不論狀態）或已有成員的結果是已採取措施；
// 「不採取措施」要沒有未撤銷的導出措施，也沒有成員的結果是已採取措施。鎖順序：店家列 → 全部成員通報（依 id）→ 案件列（比對版本）→ 詢問列；成員先讀再鎖：讀之前已鎖店家，
// 所有改成員的寫入者都先鎖店家，所以成員在讀與鎖之間不會變。補充不改成員，也不鎖案件。
func (s *Service) Decide(ctx context.Context, in model.DecideCaseInput) (model.AdminCase, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.AdminCase{}, err
	}
	if !slices.Contains(model.DecisionValues(), in.Decision) {
		return model.AdminCase{}, invalidOn("Decision", text.DecideDecisionRequired())
	}
	note, err := resultNote(in.Note)
	if err != nil {
		return model.AdminCase{}, err
	}
	var result model.AdminCase
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		found, err := q.CaseByID(ctx, in.CaseID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errCaseNotFound()
		}
		if err != nil {
			return err
		}
		if _, err := stint.LockAccountOf(ctx, q, found.StoreID); err != nil {
			return err
		}
		members, err := q.CaseReportIDs(ctx, &in.CaseID)
		if err != nil {
			return err
		}
		if _, err := q.LockReports(ctx, members); err != nil {
			return err
		}
		locked, err := lockCase(ctx, q, in.CaseID, in.ExpectedVersion)
		if err != nil {
			return err
		}
		facts, err := caseFactsOf(ctx, q, locked.ID)
		if err != nil {
			return err
		}
		if err := caseBlocked(facts.Availability().Decide[in.Decision], model.CaseActionDecide); err != nil {
			return err
		}
		now := s.now()
		if _, err := q.DecideCase(ctx, store.DecideCaseParams{
			Now: now, AdminID: in.AdminID, Decision: string(in.Decision), Note: note, ID: locked.ID, ExpectedVersion: locked.Version,
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errCaseVersion()
			}
			return err
		}
		// 還沒結束的詢問在同一個 transaction 由系統撤回（鎖順序：店家列、通報列、案件列、詢問列）。
		if err := withdrawOpenInquiry(ctx, q, locked.ID, in.AdminID, now); err != nil {
			return err
		}
		outcome := in.Decision.DefaultOutcome()
		resolved, err := q.ResolveCaseReports(ctx, store.ResolveCaseReportsParams{
			Outcome: string(outcome), Now: now, AdminID: in.AdminID, Note: note, CaseID: &locked.ID,
		})
		if err != nil {
			return err
		}
		caseID := locked.ID
		for _, r := range resolved {
			if err := events.Publish(ctx, s.d.Events, model.ReportResolved{
				EventID: uuid.NewV7(), OccurredAt: now, ReportID: r.ID, StoreID: r.StoreID, ReporterBuyerID: r.ReporterBuyerID, Outcome: outcome, CaseID: &caseID,
			}); err != nil {
				return err
			}
		}
		result, err = readCase(ctx, q, locked.ID, s.now())
		return err
	})
	return result, err
}

// ChangeSeverity 修改案件的嚴重度，必須附原因：原因寫成案件的一則內部註記，改前與改後記在那則註記上（同一個 transaction）。只限進行中的案件，新的嚴重度必須和目前的不同。
// 鎖順序：店家列 → 案件列（比對版本）。
func (s *Service) ChangeSeverity(ctx context.Context, in model.ChangeCaseSeverityInput) (model.AdminCase, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.AdminCase{}, err
	}
	if err := validSeverity(in.Severity); err != nil {
		return model.AdminCase{}, err
	}
	note, err := model.NoteBody(in.Note, "Note")
	if err != nil {
		return model.AdminCase{}, invalidOn("Note", text.SeverityNoteLength())
	}
	var result model.AdminCase
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		found, err := q.CaseByID(ctx, in.CaseID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errCaseNotFound()
		}
		if err != nil {
			return err
		}
		if _, err := stint.LockAccountOf(ctx, q, found.StoreID); err != nil {
			return err
		}
		locked, err := lockCase(ctx, q, in.CaseID, in.ExpectedVersion)
		if err != nil {
			return err
		}
		facts, err := caseFactsOf(ctx, q, locked.ID)
		if err != nil {
			return err
		}
		if err := caseBlocked(facts.Availability().ChangeSeverity, model.CaseActionChangeSeverity); err != nil {
			return err
		}
		if model.Severity(locked.Severity) == in.Severity {
			return invalidOn("Severity", text.SeverityUnchanged())
		}
		now := s.now()
		if _, err := q.ChangeCaseSeverity(ctx, store.ChangeCaseSeverityParams{
			Severity: string(in.Severity), Now: now, ID: locked.ID, ExpectedVersion: locked.Version,
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errCaseVersion()
			}
			return err
		}
		from, to := locked.Severity, string(in.Severity)
		if _, err := q.InsertGovernanceNote(ctx, store.InsertGovernanceNoteParams{
			NoteID: uuid.NewV7(), CaseID: &locked.ID, AdminID: in.AdminID, Body: note, SeverityFrom: &from, SeverityTo: &to, Now: now,
		}); err != nil {
			return err
		}
		result, err = readCase(ctx, q, locked.ID, s.now())
		return err
	})
	return result, err
}

// Case 讀取一個案件的後台檢視：移動紀錄、此刻可以做的動作與 etag（Version）。
func (s *Service) Case(ctx context.Context, caseID uuid.UUID) (model.AdminCase, error) {
	var out model.AdminCase
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		var err error
		out, err = readCase(ctx, store.New(s.d.DB.Conn(ctx)), caseID, s.now())
		return err
	})
	return out, err
}

// AdminCases 是 admin 的案件清單：開案時間新到舊，可依狀態、類別與店家篩選；每頁的查詢數固定，與頁大小無關。
func (s *Service) AdminCases(ctx context.Context, in model.CaseListInput) (model.AdminCasePage, error) {
	size, err := pageSize(in.PageSize)
	if err != nil {
		return model.AdminCasePage{}, err
	}
	if in.State != nil {
		if err := model.OneOf(*in.State, model.CaseStateValues(), "State", text.ListStateInvalid().Text); err != nil {
			return model.AdminCasePage{}, err
		}
	}
	if in.Category != nil {
		if err := model.OneOf(*in.Category, policy.Assessable(), "Category", text.ListCategoryInvalid().Text); err != nil {
			return model.AdminCasePage{}, err
		}
	}
	if in.StoreID < 0 {
		return model.AdminCasePage{}, model.Positive(in.StoreID)
	}
	if in.AwaitingSeller && in.State != nil && *in.State == model.CaseStateClosed {
		return model.AdminCasePage{}, invalidOn("AwaitingSeller", text.CaseAwaitingSellerWithClosed())
	}
	params := store.AdminCasesParams{PageSize: size + 1, AwaitingSeller: in.AwaitingSeller}
	if in.State != nil {
		params.State = (*string)(in.State)
	}
	if in.Category != nil {
		params.Category = (*string)(in.Category)
	}
	if in.StoreID > 0 {
		params.StoreID = &in.StoreID
	}
	if in.After != nil {
		params.AfterOpenedAt, params.AfterID = &in.After.OpenedAt, &in.After.ID
	}
	var page model.AdminCasePage
	err = s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		found, err := q.AdminCases(ctx, params)
		if err != nil {
			return err
		}
		more := len(found) > int(size)
		if more {
			found = found[:size]
		}
		rows := make([]caseRow, len(found))
		for i, r := range found {
			rows[i] = caseRow{row: r.SellerCase, storeName: r.StoreName, storeClosed: r.StoreClosed, storePurged: r.StorePurged}
		}
		facts, err := caseFacts(ctx, q, rows)
		if err != nil {
			return err
		}
		now := s.now()
		if err := loadCaseDeadlines(ctx, q, now, facts); err != nil {
			return err
		}
		page.Items = adminCases(rows, facts, now)
		if more {
			last := page.Items[size-1]
			page.Next = &model.CaseCursor{OpenedAt: last.OpenedAt, ID: last.ID}
		}
		return nil
	})
	return page, err
}

// CreateCaseNote 新增案件的一則內部註記：只新增、不帶 etag，也不動 cases（版本與 updated_at 都不變，所以後台依舊 etag 的寫入不受影響）；已結案的案件也可以寫。
// 案件不存在由外鍵擋下，回 NotFound。改嚴重度的改前改後只由 ChangeSeverity 寫，這裡不接受。
func (s *Service) CreateCaseNote(ctx context.Context, in model.CreateCaseNoteInput) (model.GovernanceNote, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.GovernanceNote{}, err
	}
	body, err := model.NoteBody(in.Body, "Body")
	if err != nil {
		return model.GovernanceNote{}, err
	}
	row, err := store.New(s.d.DB.Conn(ctx)).InsertGovernanceNote(ctx, store.InsertGovernanceNoteParams{
		NoteID: uuid.NewV7(), CaseID: &in.CaseID, AdminID: in.AdminID, Body: body, Now: s.now(),
	})
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23503" && pg.ConstraintName == "governance_notes_case_id_fkey" {
		return model.GovernanceNote{}, errCaseNotFound()
	}
	if err != nil {
		return model.GovernanceNote{}, err
	}
	// 回應取資料庫存下的值（微秒），之後 CaseNotes 讀到的才會完全相同。
	return toNote(row), nil
}

// CaseNotes 是案件的內部註記，舊到新（keyset）；案件不存在回 NotFound。多讀一筆判斷有沒有下一頁。
func (s *Service) CaseNotes(ctx context.Context, in model.CaseNoteListInput) (model.NotePage, error) {
	size, err := pageSize(in.PageSize)
	if err != nil {
		return model.NotePage{}, err
	}
	params := store.CaseNotesParams{CaseID: &in.CaseID, PageSize: size + 1}
	if in.After != nil {
		params.AfterCreatedAt, params.AfterID = &in.After.CreatedAt, &in.After.ID
	}
	var page model.NotePage
	err = s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.CaseByID(ctx, in.CaseID); errors.Is(err, pgx.ErrNoRows) {
			return errCaseNotFound()
		} else if err != nil {
			return err
		}
		rows, err := q.CaseNotes(ctx, params)
		if err != nil {
			return err
		}
		page.Items = make([]model.GovernanceNote, len(rows))
		for i, n := range rows {
			page.Items[i] = toNote(n)
		}
		return nil
	})
	if err != nil {
		return model.NotePage{}, err
	}
	if len(page.Items) > int(size) {
		page.Items = page.Items[:size]
		last := page.Items[size-1]
		page.Next = &model.NoteCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return page, nil
}

// toNote 把註記列轉成模型：改嚴重度時寫的那則帶改前與改後。
func toNote(row store.SellerGovernanceNote) model.GovernanceNote {
	n := model.GovernanceNote{ID: row.NoteID, AdminID: row.AdminID, Body: row.Body, CreatedAt: row.CreatedAt}
	if row.SeverityFrom != nil && row.SeverityTo != nil {
		n.SeverityChange = &model.SeverityChange{From: model.Severity(*row.SeverityFrom), To: model.Severity(*row.SeverityTo)}
	}
	return n
}
