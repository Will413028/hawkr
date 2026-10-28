package request

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion/internal/store"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/auditreason"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
)

// Buyer 與 Identity 只經公開 API 參與同一 transaction。BeginDeletion／CloseForDeletion 只在已以 LockAccount 鎖住帳號列的
// transaction 內呼叫（鎖順序：帳號列先於買家列）；關閉買家只有這一條路徑。
type Buyer interface {
	CurrentBuyerID(context.Context, int64) (int64, error)
	LockForDeletion(context.Context, int64) error
	BeginDeletion(context.Context, int64) error
	CloseForDeletion(context.Context, int64) error
}

// Seller 是店家的參與者動作，與 Buyer 相同：BeginDeletion／CloseForDeletion 只在已以 LockAccount 鎖住帳號列的
// transaction 內呼叫（鎖順序：帳號列先於買家列、店家列）。
type Seller interface {
	CurrentStore(context.Context, int64) (seller.Store, error)
	LockForDeletion(context.Context, int64) error
	BeginDeletion(context.Context, int64) error
	CloseForDeletion(context.Context, int64) error
	ClearBlocksAgainstBuyer(context.Context, int64) error
}
type Identity interface {
	Account(context.Context, int64) (identity.Account, error)
	LockAccount(context.Context, int64) error
	BeginDeletion(context.Context, int64) error
	DeleteAccount(context.Context, int64) error
}

type Deps struct {
	DB                 *db.DB
	Identity           Identity
	Buyer              Buyer
	Seller             Seller
	HasOpenOrders      func(context.Context, int64) (bool, error)
	HasOpenStoreOrders func(context.Context, int64) (bool, error)
	Now                func() time.Time
	NewID              func() uuid.UUID
	Logger             *slog.Logger
}
type Service struct{ d Deps }

func New(d Deps) *Service {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{d: d}
}

// normalize 驗證輸入並回傳正規化後的輸入：管理員的理由與關店的理由同一條規則（platform/auditreason：去掉前後空白，1 到 1000
// 字）；帳號本人的理由選填，只限長度。錯誤帶欄位（AccountID、OperatorAdminID、Reason），傳輸層對應到請求的路徑。
func normalize(in model.RequestInput) (model.RequestInput, error) {
	if in.AccountID <= 0 {
		return in, apperr.InvalidInputOn([]string{"AccountID"}, "帳號 ID 必須為正數")
	}
	switch in.Requester {
	case model.Admin:
		if in.OperatorAdminID == nil || *in.OperatorAdminID <= 0 {
			return in, apperr.InvalidInputOn([]string{"OperatorAdminID"}, "管理員 ID 必須為正數")
		}
		reason, err := auditreason.Required(in.Reason, "Reason")
		if err != nil {
			return in, err
		}
		in.Reason = reason
	case model.AccountOwner:
		if in.OperatorAdminID != nil {
			return in, apperr.InvalidInputOn([]string{"OperatorAdminID"}, "帳號本人申請不可指定管理員")
		}
		if len([]rune(in.Reason)) > auditreason.MaxRunes {
			return in, apperr.InvalidInputOn([]string{"Reason"}, "刪帳原因不可超過 1000 字")
		}
	default:
		return in, apperr.InvalidInputOn([]string{"Requester"}, "刪帳申請來源不合法")
	}
	return in, nil
}

// lockAccount 以 identity 的帳號列鎖序列化同一帳號的申請，以及與授予 SELLER、刪帳號之間的交錯。
func (s *Service) lockAccount(ctx context.Context, accountID int64) error {
	return s.d.Identity.LockAccount(ctx, accountID)
}

type snapshot struct {
	deleted, seller bool
	buyerID         int64
	// store 是帳號目前的店家，由 withStore 讀取（申請與完成等待中的申請都一樣）。
	store *seller.Store
}

// snapshot 讀取帳號與現任 buyer_id。帳號活著 ⇔ 買家活著（ADR 2026-09-30-account-created-provisions-buyer）：
// 未刪除的帳號沒有現任買家是不變量被破壞，回 internal 並記 error log，不當成「沒有買家」刪帳。
func (s *Service) snapshot(ctx context.Context, accountID int64) (snapshot, error) {
	a, err := s.d.Identity.Account(ctx, accountID)
	if err != nil {
		return snapshot{}, err
	}
	snap := snapshot{deleted: a.Deleted, seller: slices.Contains(a.Roles, identity.RoleSeller)}
	if snap.deleted {
		return snap, nil
	}
	buyerID, err := s.d.Buyer.CurrentBuyerID(ctx, accountID)
	if apperr.KindOf(err) == apperr.KindNotFound {
		// 交易外的讀取不是同一個 snapshot：讀帳號之後、讀買家之前，另一個申請可能剛好完成刪帳（同交易關閉買家）。
		// 重讀帳號，已刪除就照已刪除處理；仍存活才是不變量被破壞。
		again, aerr := s.d.Identity.Account(ctx, accountID)
		if aerr == nil && again.Deleted {
			return snapshot{deleted: true}, nil
		}
		return snapshot{}, s.brokenInvariant(ctx, accountID, "存活的帳號沒有現任買家", err)
	}
	if err != nil {
		return snapshot{}, err
	}
	snap.buyerID = buyerID
	return snap, nil
}

// withStore 讀取存活帳號目前的店家（不論有沒有 SELLER）。SELLER 與現任店家成對（開店與授予 SELLER、關店與移除 SELLER
// 各在同一個 transaction）：有店家卻沒有 SELLER、有 SELLER 卻沒有店家都是本模組以外的程式破壞了不變量，回 internal 並記
// error log。
func (s *Service) withStore(ctx context.Context, accountID int64, snap *snapshot) error {
	if snap.deleted {
		return nil
	}
	current, err := s.d.Seller.CurrentStore(ctx, accountID)
	switch {
	case apperr.KindOf(err) == apperr.KindNotFound:
		if snap.seller {
			return s.brokenInvariant(ctx, accountID, "帳號有 SELLER 但沒有目前的店家", nil)
		}
		return nil
	case err != nil:
		return err
	case !snap.seller:
		return s.brokenInvariant(ctx, accountID, "帳號有目前的店家但沒有 SELLER", nil)
	}
	snap.store = &current
	return nil
}

// openOrders 在 transaction 外查帳號有沒有未結訂單：買家的訂單，以及帳號目前店家（snap.store，呼叫方已讀）的訂單。買家先查，
// 已經有未結訂單就不必再問店家。任何一個查詢失敗都回 unavailable（fail-closed）：不確定就不刪、不進入等待。
func (s *Service) openOrders(ctx context.Context, snap snapshot) (bool, error) {
	open, err := s.d.HasOpenOrders(ctx, snap.buyerID)
	if err == nil && !open && snap.store != nil {
		open, err = s.d.HasOpenStoreOrders(ctx, snap.store.StoreID)
	}
	if err != nil {
		return false, apperr.WithCause(apperr.Unavailable("暫時無法確認未完成的訂單"), err)
	}
	return open, nil
}

// sameStore 回報交易外查訂單的店家與交易內讀到的是不是同一個 stint：不是的話（店家在查詢之後關閉又重新入駐）查到的
// 訂單不屬於現在的店家，要重來。
func sameStore(before, current *seller.Store) bool {
	if before == nil || current == nil {
		return before == current
	}
	return before.StoreID == current.StoreID
}

func errStoreChanged() error { return apperr.Unavailable("店家剛剛有變動，請重試") }

// pendingStore 回報店家是不是已隨這個帳號的刪除申請進入等待（完成等待中的申請時一起關閉）。
func pendingStore(snap snapshot) bool {
	return snap.store != nil && snap.store.Status == seller.PendingDeletion
}

func (s *Service) brokenInvariant(ctx context.Context, accountID int64, what string, cause error) error {
	s.d.Logger.ErrorContext(ctx, "accountdeletion："+what, "account_id", accountID, "error", cause)
	return fmt.Errorf("accountdeletion: account %d: %s", accountID, what)
}

// Request 的遠端查詢在 transaction 外；交易內先取 account-scoped lock，再重讀帳號（SELLER 可能在查詢之後才授予、
// 帳號可能已被刪除）。一個帳號一生只有一個買家，所以交易外查訂單用的 buyer_id 在交易內不會變成另一個，不必比對重試。
func (s *Service) Request(ctx context.Context, in model.RequestInput) (model.State, error) {
	in, err := normalize(in)
	if err != nil {
		return "", err
	}
	before, err := s.snapshot(ctx, in.AccountID)
	if err != nil {
		return "", err
	}
	open := false
	if !before.deleted {
		// 不論申請者，店家都是參與者：店家的訂單也要查。
		if err := s.withStore(ctx, in.AccountID, &before); err != nil {
			return "", err
		}
		if open, err = s.openOrders(ctx, before); err != nil {
			return "", err
		}
	}
	var state model.State
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.lockAccount(ctx, in.AccountID); err != nil {
			return err
		}
		current, err := s.snapshot(ctx, in.AccountID)
		if err != nil {
			return err
		}
		q := store.New(s.d.DB.Conn(ctx))
		now := s.d.Now().UTC()
		if current.deleted {
			row, err := s.recordCompleted(ctx, q, in.AccountID, now)
			if err != nil {
				return err
			}
			if err := s.attempt(ctx, q, row.ID, in, now); err != nil {
				return err
			}
			state = model.Completed
			return nil
		}
		if err := s.withStore(ctx, in.AccountID, &current); err != nil {
			return err
		}
		if !sameStore(before.store, current.store) {
			return errStoreChanged()
		}
		if open {
			if err := s.d.Identity.BeginDeletion(ctx, in.AccountID); err != nil {
				return err
			}
			if err := s.lockParticipants(ctx, current); err != nil {
				return err
			}
			if err := s.d.Buyer.BeginDeletion(ctx, current.buyerID); err != nil {
				return err
			}
			if current.store != nil {
				if err := s.d.Seller.BeginDeletion(ctx, current.store.StoreID); err != nil {
					return err
				}
			}
			row, err := q.UpsertPendingRequest(ctx, store.UpsertPendingRequestParams{ID: s.d.NewID(), AccountID: in.AccountID, RequestedAt: now})
			if err != nil {
				return err
			}
			if err := s.attempt(ctx, q, row.ID, in, now); err != nil {
				return err
			}
			state = model.PendingOrders
			return nil
		}
		if err := s.closeParticipants(ctx, current); err != nil {
			return err
		}
		if err := s.d.Identity.DeleteAccount(ctx, in.AccountID); err != nil {
			return err
		}
		row, err := s.recordCompleted(ctx, q, in.AccountID, now)
		if err != nil {
			return err
		}
		if err := s.attempt(ctx, q, row.ID, in, now); err != nil {
			return err
		}
		state = model.Completed
		return nil
	})
	return state, err
}

// CompletePending 重新查未結訂單；待辦不存在且帳號未刪除時拒絕。
func (s *Service) CompletePending(ctx context.Context, accountID int64) (model.State, error) {
	if accountID <= 0 {
		return "", apperr.InvalidInput("帳號 ID 必須為正數")
	}
	before, err := s.snapshot(ctx, accountID)
	if err != nil {
		return "", err
	}
	open := false
	if !before.deleted {
		// 先確認有待辦（便宜的 DB 讀取），沒有待辦的帳號不必對訂單與店家發 RPC；交易內鎖住帳號之後會再確認一次。
		if _, err := store.New(s.d.DB.Conn(ctx)).PendingRequest(ctx, accountID); errors.Is(err, pgx.ErrNoRows) {
			return "", apperr.FailedPrecondition("帳號沒有待完成的刪除")
		} else if err != nil {
			return "", err
		}
		if err := s.withStore(ctx, accountID, &before); err != nil {
			return "", err
		}
		if open, err = s.openOrders(ctx, before); err != nil {
			return "", err
		}
	}
	var state model.State
	err = s.d.DB.InTx(ctx, func(ctx context.Context) error {
		if err := s.lockAccount(ctx, accountID); err != nil {
			return err
		}
		current, err := s.snapshot(ctx, accountID)
		if err != nil {
			return err
		}
		if err := s.withStore(ctx, accountID, &current); err != nil {
			return err
		}
		if !current.deleted && !sameStore(before.store, current.store) {
			return errStoreChanged()
		}
		if current.seller && !current.deleted && !pendingStore(current) {
			return s.brokenInvariant(ctx, accountID, "等待中的申請，店家卻不在等待刪除", nil)
		}
		q := store.New(s.d.DB.Conn(ctx))
		pending, err := q.PendingRequest(ctx, accountID)
		if errors.Is(err, pgx.ErrNoRows) {
			if current.deleted {
				state = model.Completed
				return nil
			}
			return apperr.FailedPrecondition("帳號沒有待完成的刪除")
		}
		if err != nil {
			return err
		}
		if current.deleted {
			now := s.d.Now().UTC()
			_, err = q.CompleteRequest(ctx, store.CompleteRequestParams{ID: pending.ID, CompletedAt: &now})
			state = model.Completed
			return err
		}
		if open {
			state = model.PendingOrders
			return nil
		}
		if err := s.closeParticipants(ctx, current); err != nil {
			return err
		}
		if err := s.d.Identity.DeleteAccount(ctx, accountID); err != nil {
			return err
		}
		_, err = s.recordCompleted(ctx, q, accountID, s.d.Now().UTC())
		state = model.Completed
		return err
	})
	return state, err
}

// lockParticipants 依單體規則 4 的順序（帳號列已鎖，然後買家列、店家列）先鎖完買家與店家，之後才派發買家的刪除事件：事件的同 transaction
// 訂閱者（social）會鎖上層模組的列，必須排在店家列之後。重複鎖同一列是 no-op，所以 BeginDeletion 與 CloseForDeletion 各自再鎖不會反序。
// TestDeletionLocksBuyerAndSellerBeforeSocialRows 量這個順序。
func (s *Service) lockParticipants(ctx context.Context, snap snapshot) error {
	if err := s.d.Buyer.LockForDeletion(ctx, snap.buyerID); err != nil {
		return err
	}
	if snap.store == nil {
		return nil
	}
	return s.d.Seller.LockForDeletion(ctx, snap.store.StoreID)
}

// closeParticipants 在已鎖住帳號列的 transaction 內刪掉各店對這買家的封鎖（seller 擁有那張表）、關閉買家與（有的話）店家，之後由呼叫方刪除帳號。
// 店家封鎖在買家的刪除事件派發之前刪：事件的訂閱者鎖 social 的列，必須排在店家之後（規則 4）。
func (s *Service) closeParticipants(ctx context.Context, snap snapshot) error {
	if err := s.lockParticipants(ctx, snap); err != nil {
		return err
	}
	if err := s.d.Seller.ClearBlocksAgainstBuyer(ctx, snap.buyerID); err != nil {
		return err
	}
	if err := s.d.Buyer.CloseForDeletion(ctx, snap.buyerID); err != nil {
		return err
	}
	if snap.store == nil {
		return nil
	}
	return s.d.Seller.CloseForDeletion(ctx, snap.store.StoreID)
}

func (s *Service) recordCompleted(ctx context.Context, q *store.Queries, accountID int64, now time.Time) (store.AccountdeletionDeletionRequest, error) {
	pending, err := q.PendingRequest(ctx, accountID)
	if err == nil {
		return q.CompleteRequest(ctx, store.CompleteRequestParams{ID: pending.ID, CompletedAt: &now})
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return store.AccountdeletionDeletionRequest{}, err
	}
	last, err := q.LatestRequest(ctx, accountID)
	if err == nil {
		return last, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return store.AccountdeletionDeletionRequest{}, err
	}
	return q.InsertCompletedRequest(ctx, store.InsertCompletedRequestParams{ID: s.d.NewID(), AccountID: accountID, RequestedAt: now})
}
func (s *Service) attempt(ctx context.Context, q *store.Queries, requestID uuid.UUID, in model.RequestInput, now time.Time) error {
	return q.InsertRequestAttempt(ctx, store.InsertRequestAttemptParams{ID: s.d.NewID(), RequestID: requestID, Requester: string(in.Requester), OperatorAdminID: in.OperatorAdminID, Reason: in.Reason, AttemptedAt: now})
}
