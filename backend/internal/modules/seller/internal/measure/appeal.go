package measure

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/mediaref"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/text"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// errAppealUnavailable 是申覆的前置條件不成立時的失敗，訊息是 AppealUnavailabilities 的第一筆（與 can_appeal、API 的 appeal_unavailabilities 同一個規則與同一份文字）。
// 原因清單是空的表示呼叫端的判斷與模型不一致，是程式錯誤，回內部錯誤，不對使用者說通用的句子。
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

// requestSpec 是兩種賣家請求在提出時不同的地方：種類、理由的欄位名稱、前置條件與失敗訊息。
type requestSpec struct {
	kind        model.RequestKind
	reasonLabel string
	allowed     func(model.Measure) bool
	unavailable func(uitext.Locale, model.Measure) error
	// retiredMeansEnded 為 true（申覆）時，措施所在的店家已退場（第一段清理）而且是被這筆終止關閉的，就回「申覆期限已過」。
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
	if err := mediaref.EvidenceTokens(in.EvidenceTokens); err != nil {
		return model.Measure{}, err
	}
	if _, ok := db.TxFrom(ctx); ok {
		return model.Measure{}, apperr.FailedPrecondition("請求必須在 transaction 外開始")
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
	evidence, err := mediaref.FinalizeEvidence(ctx, s.d.Media, media.Uploader{ID: in.AccountID}, in.EvidenceTokens)
	if err != nil {
		return model.Measure{}, err
	}
	var result model.Measure
	err = s.retryScope(ctx, func() error {
		return s.d.DB.InTx(ctx, func(ctx context.Context) error {
			q := store.New(s.d.DB.Conn(ctx))
			locked, row, err := s.lockForRequest(ctx, q, in.MeasureID)
			if err != nil {
				return err
			}
			// 現在時間在鎖之後取：等鎖的期間退場可能已經 commit，可用性的時間判斷要晚於它；結果不依賴時鐘的是下面的 retired_at 檢查。
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
			// 「已退場 ⇒ 申覆期限已過」只在造成這家店目前關閉的終止合作上成立：第一段清理（退場，RetireDueStores）對 closed_by = TERMINATION 的
			// retire_after 是 greatest(關店後 StoreRetireAfter, 帳號上未撤銷終止中最晚的申覆期限)，退場只在 now > retire_after 時發生，所以那一筆終止
			// （未撤銷）的期限必已過，不論這個實例的時鐘怎麼走（多實例的時鐘差）；第二段（個資清理）一定在退場之後，所以看 retired_at 就涵蓋兩段。
			// 其他措施不成立：停權與警告沒有申覆期限，賣家自己關店的退場只看關店後 StoreRetireAfter（30 天），預告終止的期限（生效後 184 天）可能還沒到，
			// 這些照常申覆。只看措施所在的店家（帳號早先自己關閉而退場的舊 stint 不代表這筆措施的期限已過）。解除請求沒有 legal hold，不受影響。
			// 放在可用性檢查之後：失敗原因的順序要和讀模型一致（已撤銷、已申覆過、期限已過、有待處理的請求），已撤銷的終止在已退場的店家上回的是「已撤銷」。
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
			for i, e := range evidence {
				if err := q.InsertRequestEvidence(ctx, store.InsertRequestEvidenceParams{
					RequestID: request.ID, Position: int32(i), AssetID: e.AssetID, ObjectKey: e.ObjectKey, //nolint:gosec // 佐證最多 MaxEvidence 張
				}); err != nil {
					return err
				}
			}
			if len(evidence) > 0 {
				if err := mediaref.Changed(ctx, q, s.d.Jobs, now, mediaref.EvidenceRef(request.ID)); err != nil {
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

// closedByTermination 回傳這筆措施是不是店家目前這次關閉的原因（終止合作、closed_by = TERMINATION 且 closed_by_measure_id 是它）。
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

// lockForRequest 是賣家請求（申覆與解除請求）的鎖：先鎖措施所在帳號全部的 stint（所有措施的寫入都先鎖它），再鎖措施列，與 lockMeasure 同一個順序。
// 措施是終止合作時，帳號上以它為來源的關聯終止所在的帳號也要鎖：清理的 legal hold 含「關聯終止的來源有待處理的申覆」，被關聯帳號的清理要和這筆申覆競爭
// 同一組店家列，鎖了才和清理的再判斷串行（step10e-3cb §2.4）。做法同撤銷（lockForRevoke）：不加鎖預讀被關聯的帳號，依 account_id 升序鎖
// （stint.LockAccounts，與關聯終止的建立、撤銷同一個跨帳號鎖序），鎖住措施列之後重讀；超出已鎖的範圍（剛好新增了關聯終止）回 errScopeChanged，
// 呼叫方整個 transaction 重來。回傳措施所在的店家列（鎖住之後讀）與措施列。
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
			after := row
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
