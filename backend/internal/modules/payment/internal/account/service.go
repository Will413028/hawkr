package account

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const (
	// callTimeout 是單次呼叫 provider 的上限。
	callTimeout = 20 * time.Second
	// jobTimeout 是一次 job 的上限：一次呼叫加資料庫。
	jobTimeout = 60 * time.Second
	// writeTimeout 是寫結果的 transaction 的上限（job 的 ctx 逾時被取消後仍要能寫入）。
	writeTimeout = 15 * time.Second
	// maxAttempts 是 job 暫時性失敗時 River 重試的上限（退避是 River 預設的 attempt^4 秒，約 80 分鐘內用完）。
	maxAttempts = 8
	// sweepBatch 是掃描一次排入的上限。
	sweepBatch = 100
	// eventGrace 是 inbox 事件多久還沒處理之後由掃描補排同步（正常情況下同步 job 很快就涵蓋它）。
	eventGrace = time.Minute
	// createIdle 是建立列閒置多久之後由掃描補排建立 job（保底，平常 job 一定存在）。
	createIdle = 10 * time.Minute
	// EventRetention 是收款事件原始 payload 在 inbox 的保存期（D29）：涵蓋 Stripe 重送事件的 3 天與事後查核的時間。
	EventRetention = 30 * 24 * time.Hour
	// syncProcessor 與 eventProcessor 是 inbox 的處理者名稱。
	syncProcessor  = "payment.account_sync"
	eventProcessor = "payment.account_event"
)

// Owners 是擁有者（店家）是否營運中的查詢，由組裝點以 seller 的公開 API 實作（payment 是葉節點，不讀 seller）。建立 job 與補排建立只為營運中的擁有者
// 建立外部帳戶。
type Owners interface {
	OwnerActive(ctx context.Context, storeID int64) (bool, error)
}

// Deps 外部依賴；clock 由組裝點注入。Provider 為 nil 表示沒有設定收款開通：RequestAccount 什麼都不做，開通相關的呼叫回 FailedPrecondition，
// 這時不需要 Inbox 與 Owners；設定了 Provider 時兩者必填。
type Deps struct {
	DB       *db.DB
	Events   *events.Bus
	Jobs     jobs.Enqueuer
	Inbox    *inbox.Inbox
	Provider Provider
	Owners   Owners
	Now      func() time.Time
	Logger   *slog.Logger
}

// Service 是收款帳戶的 use case。
type Service struct{ d Deps }

// New 建立服務；設定了 provider 時把它的事件處理登記到 inbox。
func New(d Deps) *Service {
	if d.DB == nil || d.Events == nil || d.Jobs == nil || d.Now == nil || d.Logger == nil {
		panic("payment/account: missing dependency")
	}
	if d.Provider != nil && (d.Inbox == nil || d.Owners == nil) {
		panic("payment/account: Inbox and Owners are required with a provider")
	}
	s := &Service{d: d}
	if d.Provider != nil {
		d.Inbox.Register(inbox.Source{Provider: d.Provider.Name(), Retention: EventRetention, Handle: s.handleEvent})
	}
	return s
}

// Configured 回傳是否設定了 provider。
func (s *Service) Configured() bool { return s.d.Provider != nil }

// RequestAccount 在擁有者核准的 transaction 內（呼叫方的 ctx）寫「建立中」的帳戶列並排入以 store_id 為唯一鍵的建立 job（D16、D19）；列寫入時由
// 資料庫產生建立用的冪等鍵 UUID。已有列時不改它（INSERT ON CONFLICT DO NOTHING：呼叫方已鎖住上層的列，不再等下層 payment 的既有列，
// platform/lockorder），關店再開沿用同一個外部帳戶；既有列還沒有外部 ID、也沒有進入建立失敗時重新排入建立 job（唯一鍵合併）。
// contactEmail 是呼叫方傳入的聯絡信箱（D23），只在寫新列時快照；沒有時回 failed_precondition，核准整筆不寫入。這裡不呼叫 provider（規則 5）。
// 沒有設定 provider 時什麼都不做。
func (s *Service) RequestAccount(ctx context.Context, storeID int64, contactEmail *string, now time.Time) error {
	if s.d.Provider == nil {
		return nil
	}
	if err := model.Positive(storeID); err != nil {
		return err
	}
	if contactEmail == nil {
		return apperr.FailedPrecondition("帳號缺少聯絡信箱，無法建立收款帳戶")
	}
	q := store.New(s.d.DB.Conn(ctx))
	_, err := q.InsertPaymentAccount(ctx, store.InsertPaymentAccountParams{
		StoreID: storeID, Provider: s.d.Provider.Name(), ContactEmail: *contactEmail, RequirementCollection: string(s.d.Provider.Capabilities().RequirementCollection), Now: now,
	})
	if err != nil {
		return err
	}
	row, err := q.PaymentAccountByStore(ctx, storeID)
	if err != nil {
		return err
	}
	if row.ExternalAccountID != nil || row.CreateFailedAt != nil {
		return nil
	}
	return enqueueEnsure(ctx, s.d.Jobs, storeID)
}

// ToModel 轉成對外的型別。
func ToModel(r store.PaymentAccount) model.PaymentAccount {
	return model.PaymentAccount{
		StoreID: r.StoreID, Provider: r.Provider, ExternalAccountID: r.ExternalAccountID,
		RequirementCollection: model.RequirementCollection(r.RequirementCollection), Status: model.PaymentAccountStatus(r.Status),
		TransfersActive: r.TransfersActive, PayoutsActive: r.PayoutsActive, Requirements: r.Requirements, StatusReason: r.StatusReason,
		FetchedAt: r.FetchedAt, CreateFailedAt: r.CreateFailedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// Account 讀擁有者的收款帳戶；還沒有（沒有核准過）回 NotFound。
func (s *Service) Account(ctx context.Context, storeID int64) (model.PaymentAccount, error) {
	if err := model.Positive(storeID); err != nil {
		return model.PaymentAccount{}, err
	}
	row, err := store.New(s.d.DB.Conn(ctx)).PaymentAccountByStore(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PaymentAccount{}, apperr.NotFound("店家還沒有收款帳戶")
	}
	if err != nil {
		return model.PaymentAccount{}, err
	}
	return ToModel(row), nil
}

// Ensure 是建立 job 的內容：先讀本地列，確認擁有者營運中（Owners），在 transaction 外呼叫 provider 建立外部帳戶（冪等鍵是列上保存的 UUID），
// 再於 transaction 內填上外部 ID 並排入同步 job。已經有外部 ID、已進入失敗、擁有者已關閉時什麼都不做。暫時性的失敗回傳錯誤由 River 退避重試；
// final 為 true（最後一次嘗試）或失敗是永久的時，記下建立失敗的時刻（進後台待辦 PAYMENT_ACCOUNT_CREATE_FAILED）並結束。
func (s *Service) Ensure(ctx context.Context, storeID int64, final bool) error {
	p := s.d.Provider
	if p == nil {
		return nil
	}
	row, err := store.New(s.d.DB.Conn(ctx)).PaymentAccountByStore(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.ExternalAccountID != nil || row.CreateFailedAt != nil || row.Provider != p.Name() {
		return nil
	}
	active, err := s.d.Owners.OwnerActive(ctx, storeID)
	if err != nil {
		return err
	}
	if !active {
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	externalID, callErr := p.EnsureAccount(callCtx, EnsureInput{StoreID: storeID, IdempotencyKey: row.CreateIdempotencyKey.String(), ContactEmail: row.ContactEmail, Collection: model.RequirementCollection(row.RequirementCollection)})
	permanent := errors.Is(callErr, ErrPermanent)
	if callErr != nil && !permanent && !final {
		return callErr
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		now := s.d.Now().UTC()
		if _, err := q.LockPaymentAccount(ctx, storeID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if callErr != nil {
			s.d.Logger.WarnContext(ctx, "payment：建立收款帳戶失敗", "store_id", storeID, "permanent", permanent, "error", callErr)
			_, markErr := q.MarkPaymentAccountCreateFailed(ctx, store.MarkPaymentAccountCreateFailedParams{StoreID: storeID, Now: &now})
			return markErr
		}
		if _, setErr := q.SetPaymentAccountExternal(ctx, store.SetPaymentAccountExternalParams{StoreID: storeID, ExternalAccountID: &externalID, Now: now}); setErr != nil {
			if errors.Is(setErr, pgx.ErrNoRows) {
				return nil
			}
			return setErr
		}
		return enqueueSync(ctx, s.d.Jobs, storeID)
	})
}

// Capabilities 回傳目前 provider 的能力描述；沒有設定 provider 時 ok 為 false。
func (s *Service) Capabilities() (caps Capabilities, ok bool) {
	if s.d.Provider == nil {
		return Capabilities{}, false
	}
	return s.d.Provider.Capabilities(), true
}

// ForgetOwners 在呼叫方（擁有者個資清理）的 transaction 內排入刪除這些擁有者帳戶列的 job：呼叫方已鎖住上層的列，不在同一個 transaction
// 等下層 payment 的列（platform/lockorder），刪除由本模組的 job 在自己的 transaction 做。沒有 ID 時什麼都不做。
func (s *Service) ForgetOwners(ctx context.Context, storeIDs []int64) error {
	if len(storeIDs) == 0 {
		return nil
	}
	if err := model.Positive(storeIDs...); err != nil {
		return err
	}
	args := PurgeArgs{StoreIDs: storeIDs}
	opts := args.InsertOpts()
	return s.d.Jobs.Enqueue(ctx, args, &opts)
}

// Purge 是刪除 job 的內容：刪掉這些擁有者的帳戶列（含聯絡信箱）。收款事件的 inbox 列由 platform 的清理依保存期刪除。
func (s *Service) Purge(ctx context.Context, storeIDs []int64) error {
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		_, err := store.New(s.d.DB.Conn(ctx)).DeletePaymentAccounts(ctx, storeIDs)
		return err
	})
}
