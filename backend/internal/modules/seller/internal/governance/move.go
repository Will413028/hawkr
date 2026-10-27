package governance

import (
	"context"
	"errors"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
)

// Move 把通報移到其他案件（改平台判定類別）：通報移到這家店新類別進行中的案件（去處的規則同 Triage：加入、或開新案件，此刻不成立回案件版本衝突）。只有進行中案件裡、還沒有結果、不是認定來源的通報可以，
// 案件有導出的措施或轉報紀錄時最後一筆不能移走（Availability().Move；記錄轉報也先鎖店家，所以讀到的轉報紀錄在移動期間不會變）。移動留紀錄（report_case_moves）；移走最後一筆時原案件以 NO_REPORTS 關閉（還沒結束的詢問由系統撤回），不算一次決定、不發事件。
// 鎖順序：店家列 → 通報列（比對版本）→ 原案件與目標案件（依 id 升序）。
func (s *Service) Move(ctx context.Context, in model.MoveReportInput) (model.MoveReportResult, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.MoveReportResult{}, err
	}
	if err := validAssessed(in.AssessedCategory); err != nil {
		return model.MoveReportResult{}, err
	}
	if err := validTarget(in.Target); err != nil {
		return model.MoveReportResult{}, err
	}
	var result model.MoveReportResult
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		found, err := q.ReportByID(ctx, in.ReportID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errReportNotFound()
		}
		if err != nil {
			return err
		}
		owner, err := stint.LockAccountOf(ctx, q, found.StoreID)
		if err != nil {
			return err
		}
		locked, err := lockReport(ctx, q, in.ReportID, in.ExpectedVersion)
		if err != nil {
			return err
		}
		// 不在案件裡的通報沒有案件可鎖，由下面的 Availability().Move（NOT_IN_OPEN_CASE）回錯誤。
		category := string(in.AssessedCategory)
		var source *store.SellerCase
		var target *store.SellerCase
		if locked.CaseID != nil {
			// 這家店新類別進行中的案件不鎖就讀：呼叫方已鎖住店家列，所有改案件的指令都先鎖店家，所以讀到之後不會被別人關閉或另開。
			ids := []uuid.UUID{*locked.CaseID}
			if open, err := q.OpenCaseOf(ctx, store.OpenCaseOfParams{StoreID: locked.StoreID, Category: category}); err == nil {
				ids = append(ids, open.ID)
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			rows, err := q.LockCases(ctx, ids)
			if err != nil {
				return err
			}
			for i := range rows {
				switch {
				case rows[i].ID == *locked.CaseID:
					source = &rows[i]
				case rows[i].Category == category && rows[i].ClosedAt == nil:
					target = &rows[i]
				}
			}
		}
		facts, err := reportFactsOf(ctx, q, locked, owner.Status == string(model.Closed))
		if err != nil {
			return err
		}
		if err := reportBlocked(facts.Availability().Move); err != nil {
			return err
		}
		if locked.AssessedCategory != nil && *locked.AssessedCategory == category {
			return invalidOn("AssessedCategory", text.MoveSameCategory())
		}
		now := s.now()
		toCase, created, err := joinCase(ctx, q, target, locked.StoreID, category, in.Target, in.AdminID, now)
		if err != nil {
			return err
		}
		updated, err := q.MoveReport(ctx, store.MoveReportParams{
			CaseID: toCase.ID, Category: category, Now: now, ID: locked.ID, ExpectedVersion: locked.Version,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errReportVersion()
		}
		if err != nil {
			return err
		}
		if err := q.InsertReportCaseMove(ctx, store.InsertReportCaseMoveParams{
			ID: uuid.NewV7(), ReportID: locked.ID, StoreID: locked.StoreID, FromCaseID: source.ID, ToCaseID: toCase.ID, AdminID: in.AdminID, Now: now,
		}); err != nil {
			return err
		}
		// 原案件：沒有成員了就以 NO_REPORTS 關閉（同時版本加一），否則是一次處理（BumpCase）。
		_, closeErr := q.CloseEmptyCase(ctx, store.CloseEmptyCaseParams{Now: now, AdminID: in.AdminID, ID: source.ID})
		closed := closeErr == nil
		if closed {
			// 結案時還沒結束的詢問由系統撤回（鎖順序：店家列、通報列、案件列、詢問列）。
			if err := withdrawOpenInquiry(ctx, q, source.ID, in.AdminID, now); err != nil {
				return err
			}
		}
		switch {
		case errors.Is(closeErr, pgx.ErrNoRows):
			if err := q.BumpCase(ctx, store.BumpCaseParams{ID: source.ID, Now: now}); err != nil {
				return err
			}
		case closeErr != nil:
			return closeErr
		}
		if !created {
			if err := q.BumpCase(ctx, store.BumpCaseParams{ID: toCase.ID, Now: now}); err != nil {
				return err
			}
		}
		view, err := readReport(ctx, q, updated.ID, now)
		if err != nil {
			return err
		}
		result = model.MoveReportResult{Report: view, SourceCaseClosed: closed}
		return nil
	})
	return result, err
}
