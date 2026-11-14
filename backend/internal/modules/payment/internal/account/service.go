package account

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/payment/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/inbox"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

const (
	// callTimeout 是單次呼叫 provider 的上限。
	callTimeout = 20 * time.Second
	// maxProviderCallsPerJob 是一次 job 最多的 provider 呼叫數：建立是取得、設定；清理是取得（重送）、關閉。
	maxProviderCallsPerJob = 2
	// jobTimeout 是一次 job 的上限：最多的 provider 呼叫各佔 callTimeout，加上幾次寫結果的 transaction（writeTimeout 內）。
	jobTimeout = maxProviderCallsPerJob*callTimeout + 4*writeTimeout
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
	// IdempotencyRetention 是金流商保證同一把冪等鍵回同一個結果的時間（Stripe 保存 30 天）；超過之後重送同一把鍵可能建出新的帳戶，所以從不對超過它的鍵重送。
	IdempotencyRetention = 30 * 24 * time.Hour
	// UnresolvedDeadline 是「呼叫過、沒有外部 ID」的列從 create_called_at 起最久等多久（D63）：期限內週期掃描以同一把鍵重送取回 ID；到了仍了結不了就進終局
	// （清理的列進 purge_blocked，其餘進建立失敗），不再重送、不換新鍵、不列出。必須小於 IdempotencyRetention，所以任何重送都在保存期內。
	UnresolvedDeadline = 7 * 24 * time.Hour
	// PurgeDeadline 是列被標成等待清理之後，仍然關不掉或問不到 provider 的最長時間，超過進終局（D60：既有重試是單一 job 8 次約 80 分鐘、掃描每 15 分鐘，
	// 7 天涵蓋金流商故障與金鑰設定修正）。
	PurgeDeadline = 7 * 24 * time.Hour
	// EventRetention 是收款事件原始 payload 在 inbox 的保存期（D29）：涵蓋 Stripe 重送事件的 3 天與事後查核的時間。
	EventRetention = 30 * 24 * time.Hour
	// syncProcessor、eventProcessor 與 purgeProcessor 是 inbox 的處理者名稱。
	syncProcessor  = "payment.account_sync"
	eventProcessor = "payment.account_event"
	purgeProcessor = "payment.account_purge"
)

// UnresolvedDeadline 必須小於冪等鍵保存期：差值為負時這行無法編譯。
const _ = uint64(IdempotencyRetention - UnresolvedDeadline - 1)

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
	DB     *db.DB
	Events *events.Bus
	Jobs   jobs.Enqueuer
	Inbox  *inbox.Inbox
	// Audit 是 platform 的後台動作紀錄（D62）：管理員的寫入（重試建立、重試關閉、刪除對應）在領域變更的同一個 transaction 內寫一筆。
	// 只有後台寫入需要它：組裝點（cmd/hawkr）一律傳入；沒有傳入時這三個寫入回錯誤、不做領域變更（沒有紀錄的後台寫入不成立）。
	Audit    *adminaudit.Log
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
// platform/lockorder），關店再開沿用同一個外部帳戶；既有列還沒完成建立、也沒有進入建立失敗時重新排入建立 job（唯一鍵合併）。
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
func ToModel(r store.PaymentAccount, now time.Time) model.PaymentAccount {
	return model.PaymentAccount{
		PurgePendingAt: r.PurgePendingAt, CreateUnresolved: r.PurgePendingAt == nil && r.CreateFailedAt != nil && keyExpired(r, now),
		StoreID: r.StoreID, Provider: r.Provider, ExternalAccountID: r.ExternalAccountID,
		RequirementCollection: model.RequirementCollection(r.RequirementCollection), Status: model.PaymentAccountStatus(r.Status),
		TransfersActive: r.TransfersActive, PayoutsActive: r.PayoutsActive, Requirements: r.Requirements, StatusReason: r.StatusReason,
		FetchedAt: r.FetchedAt, CreateFailedAt: r.CreateFailedAt, PurgeBlockedAt: r.PurgeBlockedAt, PurgeBlockedReason: r.PurgeBlockedReason,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
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
	return ToModel(row, s.d.Now().UTC()), nil
}

// Ensure 是建立 job 的內容：先讀本地列，確認擁有者營運中（Owners），在 transaction 外讀擁有者目前的聯絡信箱（D50），分兩步建立外部帳戶（D55、D59）：
//  0. 呼叫 provider 之前先在 transaction 內記「已呼叫」的時刻（create_called_at；之後列上分得出「從沒呼叫」與「結果不明」）；
//  1. 取得外部 ID（同一把冪等鍵重送得到同一個帳戶；結果不明超過 UnresolvedDeadline 的列不重送，進建立失敗，D63），
//     拿到 ID 立刻在 transaction 內寫進列（不等第二步）；
//  2. 完成設定（寫入信箱與收款設定），成功才在 transaction 內記 create_completed_at 並排入同步 job：從這一刻起帳戶才算已建立，可以開通、輪詢、同步。
//
// 已完成建立、已進入失敗、等待清理、擁有者已關閉時什麼都不做。擁有者沒有聯絡信箱時不呼叫 provider，視為永久失敗（重試不會憑空生出信箱；擁有者補填之後由管理員重試建立）。
// 暫時性的失敗回傳錯誤由 River 退避重試；final 為 true（最後一次嘗試）或失敗是永久的時，記下建立失敗的時刻（進後台待辦 PAYMENT_ACCOUNT_CREATE_FAILED）並結束；
// 第一步已寫進列的外部 ID 不會因此丟掉，管理員重試建立從第二步接續。
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
	if unresolvedExpired(row, s.d.Now().UTC()) {
		return s.createFailed(ctx, storeID, fmt.Errorf("%w：建立結果不明，超過 %s 仍沒有取回外部 ID", ErrPermanent, UnresolvedDeadline), final)
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
	if !hasEmail {
		return s.createFailed(ctx, storeID, fmt.Errorf("%w：擁有者沒有可用的聯絡信箱", ErrPermanent), final)
	}
	row, ok, err := s.markCalled(ctx, storeID)
	if err != nil || !ok {
		return err
	}
	externalID := ""
	if row.ExternalAccountID != nil {
		externalID = *row.ExternalAccountID
	} else {
		id, stored, err := s.acquire(ctx, row)
		if err != nil {
			return s.createFailed(ctx, storeID, err, final)
		}
		if !stored {
			return nil
		}
		externalID = id
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	if err := p.ConfigureAccount(callCtx, ConfigureInput{ExternalID: externalID, ContactEmail: email, Collection: model.RequirementCollection(row.RequirementCollection)}); err != nil {
		return s.createFailed(ctx, storeID, err, final)
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.LockPaymentAccount(ctx, storeID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if _, err := q.CompletePaymentAccountCreate(ctx, store.CompletePaymentAccountCreateParams{StoreID: storeID, Now: s.d.Now().UTC()}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		return enqueueSync(ctx, s.d.Jobs, storeID)
	})
}

// markCalled 在呼叫 provider 之前，於 transaction 內記下「已呼叫」的時刻並回傳最新的列；列已不需要建立（已完成、等待清理、不在了）時 ok 為 false。
func (s *Service) markCalled(ctx context.Context, storeID int64) (row store.PaymentAccount, ok bool, err error) {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	err = s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.LockPaymentAccount(ctx, storeID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		r, err := q.MarkPaymentAccountCreateCalled(ctx, store.MarkPaymentAccountCreateCalledParams{StoreID: storeID, Now: s.d.Now().UTC()})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		row, ok = r, err == nil
		return err
	})
	return row, ok && err == nil, err
}

// acquire 是建立的第一步：以列的冪等鍵取得外部 ID 並立刻寫進列。呼叫方已確認列還在 UnresolvedDeadline 內（超過的不重送，D63）。
// stored 為 false 表示列已不需要建立（被並行的清理標記或已刪），ID 沒有寫入（清理會以冪等鍵找回它）。
func (s *Service) acquire(ctx context.Context, row store.PaymentAccount) (id string, stored bool, err error) {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	if id, err = s.d.Provider.AcquireAccount(callCtx, AcquireInput{StoreID: row.StoreID, IdempotencyKey: row.CreateIdempotencyKey.String()}); err != nil {
		return "", false, err
	}
	stored, err = s.storeExternal(ctx, row.StoreID, id, false)
	return id, stored, err
}

// calledWithoutID 回傳列是否「呼叫過、沒有外部 ID、還沒完成建立」：建立結果不明的列。
func calledWithoutID(row store.PaymentAccount) bool {
	return row.ExternalAccountID == nil && row.CreateCompletedAt == nil && row.CreateCalledAt != nil
}

// unresolvedExpired 回傳列是否為結果不明而且已超過 UnresolvedDeadline：自動的路徑（建立 job、週期掃描、清理）不再重送（終局，D63）。
// 管理員重試可以在冪等鍵保存期內（keyExpired 為假）以同一把鍵重送。
func unresolvedExpired(row store.PaymentAccount, now time.Time) bool {
	return calledWithoutID(row) && now.Sub(*row.CreateCalledAt) > UnresolvedDeadline
}

// keyExpired 回傳列是否為結果不明而且已超過 IdempotencyRetention：同一把鍵不再保證回同一個帳戶，任何路徑都不重送；
// 只有管理員確認金流商沒有這家店的帳戶之後才換新鍵（D63）。
func keyExpired(row store.PaymentAccount, now time.Time) bool {
	return calledWithoutID(row) && now.Sub(*row.CreateCalledAt) > IdempotencyRetention
}

// storeExternal 在 transaction 內把取得的外部 ID 寫進列（第一步拿到就寫）；列不需要它了（等待清理、已完成、已刪、已有外部 ID）時 stored 為 false。
// forPurge 為真時寫進已標記等待清理的列（清理取得的 ID，讓之後的關閉走有外部 ID 的路），否則寫進沒有等待清理的列。
func (s *Service) storeExternal(ctx context.Context, storeID int64, external string, forPurge bool) (stored bool, err error) {
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	err = s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.LockPaymentAccount(ctx, storeID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		var err error
		if forPurge {
			_, err = q.SetPaymentAccountExternalForPurge(ctx, store.SetPaymentAccountExternalForPurgeParams{StoreID: storeID, ExternalAccountID: &external, Now: s.d.Now().UTC()})
		} else {
			_, err = q.SetPaymentAccountExternal(ctx, store.SetPaymentAccountExternalParams{StoreID: storeID, ExternalAccountID: &external, Now: s.d.Now().UTC()})
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		stored = err == nil
		return err
	})
	return stored && err == nil, err
}

// createFailed 處理建立時的失敗：暫時性且不是最後一次嘗試時回傳錯誤由 River 重試；否則記下建立失敗的時刻（進後台待辦）並結束。
func (s *Service) createFailed(ctx context.Context, storeID int64, callErr error, final bool) error {
	permanent := errors.Is(callErr, ErrPermanent)
	if !permanent && !final {
		return callErr
	}
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
		s.d.Logger.WarnContext(ctx, "payment：建立收款帳戶失敗", "store_id", storeID, "permanent", permanent, "error", callErr)
		now := s.d.Now().UTC()
		_, err := q.MarkPaymentAccountCreateFailed(ctx, store.MarkPaymentAccountCreateFailedParams{StoreID: storeID, Now: &now})
		return err
	})
}

// Resolve 是「呼叫過、沒有外部 ID」的非清理列的了結（D63）：期限內以列的同一把冪等鍵重送 AcquireAccount 取回 ID 寫進列（不論擁有者是否營運中、是否已記建立失敗；
// 取回之後列是半建立：擁有者營運中時由建立的保底補排接續第二步，建立失敗的列等管理員重試）；超過 UnresolvedDeadline 仍沒有 ID 就進建立失敗（後台待辦），不重送、不換新鍵。
// 等待清理的列不在這裡：清理 job 對它做同樣的重送與終局（purgeReplay、purgeUnresolved）。暫時性失敗回傳錯誤由 River 重試，最後一次嘗試仍失敗就留著給下一次掃描。
func (s *Service) Resolve(ctx context.Context, storeID int64, final bool) error {
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
	if row.Provider != p.Name() || row.PurgePendingAt != nil || row.CreateCalledAt == nil || row.ExternalAccountID != nil || row.CreateCompletedAt != nil {
		return nil
	}
	if unresolvedExpired(row, s.d.Now().UTC()) {
		return s.createFailed(ctx, storeID, fmt.Errorf("%w：建立結果不明，超過 %s 仍沒有取回外部 ID", ErrPermanent, UnresolvedDeadline), final)
	}
	if _, _, err := s.acquire(ctx, row); err != nil {
		switch {
		case errors.Is(err, ErrPermanent):
			return s.createFailed(ctx, storeID, err, final)
		case !final:
			return err
		}
		s.d.Logger.WarnContext(ctx, "payment：取回建立結果不明的外部帳戶失敗，等下一次掃描重排", "store_id", storeID, "error", err)
	}
	return nil
}

// Replay 是管理員在冪等鍵保存期內重試「結果不明、已超過 UnresolvedDeadline」的建立失敗列（D63）：列保持建立失敗（週期掃描因此不會再把它標成終局），
// 以列上同一把鍵重送 AcquireAccount；取回 ID 就在同一個 transaction 寫進列、清空建立失敗並排入建立 job 接續第二步。已呼叫的時刻與鍵都不變。
// 超過 IdempotencyRetention、列已不是這個狀態時什麼都不做。永久失敗或最後一次嘗試仍失敗只記警示 log，列留在建立失敗，管理員可再重試。
func (s *Service) Replay(ctx context.Context, storeID int64, final bool) error {
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
	if row.Provider != p.Name() || row.PurgePendingAt != nil || row.CreateFailedAt == nil || !calledWithoutID(row) || keyExpired(row, s.d.Now().UTC()) {
		return nil
	}
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	id, err := p.AcquireAccount(callCtx, AcquireInput{StoreID: row.StoreID, IdempotencyKey: row.CreateIdempotencyKey.String()})
	if err != nil {
		if errors.Is(err, ErrPermanent) || final {
			s.d.Logger.WarnContext(ctx, "payment：管理員重試以同一把鍵取回外部帳戶失敗，列留在建立失敗", "store_id", storeID, "error", err)
			return nil
		}
		return err
	}
	wctx, cancelWrite := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancelWrite()
	return s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.LockPaymentAccount(ctx, storeID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		now := s.d.Now().UTC()
		if _, err := q.SetPaymentAccountExternal(ctx, store.SetPaymentAccountExternalParams{StoreID: storeID, ExternalAccountID: &id, Now: now}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		if _, err := q.ClearPaymentAccountCreateFailed(ctx, store.ClearPaymentAccountCreateFailedParams{StoreID: storeID, Now: now}); err != nil {
			return err
		}
		return enqueueEnsure(ctx, s.d.Jobs, storeID)
	})
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
	// purgeDelete 是直接刪列：從沒呼叫過 provider（沒有「已呼叫」的時刻，所以沒有外部帳戶），不需要 provider。
	purgeDelete purgePlan = iota
	// purgeClose 是有外部 ID、provider 可用：先讀帳戶、照它實際的設定關閉，成功才刪列。
	purgeClose
	// purgeReplay 是呼叫過、沒有外部 ID、還在 UnresolvedDeadline（7 天，小於冪等鍵保存期）內：以同一把鍵取得 ID 寫回，再關閉（D55）。
	purgeReplay
	// purgeUnresolved 是呼叫過、沒有外部 ID、超過 UnresolvedDeadline：不再重送也不列出，進終局（原因 blockUnresolvedCreation），等管理員（D63）。
	purgeUnresolved
	// purgeDefer 是呼叫過、但問不到 provider（沒有設定，或列的 provider 與目前設定不同）：列留著，超過期限進終局。
	purgeDefer
)

// planPurge 決定清理一列的處理方式；provider 是目前設定的 provider 代碼，沒有設定時是空字串。
func planPurge(row store.PaymentAccount, provider string, now time.Time) purgePlan {
	switch {
	case row.CreateCalledAt == nil:
		return purgeDelete
	case provider == "" || row.Provider != provider:
		return purgeDefer
	case row.ExternalAccountID != nil:
		return purgeClose
	case now.Sub(*row.CreateCalledAt) <= UnresolvedDeadline:
		return purgeReplay
	default:
		return purgeUnresolved
	}
}

// 清理進終局的原因代碼（purge_blocked_reason 的開頭，後面接冒號與細節）。
const (
	blockProviderRefused     = "provider_refused"
	blockProviderUnreachable = "provider_unconfigured"
	blockDeadlineExceeded    = "deadline_exceeded"
	blockUnresolvedCreation  = "unresolved_creation"
)

// Purge 是「關閉後刪除」job 的內容（D52、D59）：第一步，每一列先在自己的 transaction 標成等待清理（purge_pending_at，之後任何錯誤、逾時或程序被殺都有週期掃描接手），
// 標好之後重讀列，依 planPurge 處理：
//   - 從沒呼叫過 provider 的列直接刪，不需要 provider；
//   - 有外部 ID 而 provider 可用：在 transaction 外先讀帳戶、照它實際的設定關閉（冪等：已關閉視為成功），成功才刪列並把它外部帳戶在 inbox 的未處理事件標為已處理；
//   - 呼叫過、沒有外部 ID、provider 可用：UnresolvedDeadline（7 天）內以同一把鍵取得 ID 寫回再關閉；超過進終局（unresolved_creation），不重送、不列出（D63）；
//   - 呼叫過、問不到 provider（沒有設定或不同）：列留著。
//
// 終局：provider 明確拒絕（永久失敗），或標成等待清理已超過 purgeDeadline（7 天，D60）而仍然關不掉或問不到 provider，列標成 purge_blocked_at（原因 purge_blocked_reason），
// 進後台待辦 PAYMENT_ACCOUNT_PURGE_STUCK，週期掃描與這個 job 都不再處理它，直到管理員「重試關閉」或「刪除對應」。暫時性的失敗回傳錯誤由 River 退避重試；
// 最後一次嘗試仍失敗而還沒逾期時列留著（已標記），週期掃描重排。
func (s *Service) Purge(ctx context.Context, storeID int64, final bool) error {
	q := store.New(s.d.DB.Conn(ctx))
	row, err := q.PaymentAccountByStore(ctx, storeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.PurgeBlockedAt != nil {
		return nil
	}
	provider := ""
	if s.d.Provider != nil {
		provider = s.d.Provider.Name()
	}
	if row.PurgePendingAt == nil {
		if err := s.markPurgePending(ctx, storeID); err != nil {
			return err
		}
		if row, err = q.PaymentAccountByStore(ctx, storeID); errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
	}
	now := s.d.Now().UTC()
	expired := row.PurgePendingAt != nil && now.After(row.PurgePendingAt.Add(PurgeDeadline))
	switch planPurge(row, provider, now) {
	case purgeDelete:
		return s.deleteRow(ctx, row, nil)
	case purgeDefer:
		if expired {
			return s.blockPurge(ctx, storeID, blockProviderUnreachable+"：金流商沒有設定或與列的 provider 不同")
		}
		s.d.Logger.WarnContext(ctx, "payment：收款帳戶的 provider 與目前設定不同或沒有設定，沒有關閉外部帳戶也沒有刪除，等設定回來再清理", "store_id", storeID, "row_provider", row.Provider, "configured_provider", provider)
		return nil
	case purgeClose:
		return s.closeThenDelete(ctx, row, *row.ExternalAccountID, final, expired)
	case purgeReplay:
		callCtx, cancel := context.WithTimeout(ctx, callTimeout)
		external, err := s.d.Provider.AcquireAccount(callCtx, AcquireInput{StoreID: storeID, IdempotencyKey: row.CreateIdempotencyKey.String()})
		cancel()
		if err != nil {
			return s.purgeFailed(ctx, row, nil, err, final, expired)
		}
		return s.storeThenClose(ctx, row, external, final, expired)
	case purgeUnresolved:
		return s.blockPurge(ctx, storeID, blockUnresolvedCreation+"：建立結果不明，超過 "+UnresolvedDeadline.String()+" 仍沒有取回外部 ID，不再重送")
	}
	return nil
}

// storeThenClose 把取得的外部 ID 寫回已標記等待清理的列，再走和有外部 ID 的列相同的關閉後刪除。順序是關鍵：關閉之後同一把鍵的重放會被金流商拒絕（measured：409
// idempotency_error，永久），所以 ID 一定要在關閉之前存下，關閉成功但刪列失敗的重試就不必再取得已關閉的帳戶。列已經不在時結束。
func (s *Service) storeThenClose(ctx context.Context, row store.PaymentAccount, external string, final, expired bool) error {
	if _, err := s.storeExternal(ctx, row.StoreID, external, true); err != nil {
		return err
	}
	fresh, err := store.New(s.d.DB.Conn(ctx)).PaymentAccountByStore(ctx, row.StoreID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if fresh.ExternalAccountID == nil || *fresh.ExternalAccountID != external {
		return nil
	}
	return s.closeThenDelete(ctx, fresh, external, final, expired)
}

// closeThenDelete 在 transaction 外關閉外部帳戶（冪等：已關閉視為成功），成功才刪列；每次 provider 呼叫有自己的逾時。
func (s *Service) closeThenDelete(ctx context.Context, row store.PaymentAccount, external string, final, expired bool) error {
	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	if err := s.d.Provider.CloseAccount(callCtx, external); err != nil {
		return s.purgeFailed(ctx, row, &external, err, final, expired)
	}
	return s.deleteRow(ctx, row, &external)
}

// purgeFailed 處理清理時 provider 呼叫的失敗：永久失敗（provider 明確拒絕）或已超過期限進終局；暫時性且不是最後一次嘗試時回傳錯誤由 River 重試；
// 否則本地列（已標記）留著並記警示 log，等週期掃描重排。
func (s *Service) purgeFailed(ctx context.Context, row store.PaymentAccount, external *string, err error, final, expired bool) error {
	permanent := errors.Is(err, ErrPermanent)
	ext := ""
	if external != nil {
		ext = *external
	}
	switch {
	case permanent:
		return s.blockPurge(ctx, row.StoreID, blockProviderRefused+"："+err.Error())
	case expired && ctx.Err() == nil:
		// job 自己被取消或逾時（ctx.Err）不算 provider 的失敗，不能因此進終局；留給下一輪。
		return s.blockPurge(ctx, row.StoreID, blockDeadlineExceeded+"："+err.Error())
	case !final:
		return err
	}
	s.d.Logger.WarnContext(ctx, "payment：取得或關閉外部帳戶失敗，本地列留著等週期掃描重排", "store_id", row.StoreID, "external_account_id", ext, "permanent", permanent, "error", err)
	return nil
}

// blockPurge 把列標成清理的終局（原因截成 200 字元內）並記警示 log；已在終局的保留最早的。
func (s *Service) blockPurge(ctx context.Context, storeID int64, reason string) error {
	reason = clipReason(reason)
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	err := s.d.DB.InTx(wctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		if _, err := q.LockPaymentAccount(ctx, storeID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return err
		}
		_, err := q.BlockPaymentAccountPurge(ctx, store.BlockPaymentAccountPurgeParams{StoreID: storeID, Now: s.d.Now().UTC(), Reason: reason})
		return err
	})
	if err == nil {
		s.d.Logger.WarnContext(ctx, "payment：收款帳戶的清理進入終局，等管理員處理", "store_id", storeID, "reason", reason)
	}
	return err
}

// RetryPurge 是管理員的「重試關閉」：只有終局的列可以重試（清除終局、等待清理的時刻重設成現在，並在同一個 transaction 重新排入清理 job、寫一筆後台紀錄，
// 紀錄說的是「已要求重試」：關閉本身由之後的清理 job 呼叫 provider）。
func (s *Service) RetryPurge(ctx context.Context, actor adminaudit.Actor, storeID int64) (model.PaymentAccount, error) {
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
		if locked.PurgeBlockedAt == nil {
			return apperr.FailedPrecondition("收款帳戶的清理不在無法關閉的狀態")
		}
		if reason := locked.PurgeBlockedReason; reason != nil && strings.HasPrefix(*reason, blockUnresolvedCreation) {
			return apperr.FailedPrecondition("建立結果不明且已超過可向金流商查回的期限，重試關閉不會有結果；請先到金流商後台確認，再刪除對應")
		}
		if out, err = q.RestartPaymentAccountPurge(ctx, store.RestartPaymentAccountPurgeParams{StoreID: storeID, Now: s.d.Now().UTC()}); err != nil {
			return err
		}
		if err := s.audit(ctx, actor, storeID, map[string]any{"outcome": "requested", "blocked_reason": codeOf(locked.PurgeBlockedReason)}); err != nil {
			return err
		}
		return enqueuePurge(ctx, s.d.Jobs, storeID)
	})
	if err != nil {
		return model.PaymentAccount{}, err
	}
	return ToModel(out, s.d.Now().UTC()), nil
}

// DeleteMapping 是管理員的「已在金流商處理，刪除對應」：只有終局的列可以做；同一個 transaction 寫一筆後台紀錄（platform adminaudit：誰、何時、哪家店，detail 帶 provider 與
// 外部 ID 與終局原因代碼，D62）並刪列（外部帳戶在 inbox 的未處理事件標為已處理）。不呼叫 provider。紀錄比列活得久，由 cmd/hawkr 的營運子命令讀回。
func (s *Service) DeleteMapping(ctx context.Context, actor adminaudit.Actor, storeID int64) error {
	if err := model.Positive(storeID); err != nil {
		return err
	}
	return s.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := store.New(s.d.DB.Conn(ctx))
		locked, err := q.LockPaymentAccount(ctx, storeID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("店家還沒有收款帳戶")
		}
		if err != nil {
			return err
		}
		if locked.PurgeBlockedAt == nil {
			return apperr.FailedPrecondition("收款帳戶的清理不在無法關閉的狀態")
		}
		detail := map[string]any{"provider": locked.Provider, "blocked_reason": codeOf(locked.PurgeBlockedReason)}
		if locked.ExternalAccountID != nil {
			detail["external_account_id"] = *locked.ExternalAccountID
		}
		if err := s.audit(ctx, actor, storeID, detail); err != nil {
			return err
		}
		return s.deleteLocked(ctx, q, storeID)
	})
}

// auditTargetType 是後台紀錄裡收款帳戶的對象種類；對象 ID 是店家 ID（擁有者）。
const auditTargetType = "payment_account"

// errAuditNotConfigured 表示組裝漏了 Deps.Audit：後台寫入沒有紀錄就不成立。
var errAuditNotConfigured = errors.New("payment：沒有設定後台動作紀錄（Deps.Audit）")

// audit 在呼叫方的 transaction 內寫一筆後台紀錄。
func (s *Service) audit(ctx context.Context, actor adminaudit.Actor, storeID int64, detail map[string]any) error {
	if s.d.Audit == nil {
		return errAuditNotConfigured
	}
	return s.d.Audit.Record(ctx, adminaudit.Entry{Actor: actor, TargetType: auditTargetType, TargetID: strconv.FormatInt(storeID, 10), Detail: detail})
}

// codeOf 取終局原因的代碼（冒號之前的部分；後面是金流商的說明，可能含細節，不進後台紀錄）。
func codeOf(reason *string) string {
	if reason == nil {
		return ""
	}
	code, _, _ := strings.Cut(*reason, ":")
	code, _, _ = strings.Cut(code, "：")
	return code
}

// createWanted 是列是否還需要建立外部帳戶：還沒完成建立（第二步成功）、沒有進入建立失敗、沒有等待清理（標記了就不建立）。
func createWanted(row store.PaymentAccount) bool {
	return row.CreateCompletedAt == nil && row.CreateFailedAt == nil && row.PurgePendingAt == nil
}

// errRowChanged 是計畫直接刪列、鎖住之後卻發現列已經變了（並行的建立剛呼叫過 provider 或剛寫入外部 ID）：不能刪，回傳錯誤讓 job 重試並重新決定。
var errRowChanged = errors.New("payment：要直接刪的帳戶列已經變了，重新決定清理方式")

// lockedRowMayBeDeleted 在鎖住列之後再檢查它是否還能照計畫刪：planned 是 nil（計畫是直接刪或對帳後沒有帳戶）時列上必須仍沒有外部 ID 且「已呼叫」的時刻沒變
// （planRow 是計畫依據的列），否則回 errRowChanged；planned 不是 nil（已關閉的那個帳戶）時，列上的外部 ID 必須還是它，否則不刪也不是錯誤。
func lockedRowMayBeDeleted(locked, planRow store.PaymentAccount, planned *string) (bool, error) {
	if planned == nil {
		sameCall := (locked.CreateCalledAt == nil) == (planRow.CreateCalledAt == nil) &&
			(locked.CreateCalledAt == nil || locked.CreateCalledAt.Equal(*planRow.CreateCalledAt))
		if locked.ExternalAccountID != nil || !sameCall {
			return false, errRowChanged
		}
		return true, nil
	}
	return locked.ExternalAccountID != nil && *locked.ExternalAccountID == *planned, nil
}

// deleteRow 刪掉擁有者的列；鎖住之後依 lockedRowMayBeDeleted 再檢查（external 是 nil 表示計畫是直接刪）。row 是計畫依據的列。
func (s *Service) deleteRow(ctx context.Context, row store.PaymentAccount, external *string) error {
	storeID := row.StoreID
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
		if ok, err := lockedRowMayBeDeleted(locked, row, external); !ok {
			return err
		}
		return s.deleteLocked(ctx, q, storeID)
	})
}

// deleteLocked 在已鎖住列的 transaction 內刪列，並把它外部帳戶在 inbox 裡到現在為止的未處理事件標為已處理（帳戶不在了，不會再有同步涵蓋它們；
// 不標記的話 platform 的清理永遠不刪，原始 payload 一直留著；inbox 列之後由 platform 的清理依保存期刪除）。
func (s *Service) deleteLocked(ctx context.Context, q *store.Queries, storeID int64) error {
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
