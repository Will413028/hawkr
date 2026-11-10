package account

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// Sync 是同步 job 的內容：在 transaction 外重讀外部帳戶，在 transaction 內整列覆寫快取、發布 PayoutAccountChanged（seller 的同 transaction 訂閱者
// 更新開通閘門的副本、把 fingerprint 寫成防詐 hash、閘門的輸入有變時重新公告；全域鎖順序 payment 先於 seller），並經 platform inbox 把這個外部帳戶在
// 讀取開始前收到的事件整批標為已處理（D29：收款事件只是「重讀帳戶」的訊號，handler 不標記，由涵蓋它的同步標記）。同步可能和別的同步並行（job 與
// seller 的同步重讀）：寫入只在這次的讀取開始時間不早於已存的 fetched_at 時進行（D32），被擋下的這次整個什麼都不做（不寫、不標記、不發布，again 為
// false：較晚開始的那次涵蓋了在這次開始之前收到的事件）。沒有帳戶列、還沒有外部 ID 的什麼都不做。again 為 true 表示讀取期間又收到事件（唯一鍵排除了新 job）：呼叫方稍後再跑一輪。重讀失敗回傳錯誤，由 River 重試；外部帳戶不存在是
// 永久失敗（ErrPermanent），回傳給 job 取消。
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if before.ExternalAccountID == nil || *before.ExternalAccountID != external {
			return nil
		}
		now := s.d.Now().UTC()
		status, reasonCode, note := StatusOf(snap)
		if note != NoteNone {
			s.d.Logger.WarnContext(ctx, "payment：收款帳戶的快照不認得或不完整，視為審核中", "store_id", storeID, "external_account_id", external, "note", string(note), "transfers", string(snap.Transfers), "details", len(snap.Details))
		}
		raw := snap.Requirements
		if len(raw) == 0 || !json.Valid(raw) {
			raw = json.RawMessage(`{}`)
		}
		var reason *string
		if reasonCode != "" {
			reason = &reasonCode
		}
		enabled := status == model.PaymentEnabled
		if _, err := q.ApplyPaymentSnapshot(ctx, store.ApplyPaymentSnapshotParams{
			StoreID: storeID, Status: string(status), TransfersActive: enabled, PayoutsActive: snap.PayoutsActive && enabled,
			Requirements: raw, StatusReason: reason, FetchedAt: &startedAt, Now: now,
		}); errors.Is(err, pgx.ErrNoRows) {
			// 較晚開始的讀取已經寫入：這次的快照較舊，不寫、不發布、不標記；涵蓋的事件與 again 都由那一次處理。
			return nil
		} else if err != nil {
			return err
		}
		err = events.Publish(ctx, s.d.Events, model.PayoutAccountChanged{
			OccurredAt: now, StoreID: storeID, Provider: before.Provider, TransfersActive: enabled, BankFingerprints: snap.BankFingerprints,
		})
		if err != nil {
			return err
		}
		if _, err := s.d.Inbox.MarkRouteProcessed(ctx, before.Provider, external, startedAt, syncProcessor); err != nil {
			return err
		}
		again, err = s.d.Inbox.RoutePending(ctx, before.Provider, external)
		return err
	})
	return again, err
}

// ReadDebounce 是賣家觸發的重讀的間隔：同一帳戶上次讀取（fetched_at，讀取開始時間）距現在不到這麼久就不向 provider 讀，直接用快取（D33）。
const ReadDebounce = 10 * time.Second

// readIsFresh 回答快取是否新到不必再讀：沒有讀過（fetchedAt 為 nil）一定要讀；剛好滿 ReadDebounce 就要讀。
func readIsFresh(fetchedAt *time.Time, now time.Time) bool {
	return fetchedAt != nil && now.Sub(*fetchedAt) < ReadDebounce
}

// SyncUnlessFresh 是賣家觸發的同步重讀（RefreshMyPaymentAccount）：上次讀取在 ReadDebounce 內就不呼叫 provider、直接成功（快取就是結果）；
// 否則走 Sync。這個限制的成立條件是外部讀取由賣家觸發、頻率不受 job 的唯一鍵約束，拿掉賣家觸發的呼叫時一起拿掉。沒有 provider、沒有帳戶列或還沒有
// 外部 ID 時什麼都不做（與 Sync 相同，快取就是現況）。重讀失敗是明確動作的失敗：暫時性的回 Unavailable（重試可能成功），外部帳戶不存在這類永久
// 失敗回 FailedPrecondition；呼叫者自己取消的原樣回傳。
func (s *Service) SyncUnlessFresh(ctx context.Context, storeID int64) error {
	if s.d.Provider == nil {
		return nil
	}
	row, err := store.New(s.d.DB.Conn(ctx)).PaymentAccountByStore(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if readIsFresh(row.FetchedAt, s.d.Now()) {
		return nil
	}
	if _, err := s.Sync(ctx, storeID); err != nil {
		switch {
		case ctx.Err() != nil:
			return err
		case errors.Is(err, ErrPermanent):
			return apperr.WithCause(apperr.FailedPrecondition("金流商沒有這個收款帳戶"), err)
		default:
			return apperr.WithCause(apperr.Unavailable("暫時無法向金流商讀取收款帳戶，請稍後再試"), err)
		}
	}
	return nil
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
	var out store.PaymentAccount
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
	session, err := s.d.Provider.CreateOnboardingSession(callCtx, SessionInput{ExternalAccountID: *acct.ExternalAccountID, Collection: acct.RequirementCollection, ReturnURL: returnURL, RefreshURL: refreshURL})
	if errors.Is(err, ErrPermanent) {
		return model.OnboardingSession{}, apperr.FailedPrecondition("目前無法開通收款帳戶")
	}
	return session, err
}
