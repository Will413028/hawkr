// Package foodreg 是食品業者登錄的審核（食安法第 8 條第 3 項）：賣家送審、撤回待審、清除登錄，管理員人工對照食藥署的登錄資料後
// 核准或退回，核准之後也可以撤銷（資源層級的更正）。核准的那一筆是店家公開字號的唯一來源（stint.AttachFood），店家表沒有食品欄位。寫入的人先鎖店家列再動登錄列
// （鎖順序：店家、登錄）；每家店最多一筆待審、最多一筆核准由 food_registrations 的兩個 partial UNIQUE 保證。核准、清除與撤銷改變公開
// 的店家資料，在同一個 transaction 內發 StoreProfileChanged 並排入快取失效（公開的 update_time 由 model.PublicStoreUpdateTime 從
// 登錄的核准、清除與撤銷時間推導，不改 stores）；裁定時發 FoodRegistrationDecided、平台撤銷時發 FoodRegistrationRevoked，送達賣家是 A8 的 notification 的工作，目前還沒有訂閱者。
package foodreg

import (
	"context"
	"errors"
	"slices"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/agreement"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/exposure"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/auditreason"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/pagination"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
)

// Deps 外部依賴；clock 由組裝點注入。
type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Jobs   jobs.Enqueuer
	// Exposure 提供 KYC 流程的狀態：寫入在鎖住店家之後以 model.Authorize 重新授權。
	Exposure *exposure.Service
	Now      func() time.Time
}

// Service 是食品業者登錄的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service { return &Service{d: d} }

// now 是寫入用的現在：截到微秒（Postgres 的 timestamptz 精度）。
func (s *Service) now() time.Time { return s.d.Now().UTC().Truncate(time.Microsecond) }

func errNotFound() error { return apperr.NotFound("找不到這筆食品業者登錄") }

// lockStore 鎖住店家列（所有寫入的第一步）；不存在回 NotFound。
func lockStore(ctx context.Context, q *store.Queries, storeID int64) (store.SellerStore, error) {
	row, err := q.LockStore(ctx, storeID)
	return row, model.Error(err)
}

// authorize 在店家列已鎖住之後以 CapStorefrontWrite 重新授權（讀取與拿到鎖之間，帳號可能已被停權、店家可能已進入等待刪除或關閉），
// 回傳授權時讀到的店家。
func (s *Service) authorize(ctx context.Context, q *store.Queries, locked store.SellerStore, now time.Time) (model.Store, error) {
	loaded, err := stint.Load(ctx, q, locked, now)
	if err != nil {
		return model.Store{}, err
	}
	return loaded, model.Authorize(loaded, model.CapStorefrontWrite)
}

// Submit 賣家送出食品業者登錄供平台核對，需要 CapStorefrontWrite。字號去掉前後空白後 1 到 50 字，不檢查格式（沒有官方規格，
// 交給人工核對）。已有待審時：字號相同就沿用它（重送不改變排隊的時間），不同就撤回舊的、換成新的，所以每家店只有一筆待審。字號與
// 目前核准的相同時不建立新登錄：有待審的就撤回它（賣家要的就是目前核准的那一個），沒有就什麼都不做。目前核准的登錄在新的被核准之前
// 照常有效。AcceptedAgreements 先於一切業務檢查驗證（種類不對回 InvalidInput，版本不是現行版本回 FailedPrecondition 的
// AGREEMENT_OUTDATED）；建立新的登錄時，確認已閱讀的紀錄與它在同一個 transaction 寫入、綁定那一筆登錄。回傳送出之後的店家。
func (s *Service) Submit(ctx context.Context, in model.SubmitFoodRegistrationInput) (model.Store, error) {
	if err := model.Positive(in.StoreID); err != nil {
		return model.Store{}, err
	}
	if err := model.CheckAgreements(model.SubmitFoodRegistrationAgreements, in.AcceptedAgreements); err != nil {
		return model.Store{}, err
	}
	number, err := normalizeNumber(in.RegistrationNumber)
	if err != nil {
		return model.Store{}, err
	}
	var out model.Store
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.now()
		locked, err := lockStore(ctx, q, in.StoreID)
		if err != nil {
			return err
		}
		before, err := s.authorize(ctx, q, locked, now)
		if err != nil {
			return err
		}
		if before.ApprovedFoodRegistration != nil && before.ApprovedFoodRegistration.RegistrationNumber == number {
			if _, err := q.WithdrawPendingFoodRegistration(ctx, store.WithdrawPendingFoodRegistrationParams{StoreID: in.StoreID, Now: now}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			out, err = stint.Load(ctx, q, locked, now)
			return err
		}
		pending, err := q.PendingFoodRegistration(ctx, in.StoreID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return err
		case model.Deref(pending.RegistrationNumber) == number:
			out = before
			return nil
		default:
			if _, err := q.WithdrawPendingFoodRegistration(ctx, store.WithdrawPendingFoodRegistrationParams{StoreID: in.StoreID, Now: now}); err != nil {
				return err
			}
		}
		created, err := q.InsertFoodRegistration(ctx, store.InsertFoodRegistrationParams{
			ID: uuid.NewV7(), StoreID: in.StoreID, AccountID: locked.AccountID, RegistrationNumber: &number, Now: now,
		})
		if err != nil {
			return err
		}
		if err := agreement.Record(ctx, q, in.StoreID, locked.AccountID, in.AcceptedAgreements, now,
			agreement.Subject{FoodRegistrationID: &created.ID}, agreement.Source{ClientIP: in.ClientIP, UserAgent: in.UserAgent}); err != nil {
			return err
		}
		out, err = stint.Load(ctx, q, locked, now)
		return err
	})
	return out, err
}

// Withdraw 賣家撤回待審的登錄，目前核准的那一筆保留，需要 CapStorefrontWrite。沒有待審的回 FailedPrecondition。
func (s *Service) Withdraw(ctx context.Context, storeID int64) (model.Store, error) {
	if err := model.Positive(storeID); err != nil {
		return model.Store{}, err
	}
	var out model.Store
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.now()
		locked, err := lockStore(ctx, q, storeID)
		if err != nil {
			return err
		}
		if _, err := s.authorize(ctx, q, locked, now); err != nil {
			return err
		}
		if _, err := q.WithdrawPendingFoodRegistration(ctx, store.WithdrawPendingFoodRegistrationParams{StoreID: storeID, Now: now}); errors.Is(err, pgx.ErrNoRows) {
			return apperr.FailedPrecondition("沒有待審的食品業者登錄")
		} else if err != nil {
			return err
		}
		out, err = stint.Load(ctx, q, locked, now)
		return err
	})
	return out, err
}

// Clear 賣家清除登錄（不再賣加工食品），需要 CapStorefrontWrite：結束目前核准的那一筆（CLEARED），同時撤回待審的，立即生效：
// 撤回一項主張不會誤導買家。兩者都沒有時回 FailedPrecondition。核准的那一筆結束時，店家公開的字號跟著消失：發 StoreProfileChanged、
// 排入快取失效。
func (s *Service) Clear(ctx context.Context, storeID int64) (model.Store, error) {
	if err := model.Positive(storeID); err != nil {
		return model.Store{}, err
	}
	var out model.Store
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.now()
		locked, err := lockStore(ctx, q, storeID)
		if err != nil {
			return err
		}
		if _, err := s.authorize(ctx, q, locked, now); err != nil {
			return err
		}
		ended := true
		if _, err := q.EndApprovedFoodRegistration(ctx, store.EndApprovedFoodRegistrationParams{StoreID: storeID, EndCause: string(model.FoodRegCleared), Now: now}); errors.Is(err, pgx.ErrNoRows) {
			ended = false
		} else if err != nil {
			return err
		}
		withdrawn := true
		if _, err := q.WithdrawPendingFoodRegistration(ctx, store.WithdrawPendingFoodRegistrationParams{StoreID: storeID, Now: now}); errors.Is(err, pgx.ErrNoRows) {
			withdrawn = false
		} else if err != nil {
			return err
		}
		if !ended && !withdrawn {
			return apperr.FailedPrecondition("沒有可以清除的食品業者登錄")
		}
		if ended {
			if err := s.publicChanged(ctx, locked, now); err != nil {
				return err
			}
		}
		out, err = stint.Load(ctx, q, locked, now)
		return err
	})
	return out, err
}

// publicChanged 在公開的字號改變（核准、清除、撤銷）的 transaction 內發 StoreProfileChanged、排入快取失效。公開的 update_time 由
// model.PublicStoreUpdateTime 從登錄的核准、清除與撤銷時間推導，這裡不改 stores（版本與 updated_at 都不動：登錄不是店家資料的編輯）。
func (s *Service) publicChanged(ctx context.Context, locked store.SellerStore, now time.Time) error {
	if err := events.Publish(ctx, s.d.Events, model.StoreProfileChanged{EventID: uuid.NewV7(), OccurredAt: now, StoreID: locked.StoreID}); err != nil {
		return err
	}
	return stint.Invalidate(ctx, s.d.Jobs, locked)
}

// List 後台的清單。沒有指定狀態或 PENDING 是收件匣：營運中的店家的待審件，再依 Inbox 分隊列（預設 DECIDABLE：店家的 KYC 已核准、
// 現在可以裁定的件，成員讀 seller.review_food_registration，依期限由近到遠，keyset 是 (期限, id)；AWAITING_KYC：還在等 KYC 核准的件，
// 送出時間舊到新，keyset 是 (送出時間, id)）；指定其他狀態是歷史，新到舊、店家的狀態不限，不可以指定 Inbox。預設 20、上限 100；
// Query 是字號或店名的子字串。每一筆另帶裁定期限（DecideDeadline），讀同一支函式。
func (s *Service) List(ctx context.Context, in model.FoodRegistrationListInput) (model.FoodRegistrationPage, error) {
	size, err := pagination.PageSize(in.PageSize, 20, 100)
	if err != nil {
		return model.FoodRegistrationPage{}, err
	}
	if in.State != nil && !slices.Contains(model.FoodRegStateValues(), *in.State) {
		return model.FoodRegistrationPage{}, apperr.InvalidInputOn([]string{"State"}, "狀態不合法")
	}
	if in.Inbox != nil && !slices.Contains(model.FoodRegInboxValues(), *in.Inbox) {
		return model.FoodRegistrationPage{}, apperr.InvalidInputOn([]string{"Inbox"}, "收件匣隊列不合法")
	}
	if in.State != nil && *in.State != model.FoodRegPending && in.Inbox != nil {
		return model.FoodRegistrationPage{}, apperr.InvalidInputOn([]string{"Inbox"}, "歷史清單沒有收件匣隊列")
	}
	if in.After != nil && !in.After.Valid() {
		return model.FoodRegistrationPage{}, apperr.InvalidInput("page_token 不合法")
	}
	pattern := model.LikePattern(in.Query)
	q := store.New(s.d.DB.Conn(ctx))
	now := s.now()
	var reviews []reviewRow
	// next 是游標的排序時間：決定於這一筆所在的隊列（可以裁定的是期限，其他是送出時間）。
	next := func(r reviewRow) time.Time { return r.reg.SubmittedAt }
	switch {
	case in.State != nil && *in.State != model.FoodRegPending:
		params := store.AdminFoodRegistrationsByStateParams{State: string(*in.State), QueryPattern: pattern, PageSize: size + 1}
		if in.After != nil {
			params.AfterID, params.AfterSubmittedAt = &in.After.ID, &in.After.SortAt
		}
		rows, err := q.AdminFoodRegistrationsByState(ctx, params)
		if err != nil {
			return model.FoodRegistrationPage{}, err
		}
		for _, r := range rows {
			reviews = append(reviews, reviewRow{reg: r.SellerFoodRegistration, storeName: r.StoreName, storeStatus: r.StoreStatus})
		}
	case in.Inbox != nil && *in.Inbox == model.FoodRegInboxAwaitingKYC:
		params := store.AdminFoodRegistrationAwaitingKYCParams{QueryPattern: pattern, PageSize: size + 1}
		if in.After != nil {
			params.AfterID, params.AfterSubmittedAt = &in.After.ID, &in.After.SortAt
		}
		rows, err := q.AdminFoodRegistrationAwaitingKYC(ctx, params)
		if err != nil {
			return model.FoodRegistrationPage{}, err
		}
		for _, r := range rows {
			reviews = append(reviews, reviewRow{reg: r.SellerFoodRegistration, storeName: r.StoreName, storeStatus: r.StoreStatus})
		}
	default:
		params := store.AdminFoodRegistrationDecidableParams{Now: now, QueryPattern: pattern, PageSize: size + 1}
		if in.After != nil {
			params.AfterID, params.AfterSortAt = &in.After.ID, &in.After.SortAt
		}
		rows, err := q.AdminFoodRegistrationDecidable(ctx, params)
		if err != nil {
			return model.FoodRegistrationPage{}, err
		}
		for _, r := range rows {
			reviews = append(reviews, reviewRow{reg: r.SellerFoodRegistration, storeName: r.StoreName, storeStatus: r.StoreStatus, sortAt: r.SortAt})
		}
		next = func(r reviewRow) time.Time { return r.sortAt }
	}
	hasMore := len(reviews) > int(size)
	if hasMore {
		reviews = reviews[:size]
	}
	page := model.FoodRegistrationPage{Items: make([]model.FoodRegistrationReview, 0, len(reviews))}
	if len(reviews) == 0 {
		return page, nil
	}
	related, err := loadRelated(ctx, q, reviews, now)
	if err != nil {
		return model.FoodRegistrationPage{}, err
	}
	for _, r := range reviews {
		page.Items = append(page.Items, r.toReview(related))
	}
	if hasMore {
		last := reviews[len(reviews)-1]
		page.Next = &model.FoodRegistrationCursor{SortAt: next(last), ID: last.reg.ID}
	}
	return page, nil
}

// Get 讀一筆登錄的後台檢視；不存在回 NotFound。
func (s *Service) Get(ctx context.Context, id uuid.UUID) (model.FoodRegistrationReview, error) {
	return s.review(ctx, store.New(s.d.DB.Conn(ctx)), id)
}

func (s *Service) review(ctx context.Context, q *store.Queries, id uuid.UUID) (model.FoodRegistrationReview, error) {
	row, err := q.FoodRegistrationReview(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.FoodRegistrationReview{}, errNotFound()
	}
	if err != nil {
		return model.FoodRegistrationReview{}, err
	}
	r := reviewRow{reg: row.SellerFoodRegistration, storeName: row.StoreName, storeStatus: row.StoreStatus}
	related, err := loadRelated(ctx, q, []reviewRow{r}, s.now())
	if err != nil {
		return model.FoodRegistrationReview{}, err
	}
	return r.toReview(related), nil
}

// Decide 管理員裁定待審的登錄。核准：店家要在營業中、帳號的 KYC 申請要是 APPROVED（核對的是已驗證過的主體，否則回
// FailedPrecondition）；目前核准的那一筆在同一個 transaction 結束（SUPERSEDED），新的成為店家公開的字號，並發
// StoreProfileChanged、排入快取失效。退回：Reason 必填，店家的公開字號不變。InternalNote 只給後台看（核准或退回都可以寫）。登錄
// 已不是待審（被撤回、被取代、已裁定）或店家已關閉回 FailedPrecondition。發出 FoodRegistrationDecided（帶字號與退回理由）。
func (s *Service) Decide(ctx context.Context, in model.DecideFoodRegistrationInput) (model.FoodRegistrationReview, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.FoodRegistrationReview{}, err
	}
	if in.Outcome != model.FoodRegApproved && in.Outcome != model.FoodRegRejected {
		return model.FoodRegistrationReview{}, apperr.InvalidInputOn([]string{"Outcome"}, "裁定只能是核准或退回")
	}
	var reason string
	if in.Outcome == model.FoodRegRejected {
		var err error
		if reason, err = auditreason.Required(in.Reason, "Reason"); err != nil {
			return model.FoodRegistrationReview{}, err
		}
	} else if in.Reason != "" {
		return model.FoodRegistrationReview{}, apperr.InvalidInputOn([]string{"Reason"}, "核准不帶退回理由")
	}
	note, err := auditreason.Optional(in.InternalNote, "InternalNote")
	if err != nil {
		return model.FoodRegistrationReview{}, err
	}
	var out model.FoodRegistrationReview
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.now()
		found, err := q.FoodRegistrationByID(ctx, in.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound()
		}
		if err != nil {
			return err
		}
		locked, err := lockStore(ctx, q, found.StoreID)
		if err != nil {
			return err
		}
		row, err := q.LockFoodRegistration(ctx, in.ID)
		if err != nil {
			return err
		}
		action := model.StoreActionRejectFoodRegistration
		if in.Outcome == model.FoodRegApproved {
			action = model.StoreActionApproveFoodRegistration
		}
		app, err := q.CurrentKYCApplication(ctx, row.StoreID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		facts := model.FoodRegistrationReview{State: model.FoodRegState(row.State), StoreStatus: model.Status(locked.Status)}
		if err == nil {
			status := model.KYCStatus(app.Status)
			facts.KYCStatus = &status
		}
		if reasons := model.FoodRegistrationUnavailableReasons(facts, action); len(reasons) > 0 {
			return model.ErrStoreActionUnavailable(reasons, action)
		}
		noteText := model.Text(note)
		if in.Outcome == model.FoodRegApproved {
			if _, err := q.EndApprovedFoodRegistration(ctx, store.EndApprovedFoodRegistrationParams{StoreID: row.StoreID, EndCause: string(model.FoodRegSuperseded), Now: now}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if _, err := q.ApproveFoodRegistration(ctx, store.ApproveFoodRegistrationParams{ID: row.ID, AdminID: in.AdminID, InternalNote: noteText, Now: now}); err != nil {
				return err
			}
			if err := s.publicChanged(ctx, locked, now); err != nil {
				return err
			}
		} else if _, err := q.RejectFoodRegistration(ctx, store.RejectFoodRegistrationParams{ID: row.ID, AdminID: in.AdminID, RejectReason: reason, InternalNote: noteText, Now: now}); err != nil {
			return err
		}
		if err := events.Publish(ctx, s.d.Events, model.FoodRegistrationDecided{
			EventID: uuid.NewV7(), OccurredAt: now, RegistrationID: row.ID, StoreID: row.StoreID, AccountID: row.AccountID,
			RegistrationNumber: model.Deref(row.RegistrationNumber), Outcome: in.Outcome, RejectReason: reason,
		}); err != nil {
			return err
		}
		out, err = s.review(ctx, q, row.ID)
		return err
	})
	return out, err
}

// Revoke 管理員撤銷核准中的登錄（資源層級的更正，不是對賣家的措施）：Reason 必填、給賣家看；撤銷不帶內部註記，核准時寫的註記留著（稽核紀錄只增不改）。撤銷後店家公開的字號
// 立即消失：發 StoreProfileChanged、排入快取失效，並發 FoodRegistrationRevoked（帶字號與理由）。店家若有同一字號的待審件不會
// 自動退回，由管理員另外裁定。登錄不存在回 NotFound，不是核准中的（待審、已退回、已結束）回 FailedPrecondition；店家的狀態不限
// （更正與店家是否營運無關）。
func (s *Service) Revoke(ctx context.Context, in model.RevokeFoodRegistrationInput) (model.FoodRegistrationReview, error) {
	if err := model.Positive(in.AdminID); err != nil {
		return model.FoodRegistrationReview{}, err
	}
	reason, err := auditreason.Required(in.Reason, "Reason")
	if err != nil {
		return model.FoodRegistrationReview{}, err
	}
	var out model.FoodRegistrationReview
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.now()
		found, err := q.FoodRegistrationByID(ctx, in.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound()
		}
		if err != nil {
			return err
		}
		locked, err := lockStore(ctx, q, found.StoreID)
		if err != nil {
			return err
		}
		row, err := q.LockFoodRegistration(ctx, in.ID)
		if err != nil {
			return err
		}
		if row.State != string(model.FoodRegApproved) {
			return apperr.FailedPrecondition("這筆食品業者登錄不是核准中，不能撤銷")
		}
		if _, err := q.RevokeApprovedFoodRegistration(ctx, store.RevokeApprovedFoodRegistrationParams{ID: row.ID, AdminID: in.AdminID, EndReason: reason, Now: now}); err != nil {
			return err
		}
		if err := s.publicChanged(ctx, locked, now); err != nil {
			return err
		}
		if err := events.Publish(ctx, s.d.Events, model.FoodRegistrationRevoked{
			EventID: uuid.NewV7(), OccurredAt: now, RegistrationID: row.ID, StoreID: row.StoreID, AccountID: row.AccountID,
			RegistrationNumber: model.Deref(row.RegistrationNumber), Reason: reason,
		}); err != nil {
			return err
		}
		out, err = s.review(ctx, q, row.ID)
		return err
	})
	return out, err
}

// normalizeNumber 去掉前後空白並檢查長度（1 到 50 字）。
func normalizeNumber(raw string) (string, error) {
	number := trimtext.Trim(raw)
	if n := utf8.RuneCountInString(number); n == 0 || n > 50 {
		return "", apperr.InvalidInputOn([]string{"RegistrationNumber"}, "食品業者登錄字號必須是 1 到 50 字")
	}
	return number, nil
}
