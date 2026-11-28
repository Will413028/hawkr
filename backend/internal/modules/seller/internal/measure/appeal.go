package measure

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

func errAppealUnavailable(l uitext.Locale, m model.Measure) error {
	us := m.AppealUnavailabilities(l)
	if len(us) == 0 {
		return fmt.Errorf("seller：措施 %s 沒有不能申覆的原因，卻被判定不能申覆", m.ID)
	}
	return apperr.FailedPrecondition("%s", us[0].Message)
}

// SubmitAppeal 賣家對自己帳號的措施提出申覆（每個措施一次，只對未撤銷的措施，已結束的措施也可以）。資格是「帳號擁有這筆措施」
// （措施所在的店家屬於這個帳號，任何一個 stint 都算），不看帳號現在有沒有店家、店家有沒有能力：措施以帳號為單位、關店不解除，
// 所有 stint 都關閉的帳號照樣受處分，必須還能申覆；措施不屬於這個帳號時回 NotFound。版本（賣家看得到的版本）要與讀到的相符
// （不符回 VersionConflict，先於狀態檢查）。佐證依 media 的順序在 transaction 外先轉正（不建立引用），寫入的 transaction 內
// 排入以請求 ID 為 ref 的引用同步。後台的版本加一、賣家的版本不變。措施已結束而仍有待處理的解除請求時，解除請求在同一個 transaction
// 先以 CLOSED（ENDED，結案時間是措施的結束時間）結案再寫申覆。發出 MeasureRequestSubmitted。
func (s *Service) SubmitAppeal(ctx context.Context, in model.SubmitAppealInput) (model.Measure, error) {
	return s.submitRequest(ctx, requestSpec{
		kind:              model.RequestKindAppeal,
		reasonLabel:       "申覆理由",
		allowed:           model.Measure.CanAppeal,
		unavailable:       errAppealUnavailable,
		retiredMeansEnded: true,
	}, in)
}

type requestSpec struct {
	kind              model.RequestKind
	reasonLabel       string
	allowed           func(model.Measure) bool
	unavailable       func(uitext.Locale, model.Measure) error
	retiredMeansEnded bool
}

func (s *Service) submitRequest(ctx context.Context, spec requestSpec, in model.SubmitAppealInput) (model.Measure, error) {
	if err := model.Positive(in.AccountID); err != nil {
		return model.Measure{}, err
	}
	if in.ExpectedVersion <= 0 {
		return model.Measure{}, errExpectedVersion()
	}
	reason, err := model.Required(in.Reason, 2000, "Reason", spec.reasonLabel)
	if err != nil {
		return model.Measure{}, err
	}
	if err := model.CheckEvidenceTokens(in.EvidenceTokens); err != nil {
		return model.Measure{}, err
	}
	current, err := s.AccountMeasure(ctx, in.AccountID, in.MeasureID)
	if err != nil {
		return model.Measure{}, err
	}
	if current.SellerVersion() != in.ExpectedVersion {
		return model.Measure{}, errMeasureVersion()
	}
	if !spec.allowed(current) {
		return model.Measure{}, spec.unavailable(uitext.LocaleFrom(ctx), current)
	}
	var result model.Measure
	err = s.retryScope(ctx, func() error {
		return s.d.DB.InTx(ctx, func(ctx context.Context) error {
			q := store.New(s.d.DB.Conn(ctx))
			locked, row, err := s.lockForRequest(ctx, q, in.MeasureID)
			if err != nil {
				return err
			}
			now := s.now()
			fresh, err := s.readRow(ctx, q, row, detailList, now)
			if err != nil {
				return err
			}
			if fresh.SellerVersion() != in.ExpectedVersion {
				return errMeasureVersion()
			}
			if !spec.allowed(fresh) {
				return spec.unavailable(uitext.LocaleFrom(ctx), fresh)
			}
			if spec.retiredMeansEnded && locked.RetiredAt != nil {
				closedByThis, err := s.closedByTermination(ctx, q, row)
				if err != nil {
					return err
				}
				if closedByThis {
					return apperr.FailedPrecondition("%s", text.AppealUnavailableWindowEnded(uitext.LocaleFrom(ctx)))
				}
			}
			if fresh.MootRemediation() != nil {
				if _, err := s.closeEndedRemediations(ctx, q, []store.SellerMeasure{row}, now); err != nil {
					return err
				}
			}
			request, err := q.InsertMeasureRequest(ctx, store.InsertMeasureRequestParams{
				ID: uuid.NewV7(), MeasureID: row.ID, Kind: string(spec.kind), Reason: reason, Revision: row.CurrentRevision, Now: now,
			})
			if err != nil {
				return err
			}
			evidence, err := model.ConfirmEvidence(ctx, s.d.Media, media.Uploader{ID: in.AccountID}, in.EvidenceTokens)
			if err != nil {
				return err
			}
			for i, e := range evidence {
				if err := q.InsertRequestEvidence(ctx, store.InsertRequestEvidenceParams{
					RequestID: request.ID, Position: int32(i), AssetID: e.AssetID, ObjectKey: e.ObjectKey, //nolint:gosec // 佐證最多 MaxEvidence 張
				}); err != nil {
					return err
				}
			}
			touched, err := q.TouchMeasure(ctx, store.TouchMeasureParams{ID: row.ID, ExpectedVersion: row.Version, Now: now})
			if err != nil {
				return err
			}
			if result, err = s.readRow(ctx, q, touched, detailList, now); err != nil {
				return err
			}
			return events.Publish(ctx, s.d.Events, model.MeasureRequestSubmitted{
				EventID: uuid.NewV7(), OccurredAt: now, MeasureID: row.ID, RequestID: request.ID, Kind: spec.kind,
				StoreID: row.StoreID, AccountID: locked.AccountID,
			})
		})
	})
	if err != nil {
		return model.Measure{}, err
	}
	return result, nil
}

func (s *Service) closedByTermination(ctx context.Context, q *store.Queries, m store.SellerMeasure) (bool, error) {
	if m.Kind != string(model.MeasureTermination) {
		return false, nil
	}
	closures, err := q.CurrentStoreClosures(ctx, []int64{m.StoreID})
	if err != nil {
		return false, err
	}
	for _, c := range closures {
		if c.ClosedBy == string(model.ClosedByTermination) && c.ClosedByMeasureID != nil && *c.ClosedByMeasureID == m.ID {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) lockForRequest(ctx context.Context, q *store.Queries, measureID uuid.UUID) (store.SellerStore, store.SellerMeasure, error) {
	current, err := q.MeasureByID(ctx, measureID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.SellerStore{}, current, errMeasureNotFound()
	}
	if err != nil {
		return store.SellerStore{}, current, err
	}
	accounts := []int64{current.AccountID}
	if current.Kind == string(model.MeasureTermination) {
		linked, err := q.LinkedMeasureAccounts(ctx, []uuid.UUID{measureID})
		if err != nil {
			return store.SellerStore{}, current, err
		}
		accounts = append(accounts, linked...)
	}
	s.hooks.preRead()
	if err := stint.LockAccounts(ctx, q, accounts); err != nil {
		return store.SellerStore{}, current, err
	}
	row, err := q.LockMeasure(ctx, measureID)
	if err != nil {
		return store.SellerStore{}, row, err
	}
	if row.Kind == string(model.MeasureTermination) {
		linked, err := q.LinkedMeasureAccounts(ctx, []uuid.UUID{measureID})
		if err != nil {
			return store.SellerStore{}, row, err
		}
		for _, id := range linked {
			if !slices.Contains(accounts, id) {
				return store.SellerStore{}, row, errScopeChanged
			}
		}
	}
	locked, err := q.StoreByID(ctx, row.StoreID)
	if err != nil {
		return store.SellerStore{}, row, model.Error(err)
	}
	s.hooks.locked()
	return locked, row, nil
}

// DecideAppeal admin 裁定待處理的申覆；版本要與讀到的相符（不符回 VersionConflict，先於狀態檢查，不推斷重送：裁定已經生效後
// 帶舊版本再裁定一樣回 VersionConflict，不寫入、不重發事件）。接受與駁回都必須附理由（條款第 18.4 條）。接受時措施以
// APPEAL_ACCEPTED 撤銷，同一個 transaction 解除效果、重算停權狀態（發 MeasureRevoked），RevokeFindings 為 true 時一併撤銷
// 這個措施自己建立的認定並連帶撤銷引用它們的其他措施（只限依據認定的措施）；駁回時措施不變（RevokeFindings 不可以設）。發出
// MeasureRequestDecided。
func (s *Service) DecideAppeal(ctx context.Context, in model.DecideAppealInput) (model.Measure, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.Measure{}, err
	}
	if in.Outcome != model.RequestAccepted && in.Outcome != model.RequestRejected {
		return model.Measure{}, apperr.InvalidInputOn([]string{"Outcome"}, "裁定只能是接受或駁回")
	}
	if in.ExpectedVersion <= 0 {
		return model.Measure{}, errExpectedVersion()
	}
	reason, err := model.Required(in.Reason, 2000, "Reason", "裁定理由")
	if err != nil {
		return model.Measure{}, err
	}
	if in.RevokeFindings && in.Outcome != model.RequestAccepted {
		return model.Measure{}, apperr.InvalidInputOn([]string{"RevokeFindings"}, "只有接受申覆時可以一併撤銷認定")
	}
	var result model.Measure
	err = s.retryScope(ctx, func() error {
		return s.d.DB.InTx(ctx, func(ctx context.Context) error {
			q := store.New(s.d.DB.Conn(ctx))
			now := s.now()
			scope, row, err := s.lockForRevoke(ctx, q, in.MeasureID, in.RevokeFindings)
			if err != nil {
				return err
			}
			if row.Version != in.ExpectedVersion {
				return errMeasureVersion()
			}
			current, err := s.readRow(ctx, q, row, detailList, now)
			if err != nil {
				return err
			}
			if !current.CanDecideAppeal() {
				return apperr.FailedPrecondition("沒有待裁定的申覆")
			}
			if in.RevokeFindings && !current.CanRevokeFindings() {
				return apperr.InvalidInputOn([]string{"RevokeFindings"}, "這個措施沒有可以撤銷的認定（只有依據認定、而且自己建立了尚未撤銷的認定的措施可以一併撤銷認定）")
			}
			plan, err := s.planRevoke(ctx, q, scope, row, in.RevokeFindings, now)
			if err != nil {
				return err
			}
			appeal := current.Appeal()
			decided, err := q.DecideRequest(ctx, store.DecideRequestParams{ID: appeal.ID, State: string(in.Outcome), Reason: reason, AdminID: in.AdminID, Now: now})
			if errors.Is(err, pgx.ErrNoRows) {
				return apperr.FailedPrecondition("沒有待裁定的申覆")
			}
			if err != nil {
				return err
			}
			var after store.SellerMeasure
			if in.Outcome == model.RequestAccepted {
				if after, err = s.revoke(ctx, q, plan, row, in.AdminID, model.RevokeAppealAccepted, now); err != nil {
					return err
				}
			} else {
				after, err = q.TouchMeasure(ctx, store.TouchMeasureParams{ID: row.ID, ExpectedVersion: row.Version, Now: now})
				if errors.Is(err, pgx.ErrNoRows) {
					return errMeasureVersion()
				}
				if err != nil {
					return err
				}
			}
			if result, err = s.readRow(ctx, q, after, detailAdmin, now); err != nil {
				return err
			}
			return events.Publish(ctx, s.d.Events, model.MeasureRequestDecided{
				EventID: uuid.NewV7(), OccurredAt: now, MeasureID: row.ID, RequestID: decided.ID, Kind: model.RequestKind(decided.Kind),
				StoreID: row.StoreID, AccountID: row.AccountID, Outcome: in.Outcome,
			})
		})
	})
	return result, err
}
