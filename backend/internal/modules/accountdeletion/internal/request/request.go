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

func (s *Service) lockAccount(ctx context.Context, accountID int64) error {
	return s.d.Identity.LockAccount(ctx, accountID)
}

type snapshot struct {
	deleted, seller bool
	buyerID         int64
	store *seller.Store
}

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

func sameStore(before, current *seller.Store) bool {
	if before == nil || current == nil {
		return before == current
	}
	return before.StoreID == current.StoreID
}

func errStoreChanged() error { return apperr.Unavailable("店家剛剛有變動，請重試") }

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
			_, err = s.completeRequest(ctx, q, pending, now)
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

func (s *Service) lockParticipants(ctx context.Context, snap snapshot) error {
	if err := s.d.Buyer.LockForDeletion(ctx, snap.buyerID); err != nil {
		return err
	}
	if snap.store == nil {
		return nil
	}
	return s.d.Seller.LockForDeletion(ctx, snap.store.StoreID)
}

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
		return s.completeRequest(ctx, q, pending, now)
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

func (s *Service) completeRequest(ctx context.Context, q *store.Queries, pending store.AccountdeletionDeletionRequest, now time.Time) (store.AccountdeletionDeletionRequest, error) {
	row, err := q.CompleteRequest(ctx, store.CompleteRequestParams{ID: pending.ID, CompletedAt: &now})
	if errors.Is(err, pgx.ErrNoRows) {
		return q.RequestByID(ctx, pending.ID)
	}
	return row, err
}

func (s *Service) attempt(ctx context.Context, q *store.Queries, requestID uuid.UUID, in model.RequestInput, now time.Time) error {
	return q.InsertRequestAttempt(ctx, store.InsertRequestAttemptParams{ID: s.d.NewID(), RequestID: requestID, Requester: string(in.Requester), OperatorAdminID: in.OperatorAdminID, Reason: in.Reason, AttemptedAt: now})
}
