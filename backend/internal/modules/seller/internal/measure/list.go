package measure

import (
	"bytes"
	"context"
	"slices"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// listRow 是各個列表查詢的共同形狀。
type listRow struct {
	row    store.SellerMeasure
	sortAt time.Time
}

func (s *Service) page(ctx context.Context, q *store.Queries, rows []listRow, size int32, now time.Time) (model.MeasurePage, error) {
	var page model.MeasurePage
	more := len(rows) > int(size)
	if more {
		rows = rows[:size]
	}
	measures := make([]store.SellerMeasure, len(rows))
	for i, r := range rows {
		measures[i] = r.row
	}
	items, err := s.hydrate(ctx, q, measures, detailList, now)
	if err != nil {
		return page, err
	}
	page.Items = items
	if more {
		last := rows[size-1]
		page.Next = &model.MeasureCursor{SortAt: last.sortAt, ID: last.row.ID}
	}
	return page, nil
}

func validState(state *model.MeasureState) (*string, error) {
	if state == nil {
		return nil, nil
	}
	if err := model.OneOf(*state, model.MeasureStateValues(), "State", "措施狀態不合法"); err != nil {
		return nil, err
	}
	return (*string)(state), nil
}

// stateIDs 回傳此刻生效中與預定生效的措施 ID（state 是 ACTIVE、SCHEDULED 或 ENDED 的篩選用；其餘狀態只看儲存的欄位，不需要。ENDED 是「不在生效中
// 也不在預定生效」，所以兩個集合都要）：與列表查詢在同一個快照，「生效中」的定義仍只有 EffectiveMeasures，「預定生效」只有 ScheduledMeasureIDs。
func (s *Service) stateIDs(ctx context.Context, q *store.Queries, accountID int64, state *string, force bool, now time.Time) (active, scheduled []uuid.UUID, err error) {
	if !force && (state == nil || (*state != string(model.StateActive) && *state != string(model.StateScheduled) && *state != string(model.StateEnded))) {
		return []uuid.UUID{}, []uuid.UUID{}, nil
	}
	if active, err = stint.ActiveMeasureIDs(ctx, q, accountID, now); err != nil {
		return nil, nil, err
	}
	scheduled, err = stint.ScheduledMeasureIDs(ctx, q, accountID, now)
	return active, scheduled, err
}

// AccountMeasures 是賣家自己的措施：帳號所有 stint 上的（措施的效果以帳號為單位），新到舊；state 篩選此刻的狀態。
func (s *Service) AccountMeasures(ctx context.Context, in model.AccountMeasureListInput) (model.MeasurePage, error) {
	if err := model.Positive(in.AccountID); err != nil {
		return model.MeasurePage{}, err
	}
	size, err := pageSize(in.PageSize)
	if err != nil {
		return model.MeasurePage{}, err
	}
	state, err := validState(in.State)
	if err != nil {
		return model.MeasurePage{}, err
	}
	var page model.MeasurePage
	err = s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.now()
		active, scheduled, err := s.stateIDs(ctx, q, in.AccountID, state, false, now)
		if err != nil {
			return err
		}
		params := store.AccountMeasuresParams{AccountID: in.AccountID, State: state, ActiveIds: active, ScheduledIds: scheduled, PageSize: size + 1}
		if in.After != nil {
			params.AfterAt, params.AfterID = &in.After.SortAt, &in.After.ID
		}
		rows, err := q.AccountMeasures(ctx, params)
		if err != nil {
			return err
		}
		list := make([]listRow, len(rows))
		for i, r := range rows {
			list[i] = listRow{row: r, sortAt: r.CreatedAt}
		}
		page, err = s.page(ctx, q, list, size, now)
		return err
	})
	return page, err
}

// AdminMeasures 是 admin 的措施列表。預設新到舊；收件匣排序的優先序是 ReviewDue（只列複審到期的 RISK 措施：成員是 seller.review_measure_review(now)，
// 生效中、現在不早於 review_by 前 24 小時，依 review_by 舊到新）、RequestState 是 PENDING（待處理請求的收件匣，申覆與解除請求在同一條查詢，依請求提出時間舊到新），
// 其餘篩選照常套用（組合合法）。RequestState 是集合篩選：措施上有任一筆請求（RequestKind 不為 nil 時只看那個種類）對外的狀態
// （model.Measure.EffectiveRequest）是這個狀態，所以措施已結束而儲存為 PENDING 的解除請求算 CLOSED、不在 PENDING 的收件匣裡。
func (s *Service) AdminMeasures(ctx context.Context, in model.AdminMeasureListInput) (model.MeasurePage, error) {
	size, err := pageSize(in.PageSize)
	if err != nil {
		return model.MeasurePage{}, err
	}
	if in.StoreID < 0 || in.AccountID < 0 {
		return model.MeasurePage{}, apperr.InvalidInput("ID 必須為正數")
	}
	state, err := validState(in.State)
	if err != nil {
		return model.MeasurePage{}, err
	}
	var basis, requestState, requestKind *string
	if in.Basis != nil {
		if err := model.OneOf(*in.Basis, model.BasisValues(), "Basis", "措施依據不合法"); err != nil {
			return model.MeasurePage{}, err
		}
		basis = (*string)(in.Basis)
	}
	if in.RequestState != nil {
		if err := model.OneOf(*in.RequestState, model.RequestStateValues(), "RequestState", "請求狀態不合法"); err != nil {
			return model.MeasurePage{}, err
		}
		requestState = (*string)(in.RequestState)
	}
	if in.RequestKind != nil {
		if err := model.OneOf(*in.RequestKind, model.RequestKindValues(), "RequestKind", "請求種類不合法"); err != nil {
			return model.MeasurePage{}, err
		}
		requestKind = (*string)(in.RequestKind)
	}
	var storeID, accountID *int64
	if in.StoreID > 0 {
		storeID = &in.StoreID
	}
	if in.AccountID > 0 {
		accountID = &in.AccountID
	}
	inbox := requestState != nil && *requestState == string(model.RequestPending)
	var page model.MeasurePage
	err = s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.now()
		var after *time.Time
		var afterID *uuid.UUID
		if in.After != nil {
			after, afterID = &in.After.SortAt, &in.After.ID
		}
		// 請求對外的狀態要看措施是否生效中，所以有請求篩選時 active_ids 一定要取（「生效中」的定義只有 EffectiveMeasures）。
		active, scheduled, err := s.stateIDs(ctx, q, in.AccountID, state, requestState != nil, now)
		if err != nil {
			return err
		}
		var list []listRow
		switch {
		case in.ReviewDue:
			rows, err := q.AdminMeasuresReviewDue(ctx, store.AdminMeasuresReviewDueParams{
				ActiveIds: active, ScheduledIds: scheduled, Now: now, StoreID: storeID, AccountID: accountID, Basis: basis, RequestState: requestState, RequestKind: requestKind, State: state,
				SourceCaseID: in.SourceCaseID, SourceReportID: in.SourceReportID, SourceAuthorityOrderID: in.SourceAuthorityOrderID, AfterAt: after, AfterID: afterID, PageSize: size + 1,
			})
			if err != nil {
				return err
			}
			for _, r := range rows {
				list = append(list, listRow{row: r.SellerMeasure, sortAt: *r.SortAt})
			}
		case inbox:
			rows, err := q.AdminMeasuresPendingRequests(ctx, store.AdminMeasuresPendingRequestsParams{
				StoreID: storeID, AccountID: accountID, Basis: basis, RequestKind: requestKind, State: state, ActiveIds: active, ScheduledIds: scheduled,
				SourceCaseID: in.SourceCaseID, SourceReportID: in.SourceReportID, SourceAuthorityOrderID: in.SourceAuthorityOrderID, AfterAt: after, AfterID: afterID, PageSize: size + 1,
			})
			if err != nil {
				return err
			}
			for _, r := range rows {
				list = append(list, listRow{row: r.SellerMeasure, sortAt: r.SortAt})
			}
		default:
			rows, err := q.AdminMeasures(ctx, store.AdminMeasuresParams{
				StoreID: storeID, AccountID: accountID, Basis: basis, RequestState: requestState, RequestKind: requestKind, State: state, ActiveIds: active, ScheduledIds: scheduled,
				SourceCaseID: in.SourceCaseID, SourceReportID: in.SourceReportID, SourceAuthorityOrderID: in.SourceAuthorityOrderID, AfterAt: after, AfterID: afterID, PageSize: size + 1,
			})
			if err != nil {
				return err
			}
			for _, r := range rows {
				list = append(list, listRow{row: r, sortAt: r.CreatedAt})
			}
		}
		page, err = s.page(ctx, q, list, size, now)
		return err
	})
	return page, err
}

// StoreWithActive 讀取指定的 stint（含已關閉的）與帳號此刻生效中的停權措施：店家、停權狀態、措施內容與可建立的終止事由（TerminationGrounds）來自同一個快照、同一個
// 時間點，中間有人結束、撤銷或到期也不會讀到互相矛盾的結果。
func (s *Service) StoreWithActive(ctx context.Context, storeID int64) (model.StoreMeasures, error) {
	if err := model.Positive(storeID); err != nil {
		return model.StoreMeasures{}, err
	}
	var result model.StoreMeasures
	err := s.d.DB.InReadSnapshot(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.now()
		row, err := q.StoreByID(ctx, storeID)
		if err != nil {
			return model.Error(err)
		}
		if result.Store, err = stint.Load(ctx, q, row, now); err != nil {
			return err
		}
		ids := make([]uuid.UUID, len(result.Store.EffectiveMeasures()))
		for i, m := range result.Store.EffectiveMeasures() {
			ids[i] = m.MeasureID
		}
		rows, err := q.MeasuresByIDs(ctx, ids)
		if err != nil {
			return err
		}
		if result.Active, err = s.hydrate(ctx, q, rows, detailList, now); err != nil {
			return err
		}
		latest, err := q.LatestStoreOfAccount(ctx, row.AccountID)
		if err != nil {
			return err
		}
		result.Terminations = model.NewTerminationEffects(model.AvailableTerminationGrounds(model.TerminationTarget{
			Closed: row.Status == string(model.Closed), LatestOfAccount: latest.StoreID == row.StoreID,
		}), now)
		result.CreateMeasure = result.Store.ActionUnavailableReasons(model.StoreActionCreateMeasure)
		slices.SortFunc(result.Active, func(a, b model.Measure) int {
			if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
				return c
			}
			return bytes.Compare(b.ID[:], a.ID[:])
		})
		return nil
	})
	return result, err
}
