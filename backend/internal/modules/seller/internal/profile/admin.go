package profile

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

// AdminList 是後台的店家清單：預設只列目前的店家（營運中與等待刪除），已關閉的要在 Statuses 明列；店名或電話模糊搜尋，或
// store_code 相同；帳號；依目前那一筆 KYC 申請的狀態、PayUni 註冊的綁定狀態（兩者都是儲存的事實）、帳號此刻是否停權篩選；
// 新到舊的 keyset，預設 20、上限 100。停權的篩選用 stint.SuspendedAccounts（和能力檢查同一支 EffectiveMeasures），
// 不在清單的 SQL 再寫一份「生效中」的定義。每一頁的停權狀態經 stint.EffectiveMany、KYC 流程經 exposure.KYCFlows（顯示用的
// model.DeriveKYCFlow）、條件經 stint.WithConditions，固定五次查詢，與頁面大小無關。
func (s *Service) AdminList(ctx context.Context, in model.StoreListInput) (model.StorePage, error) {
	size, err := pagination.PageSize(in.PageSize, 20, 100)
	if err != nil {
		return model.StorePage{}, err
	}
	statuses := in.Statuses
	if len(statuses) == 0 {
		statuses = []model.Status{model.Active, model.PendingDeletion}
	}
	params := store.AdminStoresParams{PageSize: size + 1, QueryPattern: model.LikePattern(in.Query), KycAwaitingReview: in.KYCAwaitingReview, Now: s.d.Now().UTC()}
	for _, status := range statuses {
		if !slices.Contains(model.StatusValues(), status) {
			return model.StorePage{}, apperr.InvalidInputOn([]string{"Statuses"}, "店家狀態不合法")
		}
		params.Statuses = append(params.Statuses, string(status))
	}
	if code, ok := storeCode(in.Query); ok {
		params.StoreCode = &code
	}
	if in.KYCStatus != nil {
		// 篩選的是目前那一筆還沒取消的申請：草稿是賣家自己的，已取消的不是目前的申請。
		if !slices.Contains(filterableKYCStatuses, *in.KYCStatus) {
			return model.StorePage{}, apperr.InvalidInputOn([]string{"KYCStatus"}, "申請狀態不合法")
		}
		status := string(*in.KYCStatus)
		params.KycStatus = &status
	}
	if in.PayUniBindStatus != nil {
		if !slices.Contains(model.PayUniBindStatusValues(), *in.PayUniBindStatus) {
			return model.StorePage{}, apperr.InvalidInputOn([]string{"PayUniBindStatus"}, "綁定狀態不合法")
		}
		status := string(*in.PayUniBindStatus)
		params.BindStatus = &status
	}
	if in.AccountID != nil {
		if err := model.Positive(*in.AccountID); err != nil {
			return model.StorePage{}, apperr.InvalidInputOn([]string{"AccountID"}, "帳號 ID 不合法")
		}
		params.AccountID = in.AccountID
	}
	params.SuspendedAccounts = []int64{}
	if in.Suspended != nil {
		params.Suspended = in.Suspended
		accounts, err := stint.SuspendedAccounts(ctx, store.New(s.d.DB.Conn(ctx)), s.d.Now().UTC())
		if err != nil {
			return model.StorePage{}, err
		}
		params.SuspendedAccounts = accounts
	}
	if in.After != nil {
		params.AfterCreatedAt, params.AfterID = &in.After.CreatedAt, &in.After.StoreID
	}
	q := store.New(s.d.DB.Conn(ctx))
	rows, err := q.AdminStores(ctx, params)
	if err != nil {
		return model.StorePage{}, err
	}
	hasMore := len(rows) > int(size)
	if hasMore {
		rows = rows[:size]
	}
	page := model.StorePage{Items: make([]model.StoreListItem, 0, len(rows))}
	if len(rows) == 0 {
		return page, nil
	}
	storeIDs, accountIDs := make([]int64, len(rows)), make([]int64, len(rows))
	for i, row := range rows {
		storeIDs[i], accountIDs[i] = row.StoreID, row.AccountID
	}
	suspensions, err := stint.EffectiveMany(ctx, q, accountIDs, s.d.Now().UTC())
	if err != nil {
		return model.StorePage{}, err
	}
	flows, err := s.d.Exposure.KYCFlows(ctx, q, storeIDs)
	if err != nil {
		return model.StorePage{}, err
	}
	for _, row := range rows {
		item := model.StoreListItem{Store: stint.ToStore(row), KYCFlow: model.FlowNotStarted}
		item.Store.Sanctions = suspensions[row.AccountID]
		if flow, ok := flows[row.StoreID]; ok {
			item.KYCFlow = flow
		}
		page.Items = append(page.Items, item)
	}
	listed := make([]model.Store, len(page.Items))
	for i, item := range page.Items {
		listed[i] = item.Store
	}
	if err := stint.WithConditions(ctx, q, listed); err != nil {
		return model.StorePage{}, err
	}
	if err := stint.WithClosures(ctx, q, listed); err != nil {
		return model.StorePage{}, err
	}
	for i := range page.Items {
		page.Items[i].Store = listed[i]
	}
	food := make([]*model.Store, len(page.Items))
	for i := range page.Items {
		food[i] = &page.Items[i].Store
	}
	if err := stint.AttachFood(ctx, q, food...); err != nil {
		return model.StorePage{}, err
	}
	if hasMore {
		last := page.Items[len(page.Items)-1].Store
		page.Next = &model.StoreCursor{CreatedAt: last.CreatedAt, StoreID: last.StoreID}
	}
	return page, nil
}

// filterableKYCStatuses 是清單可以篩選的申請狀態：核准與退回（儲存的事實）；審核中不在這裡：「待審核」是待辦的成員（KYCAwaitingReview），草稿與已取消不是「目前送出的申請」。
var filterableKYCStatuses = []model.KYCStatus{model.KYCStatusApproved, model.KYCStatusRejected}

// storeCode 回報關鍵字是不是 store_code（只有數字、1 到 10 位、不以 0 開頭：電話號碼不會被當成店號）。
func storeCode(keyword string) (int64, bool) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || len(keyword) > 10 || keyword[0] == '0' {
		return 0, false
	}
	var code int64
	for _, c := range keyword {
		if c < '0' || c > '9' {
			return 0, false
		}
		code = code*10 + int64(c-'0')
	}
	return code, code >= 1
}

// AdminStats 是後台的店家統計，一句查詢、同一個 snapshot；「本月」是台北時間的日曆月。
func (s *Service) AdminStats(ctx context.Context) (model.StoreStats, error) {
	now := s.d.Now().In(twtime.Taipei)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, twtime.Taipei)
	row, err := store.New(s.d.DB.Conn(ctx)).AdminStoreStats(ctx, store.AdminStoreStatsParams{
		Now: s.d.Now().UTC(), MonthStart: start.UTC(), MonthEnd: start.AddDate(0, 1, 0).UTC(),
	})
	if err != nil {
		return model.StoreStats{}, err
	}
	return model.StoreStats{
		Total: row.Total, KYCAwaitingReview: row.KycAwaitingReview, KYCApproved: row.KycApproved, KYCRejected: row.KycRejected,
		NewThisMonth: row.NewThisMonth,
	}, nil
}
