package governance

import (
	"context"
	"errors"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
)

func validTarget(t model.CaseTarget) error {
	switch {
	case t.Join != nil && t.OpenNew != nil, t.Join == nil && t.OpenNew == nil:
		return invalidOn("Target", text.TriageTargetRequired())
	case t.OpenNew != nil:
		if *t.OpenNew == "" {
			return invalidOn("Target", text.TriageSeverityRequired())
		}
		if !slices.Contains(model.SeverityValues(), *t.OpenNew) {
			return invalidOn("Target", text.SeverityRequired())
		}
	}
	return nil
}

func joinCase(ctx context.Context, q *store.Queries, existing *store.SellerCase, storeID int64, category string, target model.CaseTarget, adminID int64, now time.Time) (joined store.SellerCase, created bool, err error) {
	if join := target.Join; join != nil {
		if existing == nil || existing.ID != join.CaseID || existing.Version != join.ExpectedVersion {
			return store.SellerCase{}, false, errTargetCaseVersion()
		}
		return *existing, false, nil
	}
	if existing != nil {
		return store.SellerCase{}, false, errCategoryCaseOpened()
	}
	joined, err = q.InsertCase(ctx, store.InsertCaseParams{
		ID: uuid.NewV7(), StoreID: storeID, Category: category, Severity: string(*target.OpenNew), AdminID: adminID, Now: now,
	})
	return joined, err == nil, err
}

// Triage 分案：判定平台判定類別，把通報掛到去處（Target）指定的案件：加入這家店那個類別進行中的案件，或開新案件（嚴重度）；去處此刻不成立回案件版本衝突。只有還沒有案件也沒有結果的通報可以
// （Availability().Triage）。鎖順序：店家列 → 通報列（比對版本）→ 案件列；先做無狀態的驗證（類別的子集、去處的形狀與嚴重度的值），版本與狀態都不符時先回版本衝突。
func (s *Service) Triage(ctx context.Context, in model.TriageReportInput) (model.AdminReport, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.AdminReport{}, err
	}
	if err := validAssessed(in.AssessedCategory); err != nil {
		return model.AdminReport{}, err
	}
	if err := validTarget(in.Target); err != nil {
		return model.AdminReport{}, err
	}
	var result model.AdminReport
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
		facts, err := reportFactsOf(ctx, q, locked, owner.Status == string(model.Closed))
		if err != nil {
			return err
		}
		if err := reportBlocked(facts.Availability().Triage); err != nil {
			return err
		}
		now := s.now()
		category := string(in.AssessedCategory)
		var existing *store.SellerCase
		open, err := q.LockOpenCase(ctx, store.LockOpenCaseParams{StoreID: locked.StoreID, Category: category})
		switch {
		case err == nil:
			existing = &open
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		target, created, err := joinCase(ctx, q, existing, locked.StoreID, category, in.Target, in.AdminID, now)
		if err != nil {
			return err
		}
		updated, err := q.TriageReport(ctx, store.TriageReportParams{
			CaseID: target.ID, Category: category, Now: now, AdminID: in.AdminID, ID: locked.ID, ExpectedVersion: locked.Version,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errReportVersion()
		}
		if err != nil {
			return err
		}
		if !created {
			if err := q.BumpCase(ctx, store.BumpCaseParams{ID: target.ID, Now: now}); err != nil {
				return err
			}
		}
		result, err = readReport(ctx, q, updated.ID, now)
		return err
	})
	return result, err
}
