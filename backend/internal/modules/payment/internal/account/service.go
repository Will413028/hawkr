package account

import (
	"context"
	"errors"
	"fmt"
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
	// syncProcessor、eventProcessor 與 purgeProcessor 是 inbox 的處理者名稱。
	syncProcessor  = "payment.account_sync"
	eventProcessor = "payment.account_event"
	purgeProcessor = "payment.account_purge"
)

// Owners 是擁有者（店家）的查詢，由組裝點以 seller 與 identity 的公開 API 實作（payment 是葉節點，不讀上層）。建立 job 與補排建立只為營運中的擁有者
// 建立外部帳戶；聯絡信箱在建立外部帳戶的當下讀（D50），payment 不保存它的副本。
type Owners interface {
	OwnerActive(ctx context.Context, storeID int64) (bool, error)
	// OwnerContactEmail 回傳擁有者目前的聯絡信箱（已正規化）；沒有可用的信箱時 ok 是 false。
	OwnerContactEmail(ctx context.Context, storeID int64) (email string, ok bool, err error)
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
// 聯絡信箱不在這裡讀也不存：建立 job 在呼叫 provider 的當下才向擁有者讀（D50）。這裡不呼叫 provider（規則 5）。
// 既有列已標記等待清理（purge_pending_at）時回 FailedPrecondition，呼叫方的核准隨之失敗。沒有設定 provider 時什麼都不做。
func (s *Service) RequestAccount(ctx context.Context, storeID int64, now time.Time) error {
	if s.d.Provider == nil {
		return nil
	}
	if err := model.Positive(storeID); err != nil {
		return err
	}
	q := store.New(s.d.DB.Conn(ctx))
	_, err := q.InsertPaymentAccount(ctx, store.InsertPaymentAccountParams{
		StoreID: storeID, Provider: s.d.Provider.Name(), RequirementCollection: string(s.d.Provider.Capabilities().RequirementCollection), Now: now,
	})
	if err != nil {
		return err
	}
	row, err := q.PaymentAccountByStore(ctx, storeID)
	if err != nil {
		return err
	}
	enqueue, err := requestOutcome(row)
	if err != nil || !enqueue {
		return err
	}
	return enqueueEnsure(ctx, s.d.Jobs, storeID)
}

// requestOutcome 是 RequestAccount 讀到既有列後的決定：列已標記等待清理時回 FailedPrecondition（核准要等清理完成，不鎖也不改該列），
// 否則回傳是否要排入建立 job。
func requestOutcome(row store.PaymentAccount) (enqueue bool, err error) {
	if row.PurgePendingAt != nil {
		return false, apperr.FailedPrecondition("店家的收款帳戶正在清理中，等清理完成後才能核准")
	}
	return createWanted(row), nil
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

// Ensure 是建立 job 的內容：先讀本地列，確認擁有者營運中（Owners），在 transaction 外讀擁有者目前的聯絡信箱（D50）並呼叫 provider 建立外部帳戶
// （冪等鍵是列上保存的 UUID；adapter 分兩步建立，信箱在第二步寫入，D55），再於 transaction 內填上外部 ID 並排入同步 job。已經有外部 ID、已進入失敗、擁有者已關閉時什麼都不做。擁有者沒有聯絡信箱
// 時不呼叫 provider，視為永久失敗（重試不會憑空生出信箱；擁有者補填之後由管理員重試建立）。暫時性的失敗回傳錯誤由 River 退避重試；
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
	if !createWanted(row) || row.Provider != p.Name() {
		return nil
	}
	active, err := s.d.Owners.OwnerActive(ctx, storeID)
	if err != nil {
		return err
	}
	if !active {
		return nil
	}
	email, hasEmail, err := s.d.Owners.OwnerContactEmail(ctx, storeID)
	if err != nil {
		return err
	}
	var externalID string
	var callErr error
	if hasEmail {
		callCtx, cancel := context.WithTimeout(ctx, callTimeout)
		defer cancel()
		externalID, callErr = p.EnsureAccount(callCtx, EnsureInput{StoreID: storeID, IdempotencyKey: row.CreateIdempotencyKey.String(), ContactEmail: email, Collection: model.RequirementCollection(row.RequirementCollection)})
	} else {
		callErr = fmt.Errorf("%w：擁有者沒有可用的聯絡信箱", ErrPermanent)
	}
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

// ForgetOwners 在呼叫方（擁有者個資清理）的 transaction 內，為每個擁有者排入「關閉外部帳戶後刪除帳戶列」的 job（唯一鍵 store_id）：呼叫方已鎖住上層的列，
// 不在同一個 transaction 等下層 payment 的列（platform/lockorder），關閉與刪除由本模組的 job 做。沒有 ID 時什麼都不做。
func (s *Service) ForgetOwners(ctx context.Context, storeIDs []int64) error {
	if len(storeIDs) == 0 {
		return nil
	}
	if err := model.Positive(storeIDs...); err != nil {
		return err
	}
	for _, id := range storeIDs {
		if err := enqueuePurge(ctx, s.d.Jobs, id); err != nil {
			return err
		}
	}
	return nil
}

// purgePlan 是清理一列的處理方式。
type purgePlan int

const (
	// purgeDelete 是直接刪列：還沒有外部 ID，而且問不到 provider（沒有設定，或列的 provider 與目前設定不同）。
	purgeDelete purgePlan = iota
	// purgeResolve 是沒有外部 ID 的列、provider 與目前設定相同：以列上的冪等鍵取得外部帳戶，關閉它，成功才刪列（D55）。
	purgeResolve
	// purgeClose 是先關閉外部帳戶，成功才刪列。
	purgeClose
	// purgeDefer 是不動外部帳戶也不刪列：列的 provider 與目前設定的不同，或沒有設定 provider（刪掉就永遠找不到外部帳戶）。
	purgeDefer
)

// planPurge 決定清理一列的處理方式；provider 是目前設定的 provider 代碼，沒有設定時是空字串。沒有外部 ID 的列：provider 相同時取得外部帳戶再關閉
// （建立可能已經成功、只是 ID 沒寫進列），問不到 provider 時直接刪。
func planPurge(row store.PaymentAccount, provider string) purgePlan {
	switch {
	case row.ExternalAccountID == nil && provider != "" && row.Provider == provider:
		return purgeResolve
	case row.ExternalAccountID == nil:
		return purgeDelete
	case provider == "" || row.Provider != provider:
		return purgeDefer
	default:
		return purgeClose
	}
}

// Purge 是「關閉後刪除」job 的內容（D52）：第一步，每一列（有或沒有外部 ID）先在自己的 transaction 標成等待清理（purge_pending_at，之後任何錯誤、逾時或程序被殺都有週期掃描接手）；
// 沒有外部 ID 的列在 provider 相同時走 purgeResolved（以冪等鍵取得外部帳戶、關閉、刪；D55），問不到 provider 時記警示 log 後直接刪。有外部帳戶時在 transaction 外關閉它（冪等：已關閉視為成功），成功之後才在同一個 transaction 刪掉列，並把它外部帳戶在 inbox 裡到現在為止的
// 未處理事件標為已處理（帳戶不在了，不會再有同步涵蓋它們；不標記的話 platform 的清理永遠不刪，原始 payload 一直留著；inbox 列之後由 platform 的清理依保存期刪除）。
// 列的 provider 與目前設定不同、或沒有設定 provider 時不關也不刪，記警示 log，設定回來之後由週期掃描重排。暫時性的失敗回傳錯誤由 River 退避重試；final 為 true
// （最後一次嘗試）或失敗是永久的（例如金流商沒有這個帳戶，重試不會成功）時，本地列留著（已標記）並記警示 log（store_id 與外部帳戶 ID）；週期掃描之後重排。
func (s *Service) Purge(ctx context.Context, storeID int64, final bool) error {
	row, err := store.New(s.d.DB.Conn(ctx)).PaymentAccountByStore(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	provider := ""
	if s.d.Provider != nil {
		provider = s.d.Provider.Name()
	}
	if row.PurgePendingAt == nil {
		if err := s.markPurgePending(ctx, storeID); err != nil {
			return err
		}
	}
	switch planPurge(row, provider) {
	case purgeDelete:
		s.d.Logger.WarnContext(ctx, "payment：沒有外部 ID 的收款帳戶列問不到 provider，直接刪除（若金流商那邊已有帳戶不會被關閉）", "store_id", storeID, "row_provider", row.Provider, "configured_provider", provider)
		return s.deleteRow(ctx, storeID, nil)
	case purgeResolve:
		return s.purgeResolved(ctx, row, final)
	case purgeDefer:
		s.d.Logger.WarnContext(ctx, "payment：收款帳戶的 provider 與目前設定不同，沒有關閉外部帳戶也沒有刪除，等設定回來再清理", "store_id", storeID, "external_account_id", *row.ExternalAccountID, "row_provider", row.Provider, "configured_provider", provider)
		return nil
	case purgeClose:
	}
	return s.closeThenDelete(ctx, storeID, *row.ExternalAccountID, final)
}

// purgeResolved 清理沒有外部 ID 的列（D55）：以列上的冪等鍵取得外部帳戶（建立已成功就是那一個，否則建一個沒有擁有者個資的空帳戶），先把它的 ID 寫回已標記的列，
// 再走和有外部 ID 的列相同的關閉後刪除。順序是關鍵：關閉之後同一把鍵的重放會被金流商拒絕（measured：409 idempotency_error，永久），所以 ID 一定要在關閉之前存下，
// 關閉成功但刪列失敗的重試就不必再取得已關閉的帳戶。暫時性的失敗由 River 重試；最後一次或永久失敗時列留著並記警示 log，週期掃描之後重排。
func (s *Service) purgeResolved(ctx context.Context, row store.PaymentAccount, final bool) error {
	return resolveStoreThenClose(ctx, s.d.Provider, ResolveInput{StoreID: row.StoreID, IdempotencyKey: row.CreateIdempotencyKey.String()},
		func(ctx context.Context, external string) (bool, error) {
			return s.storeResolvedExternal(ctx, row.StoreID, external)
		},
		func(ctx context.Context, external string) error {
			return s.closeThenDelete(ctx, row.StoreID, external, final)
		},
		func(err error) error { return s.purgeFailed(ctx, row.StoreID, nil, err, final) })
}

// resolveStoreThenClose 是 purgeResolved 的順序：取得外部帳戶（自己的逾時）、save 寫回列（回報列已不在時結束）、才 closeDelete；取得失敗交給 onResolveErr。
func resolveStoreThenClose(ctx context.Context, p Provider, in ResolveInput,
	save func(ctx context.Context, external string) (gone bool, err error),
	closeDelete func(ctx context.Context, external string) error,
	onResolveErr func(error) error,
) error {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	external, err := p.ResolveAccount(callCtx, in)
	cancel()
	if err != nil {
		return onResolveErr(err)
	}
	if gone, err := save(ctx, external); gone || err != nil {
		return err
	}
	return closeDelete(ctx, external)
}

// storeResolvedExternal 在 transaction 內把取得的外部 ID 寫回已標記等待清理的列（欄位與 Ensure 寫入時相同）；列已經不在時 gone 為真。列上已經有外部 ID
// （標記前並行的建立寫入了同一個帳戶）時不改它。
func (s *Service) storeResolvedExternal(ctx context.Context, storeID int64, external string) (gone bool, err error) {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	err = s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.LockPaymentAccount(ctx, storeID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				gone = true
				return nil
			}
			return err
		}
		_, err := q.SetPaymentAccountExternalForPurge(ctx, store.SetPaymentAccountExternalForPurgeParams{StoreID: storeID, ExternalAccountID: &external, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	})
	return gone, err
}

// closeThenDelete 在 transaction 外關閉外部帳戶（冪等：已關閉視為成功），成功才刪列；每次 provider 呼叫有自己的逾時。
func (s *Service) closeThenDelete(ctx context.Context, storeID int64, external string, final bool) error {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	if err := s.d.Provider.CloseAccount(callCtx, external); err != nil {
		return s.purgeFailed(ctx, storeID, &external, err, final)
	}
	return s.deleteRow(ctx, storeID, &external)
}

// purgeFailed 處理清理時 provider 呼叫的失敗：暫時性且不是最後一次嘗試時回傳錯誤由 River 重試，否則本地列（已標記）留著並記警示 log，等週期掃描重排。
func (s *Service) purgeFailed(ctx context.Context, storeID int64, external *string, err error, final bool) error {
	permanent := errors.Is(err, ErrPermanent)
	if !permanent && !final {
		return err
	}
	ext := ""
	if external != nil {
		ext = *external
	}
	s.d.Logger.WarnContext(ctx, "payment：取得或關閉外部帳戶失敗，本地列留著等週期掃描重排", "store_id", storeID, "external_account_id", ext, "permanent", permanent, "error", err)
	return nil
}

// createWanted 是列是否還需要建立外部帳戶：還沒有外部 ID、沒有進入建立失敗、沒有等待清理（標記了就不建立）。
func createWanted(row store.PaymentAccount) bool {
	return row.ExternalAccountID == nil && row.CreateFailedAt == nil && row.PurgePendingAt == nil
}

// errExternalAccountAppeared 是計畫直接刪列、鎖住之後卻發現列已有外部 ID（並行的建立剛寫入）：不能刪，回傳錯誤讓 job 重試並走關閉的路。
var errExternalAccountAppeared = errors.New("payment：要直接刪的帳戶列已經有外部帳戶，改走關閉")

// lockedRowMayBeDeleted 在鎖住列之後再檢查它是否還能照計畫刪：planned 是 nil（計畫是直接刪）時列上必須仍沒有外部 ID，否則回 errExternalAccountAppeared；
// planned 不是 nil（已關閉的那個帳戶）時，列上的外部 ID 必須還是它，否則不刪也不是錯誤。
func lockedRowMayBeDeleted(locked store.PaymentAccount, planned *string) (bool, error) {
	if planned == nil {
		if locked.ExternalAccountID != nil {
			return false, errExternalAccountAppeared
		}
		return true, nil
	}
	return locked.ExternalAccountID != nil && *locked.ExternalAccountID == *planned, nil
}

// deleteRow 刪掉擁有者的列；鎖住之後依 lockedRowMayBeDeleted 再檢查（external 是 nil 表示計畫是直接刪）。
func (s *Service) deleteRow(ctx context.Context, storeID int64, external *string) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, err := q.LockPaymentAccount(ctx, storeID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if ok, err := lockedRowMayBeDeleted(locked, external); !ok {
			return err
		}
		rows, err := q.DeletePaymentAccounts(ctx, []int64{storeID})
		if err != nil || s.d.Inbox == nil {
			return err
		}
		now := s.d.Now().UTC()
		for _, r := range rows {
			if r.ExternalAccountID == nil {
				continue
			}
			if _, err := s.d.Inbox.MarkRouteProcessed(ctx, r.Provider, *r.ExternalAccountID, now, purgeProcessor); err != nil {
				return err
			}
		}
		return nil
	})
}

// markPurgePending 把列標成等待清理（記下時刻，已標過的保留最早的），排除在同步輪詢之外。
func (s *Service) markPurgePending(ctx context.Context, storeID int64) error {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.LockPaymentAccount(ctx, storeID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		_, err := q.MarkPaymentAccountPurgePending(ctx, store.MarkPaymentAccountPurgePendingParams{StoreID: storeID, Now: s.d.Now().UTC()})
		return err
	})
}
