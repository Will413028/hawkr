package payment

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/stint"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Sync 是同步 job 的內容：在 transaction 外重讀外部帳戶，在 transaction 內整列覆寫快取、記下涵蓋的事件、閘門的輸入有變時重新公告可曝光
// （transaction 裡最後一個鎖：帳戶列之後是公告列）。沒有帳戶列、還沒有外部 ID 的什麼都不做。again 為 true 表示讀取期間又收到事件
// （唯一鍵排除了新 job）：呼叫方稍後再跑一輪。重讀失敗回傳錯誤，由 River 重試；外部帳戶不存在是永久失敗（ErrPermanent），回傳給 job 取消。
func (s *Service) Sync(ctx context.Context, storeID int64) (again bool, err error) {
	p := s.d.Provider
	if p == nil {
		return false, nil
	}
	row, err := store.New(s.d.DB.Conn(ctx)).PaymentAccountByStore(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if row.ExternalAccountID == nil || row.Provider != p.Name() {
		return false, nil
	}
	external := *row.ExternalAccountID
	startedAt := s.d.Now().UTC()
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	snap, err := p.FetchAccount(callCtx, external)
	if err != nil {
		return false, err
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	err = s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		before, err := q.LockPaymentAccount(ctx, storeID)
		if err != nil {
			return err
		}
		if before.ExternalAccountID == nil || *before.ExternalAccountID != external {
			return nil
		}
		now := s.d.Now().UTC()
		status, reasonCode := StatusOf(s.d.Logger, snap)
		raw := snap.Requirements
		if len(raw) == 0 || !json.Valid(raw) {
			raw = json.RawMessage(`{}`)
		}
		var reason *string
		if reasonCode != "" {
			reason = &reasonCode
		}
		if _, err := q.ApplyPaymentSnapshot(ctx, store.ApplyPaymentSnapshotParams{
			StoreID: storeID, Status: string(status), TransfersActive: status == model.PaymentEnabled, PayoutsActive: snap.PayoutsActive && status == model.PaymentEnabled,
			Requirements: raw, StatusReason: reason, FetchedAt: &startedAt, Now: now,
		}); err != nil {
			return err
		}
		if _, err := q.MarkPaymentEventsProcessed(ctx, store.MarkPaymentEventsProcessedParams{
			Provider: before.Provider, ExternalAccountID: external, ReceivedBefore: startedAt, Now: &now,
		}); err != nil {
			return err
		}
		if again, err = q.PaymentEventsPending(ctx, store.PaymentEventsPendingParams{Provider: before.Provider, ExternalAccountID: external}); err != nil {
			return err
		}
		if before.TransfersActive == (status == model.PaymentEnabled) {
			return nil
		}
		if err := s.d.Exposure.Announce(ctx, q, now, storeID); err != nil {
			return err
		}
		return stint.InvalidateByID(ctx, q, s.d.Jobs, storeID)
	})
	return again, err
}

// Refresh 是管理員的「重新整理狀態」：只排同步 job（以 store_id 為唯一鍵，已經有排隊的就合併）。沒有設定 provider、店家還沒有外部帳戶時回
// FailedPrecondition，沒有這個店家的帳戶回 NotFound。
func (s *Service) Refresh(ctx context.Context, storeID int64) error {
	if s.d.Provider == nil {
		return apperr.FailedPrecondition("尚未設定收款開通")
	}
	acct, err := s.Account(ctx, storeID)
	if err != nil {
		return err
	}
	if acct.ExternalAccountID == nil {
		return apperr.FailedPrecondition("收款帳戶還沒有建立")
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error { return enqueueSync(ctx, s.d.Jobs, storeID) })
}

// RetryCreate 是管理員的「重試建立」：只有建立失敗的列可以重試（清空失敗時刻並在同一個 transaction 重新排入建立 job）。
func (s *Service) RetryCreate(ctx context.Context, storeID int64) (model.PaymentAccount, error) {
	if s.d.Provider == nil {
		return model.PaymentAccount{}, apperr.FailedPrecondition("尚未設定收款開通")
	}
	if err := model.Positive(storeID); err != nil {
		return model.PaymentAccount{}, err
	}
	var out store.SellerPaymentAccount
	err := s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, err := q.LockPaymentAccount(ctx, storeID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("店家還沒有收款帳戶")
		}
		if err != nil {
			return err
		}
		if locked.ExternalAccountID != nil || locked.CreateFailedAt == nil {
			return apperr.FailedPrecondition("收款帳戶不是建立失敗的狀態")
		}
		if _, err := q.ClearPaymentAccountCreateFailed(ctx, store.ClearPaymentAccountCreateFailedParams{StoreID: storeID, Now: s.d.Now().UTC()}); err != nil {
			return err
		}
		if err := enqueueEnsure(ctx, s.d.Jobs, storeID); err != nil {
			return err
		}
		out, err = q.PaymentAccountByStore(ctx, storeID)
		return err
	})
	if err != nil {
		return model.PaymentAccount{}, err
	}
	return ToModel(out), nil
}

// CreateOnboardingSession 為店家的帳戶產生開通 session（託管連結或內嵌元件）。外部呼叫在 transaction 外。帳戶還沒有外部 ID 回 FailedPrecondition。
func (s *Service) CreateOnboardingSession(ctx context.Context, storeID int64, returnURL, refreshURL string) (model.OnboardingSession, error) {
	if s.d.Provider == nil {
		return model.OnboardingSession{}, apperr.FailedPrecondition("尚未設定收款開通")
	}
	acct, err := s.Account(ctx, storeID)
	if err != nil {
		return model.OnboardingSession{}, err
	}
	if acct.ExternalAccountID == nil || acct.Provider != s.d.Provider.Name() {
		return model.OnboardingSession{}, apperr.FailedPrecondition("收款帳戶還沒有建立")
	}
	if acct.Status == model.PaymentDisabled {
		return model.OnboardingSession{}, apperr.FailedPrecondition("收款帳戶已被金流商停用")
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	session, err := s.d.Provider.CreateOnboardingSession(callCtx, SessionInput{ExternalAccountID: *acct.ExternalAccountID, ReturnURL: returnURL, RefreshURL: refreshURL})
	if errors.Is(err, ErrPermanent) {
		return model.OnboardingSession{}, apperr.FailedPrecondition("目前無法開通收款帳戶")
	}
	return session, err
}
