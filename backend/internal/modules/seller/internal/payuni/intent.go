package payuni

import (
	"context"
	"errors"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// BindIntent 是核准時寫入綁定意圖的輸入：店家與核准的申請。
type BindIntent struct {
	Store       store.SellerStore
	Application store.SellerKycApplication
}

// IntentParams 是綁定意圖的欄位：MerID 由 store_code 組成，綁定與同步送出的聯絡資料在 job 執行時讀店家目前的欄位，這裡不複製。
// 核准與本機開發種子（seller 的 SeedDevStores）共用，意圖的欄位只有這一處定義。
func IntentParams(in BindIntent, now time.Time) store.UpsertPayUniIntentParams {
	return store.UpsertPayUniIntentParams{
		StoreID: in.Store.StoreID, MerID: model.FormatMerID(in.Store.StoreCode), ApplicationID: in.Application.ApplicationID, Now: now,
	}
}

// RequestBind 在核准的 transaction 內寫入（或刷新）綁定意圖：bind_status 回到 PENDING，並排入以 store_id 為唯一鍵的 sync job。
// 呼叫方已鎖住店家與申請列。這裡不呼叫 PayUni（規則 5）。
func RequestBind(ctx context.Context, q *store.Queries, jc jobs.Enqueuer, in BindIntent, now time.Time) (model.PayUniRegistration, error) {
	row, err := q.UpsertPayUniIntent(ctx, IntentParams(in, now))
	if err != nil {
		return model.PayUniRegistration{}, err
	}
	if err := enqueueSync(ctx, jc, row.StoreID); err != nil {
		return model.PayUniRegistration{}, err
	}
	return stint.ToPayUniRegistration(row), nil
}

// Resume 在店家恢復營業的 transaction 內，把關店期間被放棄的綁定與同步補排回去：註冊還在等綁定（PENDING），或已綁定而有資料還沒送出
// （synced_seq 落後 intent_seq）時排入 sync job（以 store_id 為唯一鍵）；已失敗的要管理員重新綁定、已同步的什麼都不做，沒有註冊列也什麼都不做。
// 同步 job 在店家關閉時不做事（claim 跳過已關閉的店），所以關店期間排到的 job 已經結束，不補就要等下一輪掃描。呼叫方已鎖住店家列。
func Resume(ctx context.Context, q *store.Queries, jc jobs.Enqueuer, storeID int64) error {
	reg, err := q.LockPayUniRegistration(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	switch {
	case reg.BindStatus == string(model.BindPending), reg.BindStatus == string(model.BindBound) && reg.SyncedSeq < reg.IntentSeq:
		return enqueueSync(ctx, jc, storeID)
	}
	return nil
}

// RequestSync 在呼叫方的 transaction 內登記「需要送給 PayUni 的資料改變了」（店家改地址或聯絡資料）：intent_seq 加一並排入同一個
// sync job（以 store_id 為唯一鍵，連續的修改自然合併；job 讀的是執行當下的值，PENDING 期間改的資料也會被送到，盤點 D18）。
// 店家還沒有註冊列（還沒核准）時什麼都不做：核准之後的 job 會讀當時的資料。呼叫方已鎖住店家列。
func RequestSync(ctx context.Context, q *store.Queries, jc jobs.Enqueuer, storefrontBaseURL string, storeID int64, now time.Time) error {
	reg, err := q.LockPayUniRegistration(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return requestSync(ctx, q, jc, storefrontBaseURL, reg, now)
}

// requestSync 為已鎖住的註冊列登記一次需要同步的資料改變。FAILED 的列通常要管理員重新綁定，唯一例外是失敗的原因就是
// 資料不完整：而且這次改動讓 bind 需要的資料補齊了（bindLocation 的完整性檢查，與 job 同一份定義）才自動回到 PENDING 重送；
// 還是不完整就維持 FAILED，不發事件、不重置嘗試次數（每次不相干的修改都重跑一次註定失敗的 job，只會多寫一筆失敗事件）。
func requestSync(ctx context.Context, q *store.Queries, jc jobs.Enqueuer, storefrontBaseURL string, reg store.SellerPayuniRegistration, now time.Time) error {
	var err error
	switch {
	case reg.BindStatus == string(model.BindFailed) && reg.BindErrorCode != nil && *reg.BindErrorCode == model.ErrIncompleteData:
		complete, cerr := bindDataComplete(ctx, q, reg, storefrontBaseURL)
		if cerr != nil {
			return cerr
		}
		if !complete {
			return nil
		}
		reg, err = q.RebindPayUni(ctx, store.RebindPayUniParams{StoreID: reg.StoreID, Now: now})
	default:
		reg, err = q.BumpPayUniIntent(ctx, store.BumpPayUniIntentParams{StoreID: reg.StoreID, Now: now})
	}
	if err != nil {
		return err
	}
	if reg.BindStatus == string(model.BindFailed) {
		return nil // 要管理員重新綁定；重新綁定時讀目前的資料
	}
	return enqueueSync(ctx, jc, reg.StoreID)
}

// bindDataComplete 回答 bind_merchant 需要的資料現在齊了沒有（店家的地址與聯絡資料，申請的官網或店家頁的網站根網址）。
func bindDataComplete(ctx context.Context, q *store.Queries, reg store.SellerPayuniRegistration, storefrontBaseURL string) (bool, error) {
	owner, err := q.StoreByID(ctx, reg.StoreID)
	if err != nil {
		return false, model.Error(err)
	}
	app, err := q.KYCApplicationByID(ctx, reg.ApplicationID)
	if err != nil {
		return false, err
	}
	_, _, _, missing := bindLocation(owner, model.Deref(app.WebsiteUrl), storefrontBaseURL)
	return len(missing) == 0, nil
}

// IdentityLocked 回答 PayUni 端的商店身分欄位現在能不能改：註冊列存在（有 MerID）而且 PayUni 已經（或正要）收下
// （bind_status 不是 FAILED）時為真。update_merchant 只能改聯絡方式、地址與支付設定，核心身分欄位（Name、UBN、MerName、
// MerEngName、MerURL、MerDesc、MerType、MerMCC、MCName、MCID 等）由 bind_merchant 建立後不可變更（PayUni 代理商專用
// API 總覽 §8），所以這時 bind 會送出的欄位都鎖住，錯誤只能關店、以新的 store_code 重新入駐。綁定失敗（FAILED，PayUni
// 沒有收下）時解鎖，讓賣家修正資料。讀取失敗時回錯誤（呼叫方拒絕修改）。呼叫方已鎖住店家列。
func IdentityLocked(ctx context.Context, q *store.Queries, storeID int64) (bool, error) {
	row, err := q.PayUniRegistrationByStore(ctx, storeID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return false, nil
	case err != nil:
		return false, err
	}
	return model.IdentityLockedBy(stint.ToPayUniRegistration(row)), nil
}

// CheckNameUnlocked 檢查店名與介紹（MerName、MerDesc 的來源）能不能改：KYC 審核中不行；PayUni 已經（或正要）收下註冊
// 之後也不行，包含被 PayUni 拒絕、申請改回草稿修正的期間（盤點 D17：舊實作只鎖 cert 在 submitted／forwarded，讀取失敗時
// 放行，而且草稿解鎖會讓我們的紀錄和 PayUni 的分歧）。綁定失敗（FAILED）時解鎖。讀取失敗時回錯誤、拒絕修改。
func CheckNameUnlocked(ctx context.Context, q *store.Queries, storeID int64) error {
	app, err := q.CurrentKYCApplication(ctx, storeID)
	switch {
	case err == nil && app.Status == string(model.KYCStatusSubmitted):
		return apperr.FailedPrecondition("KYC 審核中，不能修改店名與介紹")
	case err != nil && !errors.Is(err, pgx.ErrNoRows):
		return err
	}
	locked, err := IdentityLocked(ctx, q, storeID)
	if err != nil {
		return err
	}
	if locked {
		return apperr.FailedPrecondition("店名與介紹已送交金流審核，不能修改；要更正請關閉商店後重新入駐")
	}
	return nil
}

// Rebind 是管理員對 FAILED 的註冊重新綁定：回到 PENDING 並排入新的 job（不是再核准一次）。可用性與方法是同一個函式
// （model.RebindUnavailableReasons，與讀取時的 can_rebind 相同）：清單空就重新綁定；第一個原因是 PAYUNI_BIND_IN_PROGRESS（表上標成
// idempotent：已經在重新綁定）時重送安全、回傳目前的列；其餘原因（PayUni 端已終止、店家已關閉或等待刪除、已綁定、註冊的申請不是
// 已核准：賣家把它改回草稿重新修改，此時送給 PayUni 的會是還沒審核的資料）回 FailedPrecondition，訊息是第一個原因的句子。
// 回傳的註冊與 Registration 一致（帶申請的賣家型態、階段與重新綁定的原因）。
func (s *Service) Rebind(ctx context.Context, in model.RebindPayUniInput) (model.AdminPayUniRegistration, error) {
	if err := model.Positive(in.StoreID, in.AdminID); err != nil {
		return model.AdminPayUniRegistration{}, err
	}
	var out model.AdminPayUniRegistration
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		// 鎖店家但不擋已關閉：已關閉的店家是 failed_precondition（STORE_CLOSED），不是找不到。
		lockedStore, err := q.LockStore(ctx, in.StoreID)
		if err != nil {
			return model.Error(err)
		}
		reg, err := q.LockPayUniRegistration(ctx, in.StoreID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("店家還沒有 PayUni 註冊")
		}
		if err != nil {
			return err
		}
		now := s.d.Now().UTC()
		current := stint.ToPayUniRegistration(reg)
		view, err := adminViewOf(ctx, q, lockedStore, current, now)
		if err != nil {
			return err
		}
		switch reasons := view.RebindUnavailable; {
		case len(reasons) == 0:
		case model.StoreActionIdempotent(model.StoreActionRebindPayUni, reasons[0]):
			out = view
			return nil
		default:
			return model.ErrStoreActionUnavailable(reasons, model.StoreActionRebindPayUni)
		}
		reg, err = q.RebindPayUni(ctx, store.RebindPayUniParams{StoreID: in.StoreID, Now: now})
		if err != nil {
			return err
		}
		// 管理員的動作留下 append-only 的紀錄（誰、何時、哪一份申請的註冊）：沿用審核紀錄的形狀。
		admin := in.AdminID
		_, err = q.InsertKYCReview(ctx, store.InsertKYCReviewParams{
			ReviewID: uuid.NewV7(), ApplicationID: reg.ApplicationID, Action: string(model.ReviewRebind), ReviewerAdminID: &admin,
			Now: now,
		})
		if err != nil {
			return err
		}
		if err := enqueueSync(ctx, s.d.Jobs, in.StoreID); err != nil {
			return err
		}
		out, err = adminViewOf(ctx, q, lockedStore, stint.ToPayUniRegistration(reg), now)
		return err
	})
	return out, err
}

// Registration 讀店家的 PayUni 註冊（含 PayUni 的狀態、所在的階段與重新綁定的可用性）；沒有時回 NotFound。
func (s *Service) Registration(ctx context.Context, storeID int64) (model.AdminPayUniRegistration, error) {
	if err := model.Positive(storeID); err != nil {
		return model.AdminPayUniRegistration{}, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	row, err := q.PayUniRegistrationByStore(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AdminPayUniRegistration{}, apperr.NotFound("店家還沒有 PayUni 註冊")
	}
	if err != nil {
		return model.AdminPayUniRegistration{}, err
	}
	owner, err := q.StoreByID(ctx, storeID)
	if err != nil {
		return model.AdminPayUniRegistration{}, model.Error(err)
	}
	return adminViewOf(ctx, q, owner, stint.ToPayUniRegistration(row), s.d.Now().UTC())
}

// adminViewOf 組出後台看到的註冊：註冊帶申請的賣家型態（withApplication）、所在的 PayUni 階段（model.PayUniPhaseOf，與賣家的流程狀態同一個函式），以及
// 重新綁定的原因（model.RebindUnavailableReasons，店家以 stint.Load 載入）。註冊所根據的申請的狀態與賣家型態來自同一次讀取。
func adminViewOf(ctx context.Context, q *store.Queries, owner store.SellerStore, current model.PayUniRegistration, now time.Time) (model.AdminPayUniRegistration, error) {
	reg, appStatus, err := withApplication(ctx, q, current)
	if err != nil {
		return model.AdminPayUniRegistration{}, err
	}
	st, err := stint.Load(ctx, q, owner, now)
	if err != nil {
		return model.AdminPayUniRegistration{}, err
	}
	return model.AdminPayUniRegistration{
		PayUniRegistration: reg,
		FlowStatus:         model.PayUniPhaseOf(reg),
		RebindUnavailable:  model.RebindUnavailableReasons(st, reg, appStatus),
	}, nil
}

// withApplication 帶上註冊的申請的賣家型態（要補的欄位依它過濾，見 model.ProblemOf），以及綁定因資料不完整而失敗時店家現在實際缺的欄位（問題只指向真的缺的店家欄位），
// 另回傳註冊所根據的申請的狀態（同一次讀取）。
func withApplication(ctx context.Context, q *store.Queries, reg model.PayUniRegistration) (model.PayUniRegistration, model.KYCStatus, error) {
	app, err := q.KYCApplicationByID(ctx, reg.ApplicationID)
	if err != nil {
		return reg, "", err
	}
	reg.SellerType = model.KYCSellerType(app.SellerType)
	if reg.BindErrorCode != nil && *reg.BindErrorCode == model.ErrIncompleteData {
		owner, err := q.StoreByID(ctx, reg.StoreID)
		if err != nil {
			return reg, "", model.Error(err)
		}
		reg.IncompleteStoreFields = MissingStoreFields(stint.ToStore(owner).Profile)
	}
	return reg, model.KYCStatus(app.Status), nil
}

// Observations 回傳店家的觀測紀錄，新到舊的 keyset（事件時間、觀測 ID），預設 20、上限 100。店家不存在回 NotFound（不是空的
// 一頁）。
func (s *Service) Observations(ctx context.Context, in model.PayUniObservationListInput) (model.PayUniObservationPage, error) {
	if err := model.Positive(in.StoreID); err != nil {
		return model.PayUniObservationPage{}, err
	}
	size, err := pagination.PageSize(in.PageSize, 20, 100)
	if err != nil {
		return model.PayUniObservationPage{}, err
	}
	q := store.New(s.d.DB.Conn(ctx))
	if _, err := q.StoreByID(ctx, in.StoreID); err != nil {
		return model.PayUniObservationPage{}, model.Error(err)
	}
	params := store.PayUniObservationsParams{StoreID: in.StoreID, PageSize: size + 1}
	if in.After != nil {
		params.AfterExecutedAt, params.AfterID = &in.After.ExecutedAt, &in.After.ObservationID
	}
	rows, err := q.PayUniObservations(ctx, params)
	if err != nil {
		return model.PayUniObservationPage{}, err
	}
	hasMore := len(rows) > int(size)
	if hasMore {
		rows = rows[:size]
	}
	page := model.PayUniObservationPage{Items: make([]model.PayUniObservation, 0, len(rows))}
	for _, r := range rows {
		page.Items = append(page.Items, toObservation(r))
	}
	if hasMore {
		last := page.Items[len(page.Items)-1]
		page.Next = &model.PayUniObservationCursor{ExecutedAt: last.ExecutedAt, ObservationID: last.ID}
	}
	return page, nil
}

// CheckResubmit 回答申請此刻能不能儲存草稿或重新送出（流程狀態的 CanResubmit）：能時回 nil，否則是 FailedPrecondition，訊息是
// model.ResubmitUnavailableReasons 第一個原因的句子（語系取自 ctx）。呼叫方已鎖住店家列與 PayUni 註冊列（需要時）。
func CheckResubmit(ctx context.Context, q *store.Queries, row store.SellerKycApplication) error {
	v, err := SellerViewOf(ctx, q, row)
	if err != nil {
		return err
	}
	if v.Flow.CanResubmit {
		return nil
	}
	return model.ErrResubmitUnavailable(v.Reasons, uitext.LocaleFrom(ctx))
}

// SellerView 是申請在同一個時刻、同一份 PayUni 註冊上的流程狀態、賣家看到的 PayUni 狀態與不能重新送出的原因：註冊只載入一次
// （帶申請的賣家型態與店家現在缺的欄位），三者都由它推導，所以 Flow.CanResubmit、PayUni.State 與 Reasons 不會分歧。PayUni 狀態沒有時是 nil。
type SellerView struct {
	Flow    model.KYCFlow
	PayUni  *model.SellerPayUni
	Reasons []model.KYCResubmitUnavailableReason
}

// SellerViewOf 讀申請所屬店家的註冊並推導 SellerView；賣家讀申請、SaveKYCDraft 與 SubmitKYC 的檢查都經它。
func SellerViewOf(ctx context.Context, q *store.Queries, row store.SellerKycApplication) (SellerView, error) {
	var reg *model.PayUniRegistration
	regRow, err := q.PayUniRegistrationByStore(ctx, row.StoreID)
	switch {
	case err == nil:
		r, _, err := withApplication(ctx, q, stint.ToPayUniRegistration(regRow))
		if err != nil {
			return SellerView{}, err
		}
		reg = &r
	case !errors.Is(err, pgx.ErrNoRows):
		return SellerView{}, err
	}
	status := model.KYCStatus(row.Status)
	v := SellerView{Flow: model.DeriveKYCFlow(status, reg)}
	if state, ok := model.SellerPayUniOf(status, reg != nil && reg.ApplicationID == row.ApplicationID, reg); ok {
		v.PayUni = &state
	}
	v.Reasons = model.ResubmitUnavailableReasons(status, v.Flow, v.PayUni)
	return v, nil
}
