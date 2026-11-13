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
// seller 的同步重讀）：寫入只在這次的讀取開始時間不早於已存的 fetched_at 時進行（D32），被擋下的這次不寫、不標記、不發布，但仍以唯讀的 RoutePending
// 回答 again（較晚開始的那次標記了在它開始之前收到的事件，之後收到的還在，由這個 again 讓呼叫方再跑一輪）。沒有帳戶列、還沒有外部 ID 的什麼都不做。
// again 為 true 表示還有未處理的事件（唯一鍵排除了新 job）：呼叫方稍後再跑一輪。重讀失敗回傳錯誤，由 River 重試；外部帳戶不存在是
// 永久失敗（ErrPermanent），回傳給 job 取消。
func (s *Service) Sync(ctx context.Context, storeID int64) (again bool, err error) {
	if s.d.Provider == nil {
		return false, nil
	}
	row, err := store.New(s.d.DB.Conn(ctx)).PaymentAccountByStore(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return s.syncRow(ctx, row)
}

// syncRow 是 Sync 的本體，row 是呼叫方剛讀到的帳戶列（只用它的外部 ID 與 provider，寫入在 transaction 內重新鎖列）。
func (s *Service) syncRow(ctx context.Context, row store.PaymentAccount) (again bool, err error) {
	p := s.d.Provider
	if row.ExternalAccountID == nil || row.Provider != p.Name() || row.PurgePendingAt != nil {
		return false, nil
	}
	storeID := row.StoreID
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
			// 較晚開始的讀取已經寫入：這次的快照較舊，不寫、不發布、不標記；只唯讀地回答還有沒有未處理的事件。
			again, err = s.d.Inbox.RoutePending(ctx, before.Provider, external)
			return err
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

// ReadDebounce 是賣家觸發的重讀的最短間隔（D33）：同一帳戶上次賣家觸發的讀取被受理（seller_read_claimed_at，含失敗與並行的）或上次寫入的讀取
// （fetched_at，讀取開始時間）距現在不到這麼久，就不向 provider 讀（回答依 busyRetryAfter）。
const ReadDebounce = 10 * time.Second

// staleBefore 是搶名額的門檻：seller_read_claimed_at 與 fetched_at 不晚於它才搶得到（剛好滿 ReadDebounce 搶得到）。
func staleBefore(now time.Time) time.Time { return now.Add(-ReadDebounce) }

// busyRetryAfter 決定搶不到名額時的回答：回 0 是成功（快取已經涵蓋這個名額：fetched_at 不早於 seller_read_claimed_at，即占名額的那次讀取已寫入，
// 或名額是別的同步寫入的），回大於 0 是 resource_exhausted 的重試等待（占名額的讀取失敗或還在進行中，快取沒有涵蓋它）：窗的剩餘時間，至少一秒。
// 兩個時間都是列上的值，now 是注入的時鐘。
func busyRetryAfter(claimedAt, fetchedAt *time.Time, now time.Time) time.Duration {
	if claimedAt == nil || (fetchedAt != nil && !fetchedAt.Before(*claimedAt)) {
		return 0
	}
	return max(claimedAt.Add(ReadDebounce).Sub(now), time.Second)
}

// busyOutcome 把 busyRetryAfter 轉成回給呼叫方的結果：成功是 nil，否則是帶重試等待（傳輸層放進 RetryInfo）的 ResourceExhausted。
func busyOutcome(claimedAt, fetchedAt *time.Time, now time.Time) error {
	if wait := busyRetryAfter(claimedAt, fetchedAt, now); wait > 0 {
		return apperr.ResourceExhausted(wait, "收款帳戶正在重新整理，請稍後再試")
	}
	return nil
}

// SyncUnlessFresh 是賣家觸發的同步重讀（RefreshMyPaymentAccount）：先以單一條件式 UPDATE 搶讀取名額（ClaimPaymentAccountSellerRead）；搶到才讀
// （同一列沿用，不重讀）。搶不到時不呼叫 provider，依列上的時間回答（busyRetryAfter）：快取已涵蓋占名額的讀取就成功，否則（那次讀取失敗或還在進行中）
// 回 ResourceExhausted 並帶窗的剩餘秒數，client 照它重試，失敗不會被快取蓋掉。這個限制的成立條件是外部讀取由賣家觸發、頻率不受 job 的唯一鍵約束，
// 拿掉賣家觸發的呼叫時一起拿掉。沒有設定 provider 回 FailedPrecondition；沒有帳戶列或還沒有外部 ID 時什麼都不做（快取就是現況）。
// 讀完仍有未處理的事件（again，例如被 D32 擋下而事件是在較晚的讀取之後到的）就在自己的 transaction 內排同步 job（唯一鍵合併），
// 這個寫入不隨呼叫者取消。重讀失敗是明確動作的失敗：暫時性的回 Unavailable，外部帳戶不存在這類永久失敗回 FailedPrecondition；呼叫者自己取消的原樣回傳。
func (s *Service) SyncUnlessFresh(ctx context.Context, storeID int64) error {
	if s.d.Provider == nil {
		return apperr.FailedPrecondition("尚未設定收款開通")
	}
	now := s.d.Now().UTC()
	cutoff := staleBefore(now)
	q := store.New(s.d.DB.Conn(ctx))
	row, err := q.ClaimPaymentAccountSellerRead(ctx, store.ClaimPaymentAccountSellerReadParams{
		StoreID: storeID, Now: &now, StaleBefore: &cutoff,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		cur, err := q.PaymentAccountByStore(ctx, storeID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if cur.ExternalAccountID == nil {
			return nil
		}
		return busyOutcome(cur.SellerReadClaimedAt, cur.FetchedAt, now)
	}
	if err != nil {
		return err
	}
	if row.Provider != s.d.Provider.Name() {
		return apperr.FailedPrecondition("收款帳戶不是由目前的金流商建立")
	}
	again, err := s.syncRow(ctx, row)
	if err != nil {
		switch {
		case ctx.Err() != nil:
			return err
		case errors.Is(err, ErrPermanent):
			return apperr.WithCause(apperr.FailedPrecondition("金流商沒有這個收款帳戶"), err)
		default:
			return apperr.WithCause(apperr.Unavailable("暫時無法向金流商讀取收款帳戶，請稍後再試"), err)
		}
	}
	if again {
		return s.d.DB.InTx(context.WithoutCancel(ctx), func(ctx context.Context) error { return enqueueSync(ctx, s.d.Jobs, storeID) })
	}
	return nil
}

// Refresh 是管理員的「重新整理狀態」：只排同步 job（以 store_id 為唯一鍵，已經有排隊的就合併）。沒有設定 provider、店家還沒有外部帳戶時回
// FailedPrecondition，沒有這個店家的帳戶回 NotFound。
func (s *Service) Refresh(ctx context.Context, storeID int64) error {
	if s.d.Provider == nil {
		return apperr.FailedPrecondition("尚未設定收款開通")
	}
	row, err := store.New(s.d.DB.Conn(ctx)).PaymentAccountByStore(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("店家還沒有收款帳戶")
	}
	if err != nil {
		return err
	}
	if row.ExternalAccountID == nil {
		return apperr.FailedPrecondition("收款帳戶還沒有建立")
	}
	if row.PurgePendingAt != nil {
		return apperr.FailedPrecondition("收款帳戶正在關閉")
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
		if locked.PurgePendingAt != nil {
			return apperr.FailedPrecondition("收款帳戶正在關閉")
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
	if err := model.Positive(storeID); err != nil {
		return model.OnboardingSession{}, err
	}
	row, err := store.New(s.d.DB.Conn(ctx)).PaymentAccountByStore(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.OnboardingSession{}, apperr.NotFound("店家還沒有收款帳戶")
	}
	if err != nil {
		return model.OnboardingSession{}, err
	}
	if row.PurgePendingAt != nil {
		return model.OnboardingSession{}, apperr.FailedPrecondition("收款帳戶正在關閉")
	}
	acct := ToModel(row)
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
