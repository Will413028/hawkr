package measure

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// errRemediationUnavailable 是請求解除的前置條件不成立時的失敗，訊息是 RemediationUnavailabilities 的第一筆（與 can_request_remediation、API 的
// remediation_unavailabilities 同一個規則與同一份文字）。原因清單是空的是程式錯誤，回內部錯誤。
func errRemediationUnavailable(l uitext.Locale, m model.Measure) error {
	us := m.RemediationUnavailabilities(l)
	if len(us) == 0 {
		return fmt.Errorf("seller：措施 %s 沒有不能請求解除的原因，卻被判定不能請求", m.ID)
	}
	return apperr.FailedPrecondition("%s", us[0].Message)
}

// RequestRemediation 賣家對自己帳號、依據風險類型且生效中的措施請求解除（條款第 17.4 條）。可以重複提出，但同時只能有一筆待處理
// 的請求（申覆與解除請求合計）、冷卻期內不能提出（Measure.NextRemediationTime）。資格、版本、佐證與寫入的規則同 SubmitAppeal：帳號
// 擁有這筆措施（不是回 NotFound），版本不符回 VersionConflict（先於狀態檢查），後台的版本加一、賣家的版本不變，所以請求生效後再送
// 一次回 FailedPrecondition。發出 MeasureRequestSubmitted{REMEDIATION}。
func (s *Service) RequestRemediation(ctx context.Context, in model.RequestRemediationInput) (model.Measure, error) {
	return s.submitRequest(ctx, requestSpec{
		kind:        model.RequestKindRemediation,
		reasonLabel: "改善說明",
		allowed:     model.Measure.CanRequestRemediation,
		unavailable: errRemediationUnavailable,
	}, model.SubmitAppealInput(in))
}

// DecideRemediation admin 裁定措施待處理的解除請求；版本要與讀到的相符（不符回 VersionConflict，先於狀態檢查，不推斷重送）。接受與駁回
// 都必須附理由。措施必須生效中（CanDecideRemediation）：已結束而仍待處理的解除請求沒有標的，回 FailedPrecondition。接受時同一個
// transaction 寫一版結束時間是現在的新版本（作者是裁定的管理員、statement 不變）、重算停權狀態、發 MeasureEnded（不是撤銷，認定與撤銷
// 原因不動，也不經 CanEnd），賣家與後台的版本都變；駁回時措施不變、只有後台的版本加一，並計入冷卻。發出 MeasureRequestDecided{REMEDIATION}。
func (s *Service) DecideRemediation(ctx context.Context, in model.DecideRemediationInput) (model.Measure, error) {
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
	var result model.Measure
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.now()
		locked, row, err := s.lockMeasure(ctx, q, in.MeasureID)
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
		if !current.CanDecideRemediation() {
			return apperr.FailedPrecondition("沒有可以裁定的解除請求")
		}
		decided, err := q.DecideRequest(ctx, store.DecideRequestParams{
			ID: current.PendingRemediation().ID, State: string(in.Outcome), Reason: reason, AdminID: in.AdminID, Now: now,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.FailedPrecondition("沒有可以裁定的解除請求")
		}
		if err != nil {
			return err
		}
		var after store.SellerMeasure
		if in.Outcome == model.RequestAccepted {
			if after, err = s.endNow(ctx, q, locked, row, current.StatementToSeller, in.AdminID, now); err != nil {
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
			StoreID: row.StoreID, AccountID: locked.AccountID, Outcome: in.Outcome,
		})
	})
	return result, err
}
