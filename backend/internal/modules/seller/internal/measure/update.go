package measure

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/suspension"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

func errExpectedVersion() error {
	return apperr.InvalidInputOn([]string{"ExpectedVersion"}, "必須提供讀取時的版本")
}

// endsChange 是結束時間的修改方向：延長、縮短，或沒有改變。
type endsChange int

const (
	endsUnchanged endsChange = iota
	endsExtended
	endsShortened
)

// compareEnds 比較目前與新的結束時間（nil 是沒有固定期間，等於無限晚）。
func compareEnds(current, next *time.Time) endsChange {
	switch {
	case current == nil && next == nil:
		return endsUnchanged
	case next == nil:
		return endsExtended
	case current == nil:
		return endsShortened
	case next.After(*current):
		return endsExtended
	case next.Before(*current):
		return endsShortened
	}
	return endsUnchanged
}

// Update 部分更新措施賣家看得到的內容（給賣家的理由、結束時間），以版本做樂觀鎖。新的結束時間必須晚於現在（不得縮到過去：
// 那會改寫賣家實際經歷的期間）；比目前晚是延長、比目前早是縮短，RISK 從沒有固定期間改成有期限算縮短，清除 RISK 的結束時間算
// 延長；依據認定的停權不能清除結束時間，警告不能設結束時間。只有生效中的停權可以延長或縮短，已結束的措施要延長（再限制）請另立
// 措施。提出申覆起措施凍結：不能改理由、不能延長（往輕改理由也不行，更正要經裁定申覆），只能縮短（FailedPrecondition，不帶
// 欄位；加重要另立一筆措施，條款第十七條第 3、4 項）。有待處理的解除請求時同樣凍結（不能改理由、不能延長，只能縮短；縮短到期後解除請求以
// CLOSED（ENDED）結案，要加重先裁定解除請求）。實際寫入時同一個 transaction 寫下一版，version 加一並重算停權狀態；
// 合併後沒有任何改變時不寫入、也不產生新的一版。內部註記不經這裡（CreateNote）。
func (s *Service) Update(ctx context.Context, in model.UpdateMeasureInput) (model.Measure, error) {
	in.EndsAt = atMicrosecond(in.EndsAt)
	if err := model.Positive(in.AdminID); err != nil {
		return model.Measure{}, err
	}
	if in.ExpectedVersion <= 0 {
		return model.Measure{}, errExpectedVersion()
	}
	var statement *string
	if in.StatementToSeller != nil {
		value, err := model.Required(*in.StatementToSeller, 2000, "StatementToSeller", "給賣家的理由")
		if err != nil {
			return model.Measure{}, err
		}
		statement = &value
	}
	var result model.Measure
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
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
		if current.Status != model.MeasureStatusEnforced {
			return errMeasureRevoked()
		}
		nextStatement, nextEnds := current.StatementToSeller, current.EndsAt
		if statement != nil {
			nextStatement = *statement
		}
		if in.ChangeEndsAt {
			if nextEnds, err = requestedEnds(current, in.EndsAt, now); err != nil {
				return err
			}
		}
		change := compareEnds(current.EndsAt, nextEnds)
		switch {
		case nextStatement != current.StatementToSeller && !current.CanUpdateStatement():
			return errActionUnavailable(current, model.MeasureActionUpdateStatement)
		case change == endsExtended && !current.CanExtend():
			return errActionUnavailable(current, model.MeasureActionExtend)
		case change == endsShortened && !current.CanShorten():
			return errActionUnavailable(current, model.MeasureActionShorten)
		case nextStatement == current.StatementToSeller && change == endsUnchanged:
			result, err = s.readRow(ctx, q, row, detailAdmin, now)
			return err
		}
		updated, err := q.AdvanceMeasureRevision(ctx, store.AdvanceMeasureRevisionParams{
			ID: row.ID, ExpectedVersion: in.ExpectedVersion, StatementToSeller: nextStatement, EndsAt: nextEnds, AdminID: &in.AdminID, Now: now,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return errMeasureVersion()
		}
		if err != nil {
			return err
		}
		if change != endsUnchanged {
			if nextEnds != nil {
				if err := suspension.ScheduleExpiry(ctx, s.d.Jobs, row.ID, *nextEnds); err != nil {
					return err
				}
			}
			if err := suspension.Recompute(ctx, q, s.d.Events, s.d.Jobs, s.d.Exposure, locked.AccountID, now); err != nil {
				return err
			}
		}
		if err := events.Publish(ctx, s.d.Events, model.MeasureRevised{
			EventID: uuid.NewV7(), OccurredAt: now, MeasureID: row.ID, StoreID: row.StoreID, AccountID: locked.AccountID,
			Revision: updated.CurrentRevision, EndsAt: nextEnds,
		}); err != nil {
			return err
		}
		result, err = s.readRow(ctx, q, store.SellerMeasure(updated), detailAdmin, now)
		return err
	})
	return result, err
}

// requestedEnds 回傳套用要求的結束時間（nil 是清除）後的值。結束時間不能調的措施（警告、終止合作、不在生效中的停權）先走可用性：
// 要求與目前不同就是延長或縮短，回 errActionUnavailable，文字與 API 的 message 同一句；與目前相同不算改變。
// 能調的措施才驗證值本身：依據認定的停權必須有結束時間；設定的值必須晚於現在。
func requestedEnds(current model.Measure, requested *time.Time, now time.Time) (*time.Time, error) {
	if len(current.ShortenUnavailableReasons()) > 0 {
		switch compareEnds(current.EndsAt, requested) {
		case endsExtended:
			return nil, errActionUnavailable(current, model.MeasureActionExtend)
		case endsShortened:
			return nil, errActionUnavailable(current, model.MeasureActionShorten)
		case endsUnchanged:
		}
		return current.EndsAt, nil
	}
	switch {
	case requested == nil && current.Basis == model.BasisViolation:
		return nil, apperr.InvalidInputOn([]string{"EndsAt"}, "依據認定的停權必須有結束時間")
	case requested != nil && !requested.After(now):
		return nil, apperr.InvalidInputOn([]string{"EndsAt"}, "結束時間必須晚於現在")
	}
	return requested, nil
}

// errMeasureRevoked 是對已撤銷的措施做動作的錯誤；不經過原因清單，狀態本身就是判斷，文字與 REVOKED 原因相同。
func errMeasureRevoked() error {
	return apperr.FailedPrecondition("%s", model.ActionUnavailableText(model.ActionReasonRevoked, model.MeasureActionUpdateStatement).Text)
}

// errActionUnavailable 說出第一個（最優先的）不能執行 action 的原因，文字是 model.ActionUnavailableText，與 API 的
// MeasureActionUnavailability.message 同一個函式；錯誤的 LocalizedMessage 就是這句。
func errActionUnavailable(m model.Measure, action model.MeasureAction) error {
	reasons := m.ActionUnavailableReasons(action)
	if len(reasons) == 0 {
		return apperr.FailedPrecondition("這筆措施目前不能操作")
	}
	return apperr.FailedPrecondition("%s", model.ActionUnavailableText(reasons[0], action).Text)
}

// End 結束 RISK 措施（風險已排除）：寫一版結束時間是現在的新版本，帳號的停權期間在同一個 transaction 結束。只限生效中的 RISK
// 停權；依據認定的措施要縮短期間或撤銷。版本要與讀到的相符。發出 MeasureEnded。
func (s *Service) End(ctx context.Context, in model.EndMeasureInput) (model.Measure, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.Measure{}, err
	}
	if in.ExpectedVersion <= 0 {
		return model.Measure{}, errExpectedVersion()
	}
	var result model.Measure
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
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
		if !current.CanEnd() {
			return errActionUnavailable(current, model.MeasureActionEnd)
		}
		updated, err := s.endNow(ctx, q, locked, row, current.StatementToSeller, in.AdminID, now)
		if err != nil {
			return err
		}
		result, err = s.readRow(ctx, q, updated, detailAdmin, now)
		return err
	})
	return result, err
}

// endNow 是結束措施在同一個 transaction 要做的事（End 與接受解除請求共用，不經 CanEnd）：寫一版結束時間是現在的新版本（給賣家的
// 理由 statement 不變，作者是 adminID）、重算帳號的停權期間、發 MeasureEnded。呼叫方已鎖住帳號與措施列，且已確認版本與狀態。
func (s *Service) endNow(ctx context.Context, q *store.Queries, locked store.SellerStore, row store.SellerMeasure, statement string, adminID int64, now time.Time) (store.SellerMeasure, error) {
	updated, err := q.AdvanceMeasureRevision(ctx, store.AdvanceMeasureRevisionParams{
		ID: row.ID, ExpectedVersion: row.Version, StatementToSeller: statement, EndsAt: &now, AdminID: &adminID, Now: now,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.SellerMeasure{}, errMeasureVersion()
	}
	if err != nil {
		return store.SellerMeasure{}, err
	}
	if err := suspension.Recompute(ctx, q, s.d.Events, s.d.Jobs, s.d.Exposure, locked.AccountID, now); err != nil {
		return store.SellerMeasure{}, err
	}
	if err := events.Publish(ctx, s.d.Events, model.MeasureEnded{
		EventID: uuid.NewV7(), OccurredAt: now, MeasureID: row.ID, StoreID: row.StoreID, AccountID: locked.AccountID,
	}); err != nil {
		return store.SellerMeasure{}, err
	}
	return store.SellerMeasure(updated), nil
}

// ScheduleReview 重排 RISK 措施的複審期限（落在現在之後、30 天以內），並在同一個 transaction 寫一則內部註記說明理由。只限
// 生效中的 RISK 停權。後台的版本加一，賣家的版本不變（複審期限只給後台看）。
func (s *Service) ScheduleReview(ctx context.Context, in model.ScheduleReviewInput) (model.Measure, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.Measure{}, err
	}
	if in.ExpectedVersion <= 0 {
		return model.Measure{}, errExpectedVersion()
	}
	note, err := model.Required(in.Note, 2000, "Note", "內部註記")
	if err != nil {
		return model.Measure{}, err
	}
	reviewBy := in.ReviewBy.UTC().Truncate(time.Microsecond)
	var result model.Measure
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.now()
		if !reviewBy.After(now) || reviewBy.After(now.Add(model.ReviewWindow)) {
			return apperr.InvalidInputOn([]string{"ReviewBy"}, "複審期限必須晚於現在、不超過 30 天")
		}
		_, row, err := s.lockMeasure(ctx, q, in.MeasureID)
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
		if !current.CanScheduleReview() {
			return apperr.FailedPrecondition("只有生效中、依據風險類型的措施可以重排複審期限")
		}
		updated, err := q.SetMeasureReview(ctx, store.SetMeasureReviewParams{ID: row.ID, ExpectedVersion: in.ExpectedVersion, ReviewBy: reviewBy, Now: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return errMeasureVersion()
		}
		if err != nil {
			return err
		}
		if _, err := q.InsertGovernanceNote(ctx, store.InsertGovernanceNoteParams{
			NoteID: uuid.NewV7(), MeasureID: &row.ID, AdminID: in.AdminID, Body: note, Now: now,
		}); err != nil {
			return err
		}
		result, err = s.readRow(ctx, q, updated, detailAdmin, now)
		return err
	})
	return result, err
}
