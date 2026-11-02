package payment

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/exposure"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
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
)

// Deps 外部依賴；clock 由組裝點注入。Provider 為 nil 表示沒有設定收款開通：核准不寫帳戶列、不排 job，開通相關的呼叫回 FailedPrecondition。
type Deps struct {
	DB       *db.DB
	Jobs     jobs.Enqueuer
	Provider Provider
	Exposure *exposure.Service
	Now      func() time.Time
	Logger   *slog.Logger
}

// Service 是收款帳戶的 use case。
type Service struct{ d Deps }

// New 建立服務。
func New(d Deps) *Service {
	if d.DB == nil || d.Jobs == nil || d.Exposure == nil || d.Now == nil || d.Logger == nil {
		panic("seller/payment: missing dependency")
	}
	return &Service{d: d}
}

// Configured 回傳是否設定了 provider。
func (s *Service) Configured() bool { return s.d.Provider != nil }

// RequestAccount 在平台審核核准的 transaction 內寫「建立中」的帳戶列（已有列時沿用，關店再開不建第二個帳戶）並排入以 store_id 為唯一鍵的建立 job
// （D16、D19）；列寫入時由資料庫產生建立用的冪等鍵 UUID。contactEmail 是申請上的收款聯絡信箱（D23），複製到列上供建立外部帳戶使用；沒有時
// 回 failed_precondition（送出前的申請沒有這個欄位），核准整筆不寫入。呼叫方已鎖住店家與申請列。這裡不呼叫 provider（規則 5）。沒有設定 provider 時什麼都不做。
func (s *Service) RequestAccount(ctx context.Context, q *store.Queries, storeID int64, contactEmail *string, now time.Time) error {
	if s.d.Provider == nil {
		return nil
	}
	if contactEmail == nil {
		return apperr.FailedPrecondition("申請缺少收款聯絡信箱，請退回申請要求賣家補填")
	}
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
func ToModel(r store.SellerPaymentAccount) model.PaymentAccount {
	return model.PaymentAccount{
		StoreID: r.StoreID, Provider: r.Provider, ExternalAccountID: r.ExternalAccountID,
		RequirementCollection: model.RequirementCollection(r.RequirementCollection), Status: model.PaymentAccountStatus(r.Status),
		TransfersActive: r.TransfersActive, PayoutsActive: r.PayoutsActive, Requirements: r.Requirements, StatusReason: r.StatusReason,
		FetchedAt: r.FetchedAt, CreateFailedAt: r.CreateFailedAt, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// Account 讀店家的收款帳戶；店家還沒有（沒有核准過）回 NotFound。
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

// Ensure 是建立 job 的內容：先讀本地列（transaction 內短讀），在 transaction 外呼叫 provider 建立外部帳戶（冪等鍵是列上保存的 UUID），再於
// transaction 內填上外部 ID 並排入同步 job。已經有外部 ID、已進入失敗、店家已關閉時什麼都不做。暫時性的失敗回傳錯誤由 River 退避重試；
// final 為 true（最後一次嘗試）或失敗是永久的時，記下建立失敗的時刻（進後台待辦 PAYMENT_ACCOUNT_CREATE_FAILED）並結束。
func (s *Service) Ensure(ctx context.Context, storeID int64, final bool) error {
	p := s.d.Provider
	if p == nil {
		return nil
	}
	var row store.SellerPaymentAccount
	var active bool
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		var err error
		if row, err = q.PaymentAccountByStore(ctx, storeID); err != nil {
			return err
		}
		owner, err := q.StoreByID(ctx, storeID)
		if err != nil {
			return model.Error(err)
		}
		active = owner.Status != string(model.Closed)
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.ExternalAccountID != nil || row.CreateFailedAt != nil || !active || row.Provider != p.Name() {
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
			return err
		}
		if callErr != nil {
			s.d.Logger.WarnContext(ctx, "seller：建立收款帳戶失敗", "store_id", storeID, "permanent", permanent, "error", callErr)
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
