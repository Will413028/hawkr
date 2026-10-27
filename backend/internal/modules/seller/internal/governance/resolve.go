package governance

import (
	"context"
	"errors"
	"slices"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// resultNote 驗證給檢舉人的結果說明（選填，最多 2000 字）並去掉前後空白；空字串是沒有說明（nil）。
func resultNote(note string) (*string, error) {
	trimmed := model.Text(note)
	if trimmed != nil && utf8.RuneCountInString(*trimmed) > model.NoteBodyMaxRunes {
		return nil, invalidOn("Note", text.ResolveNoteLength())
	}
	return trimmed, nil
}

// Resolve 設定通報的結果（已採取措施、未違規、內容不足、重複、濫報）；結果設定後不能修改。已採取措施只能設在案件裡的通報，而且案件要有導出的措施或已有成員的結果是已採取措施；
// 還沒分案的通報可以直接設定其他四個結果（不開案件）。鎖順序：店家列 → 通報列（比對版本）→ 案件列（有案件時），在鎖住之後以 Availability().Resolve 判斷；發出 ReportResolved。
func (s *Service) Resolve(ctx context.Context, in model.ResolveReportInput) (model.AdminReport, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.AdminReport{}, err
	}
	if !slices.Contains(model.OutcomeValues(), in.Outcome) {
		return model.AdminReport{}, invalidOn("Outcome", text.ResolveOutcomeRequired())
	}
	note, err := resultNote(in.Note)
	if err != nil {
		return model.AdminReport{}, err
	}
	var result model.AdminReport
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
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
		if locked.CaseID != nil {
			if _, err := q.LockCase(ctx, *locked.CaseID); err != nil {
				return err
			}
		}
		facts, err := reportFactsOf(ctx, q, locked, owner.Status == string(model.Closed))
		if err != nil {
			return err
		}
		if err := reportBlocked(facts.Availability().Resolve[in.Outcome]); err != nil {
			return err
		}
		now := s.now()
		updated, err := q.ResolveReport(ctx, store.ResolveReportParams{
			Outcome: string(in.Outcome), Now: now, AdminID: in.AdminID, Note: note, ID: locked.ID, ExpectedVersion: locked.Version,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errReportVersion()
		}
		if err != nil {
			return err
		}
		if locked.CaseID != nil {
			if err := q.BumpCase(ctx, store.BumpCaseParams{ID: *locked.CaseID, Now: now}); err != nil {
				return err
			}
		}
		if err := events.Publish(ctx, s.d.Events, model.ReportResolved{
			EventID: uuid.NewV7(), OccurredAt: now, ReportID: updated.ID, StoreID: updated.StoreID, ReporterBuyerID: updated.ReporterBuyerID,
			Outcome: in.Outcome, CaseID: updated.CaseID,
		}); err != nil {
			return err
		}
		result, err = readReport(ctx, q, updated.ID, now)
		return err
	})
	return result, err
}
