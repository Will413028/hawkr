package account

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
)

func errAccountNotFound(id int64) error { return apperr.NotFound("帳號 %d 不存在", id) }

// Account 回傳帳號，含已刪除的墓碑。
func (m *Service) Account(ctx context.Context, id int64) (model.Account, error) {
	accounts, err := m.Accounts(ctx, []int64{id})
	if err != nil {
		return model.Account{}, err
	}
	if len(accounts) == 0 {
		return model.Account{}, errAccountNotFound(id)
	}
	return accounts[0], nil
}

// Accounts 回傳 ids 裡存在的帳號（含已刪除的墓碑），依 ID 排序；不存在的 ID 略過。
func (m *Service) Accounts(ctx context.Context, ids []int64) ([]model.Account, error) {
	q := m.q(ctx)
	rows, err := q.GetAccounts(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("identity：讀取帳號：%w", err)
	}
	roles, err := q.ListRolesOfAccounts(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("identity：讀取角色：%w", err)
	}
	byAccount := map[int64][]model.Role{}
	for _, r := range roles {
		byAccount[r.AccountID] = append(byAccount[r.AccountID], model.Role(r.Role))
	}
	out := make([]model.Account, len(rows))
	for i, r := range rows {
		out[i] = model.Account{
			ID:        r.ID,
			Email:     deref(r.Email),
			Roles:     byAccount[r.ID],
			Suspended: r.SuspendedAt != nil,
			Lifecycle: lifecycle(r),
			Deleted:   r.DeletedAt != nil,
			CreatedAt: r.CreatedAt,
		}
	}
	return out, nil
}

func lifecycle(r store.IdentityAccount) model.Lifecycle {
	switch {
	case r.DeletedAt != nil:
		return model.LifecycleDeleted
	case r.DeletionPendingAt != nil:
		return model.LifecyclePendingDeletion
	default:
		return model.LifecycleActive
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// GrantRole 授予角色。SourceID 是授予它的業務身分（store_id）；已有同角色時改指到它。
func (m *Service) GrantRole(ctx context.Context, in model.GrantRoleInput) error {
	if err := knownRole(in.Role); err != nil {
		return err
	}
	return m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		a, err := lockLiveAccount(ctx, q, in.AccountID)
		if err != nil {
			return err
		}
		if a.DeletionPendingAt != nil {
			return apperr.FailedPrecondition("帳號正在刪除中，不能再授予角色")
		}
		if _, err := q.GrantAccountRole(ctx, store.GrantAccountRoleParams{
			AccountID: in.AccountID, Role: string(in.Role), SourceID: in.SourceID, Now: m.d.Now(),
		}); err != nil {
			return fmt.Errorf("identity：授予角色：%w", err)
		}
		return nil
	})
}

// LockAccount 在呼叫者的 transaction 內鎖住存活的帳號，與授予、移除角色及刪帳號序列化；帳號已刪除時不鎖
// （墓碑不再改變），從未存在時回 NotFound。
func (m *Service) LockAccount(ctx context.Context, id int64) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return errors.New("identity：LockAccount 必須在 transaction 內呼叫")
	}
	_, err := lockLiveAccount(ctx, m.q(ctx), id)
	if apperr.KindOf(err) != apperr.KindNotFound {
		return err
	}
	_, err = m.Account(ctx, id)
	return err
}

// lockLiveAccount 鎖住存活的帳號；不存在或已刪除時回 NotFound。
func lockLiveAccount(ctx context.Context, q *store.Queries, id int64) (store.IdentityAccount, error) {
	a, err := q.LockLiveAccount(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, errAccountNotFound(id)
	}
	if err != nil {
		return a, fmt.Errorf("identity：鎖定帳號：%w", err)
	}
	return a, nil
}

// BeginDeletion 讓存活的帳號進入等待刪除（PENDING_DELETION）；已在等待時成功（冪等）。只由 accountdeletion 在它的
// transaction 內、受理會等待未結訂單的申請時呼叫；之後 GrantRole 一律拒絕，直到刪除完成（DeleteAccount 清掉這個狀態）。
func (m *Service) BeginDeletion(ctx context.Context, id int64) error {
	return m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		if _, err := lockLiveAccount(ctx, q, id); err != nil {
			return err
		}
		if _, err := q.MarkDeletionPending(ctx, store.MarkDeletionPendingParams{ID: id, Now: m.d.Now()}); err != nil {
			return fmt.Errorf("identity：進入等待刪除：%w", err)
		}
		return nil
	})
}

// RemoveRole 移除角色。帳號活著就是買家，所以移除角色永遠不刪帳號（刪帳由 accountdeletion 編排）。可以重試：帳號
// 已經刪除時成功、不做任何事；從來不存在時回 NotFound。
func (m *Service) RemoveRole(ctx context.Context, in model.RemoveRoleInput) error {
	if err := knownRole(in.Role); err != nil {
		return err
	}
	return m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		_, err := lockLiveAccount(ctx, q, in.AccountID)
		if apperr.KindOf(err) == apperr.KindNotFound {
			deleted, derr := m.isDeleted(ctx, q, in.AccountID)
			if derr != nil || !deleted {
				return errors.Join(err, derr)
			}
			return nil
		} else if err != nil {
			return err
		}
		r, err := q.RevokeAccountRole(ctx, store.RevokeAccountRoleParams{AccountID: in.AccountID, Role: string(in.Role), SourceID: in.SourceID})
		if err != nil {
			return fmt.Errorf("identity：移除角色：%w", err)
		}
		if !r.Revoked && r.HadRole {
			m.d.Logger.InfoContext(ctx, "identity：角色屬於較新的來源，不移除", "account_id", in.AccountID, "role", in.Role)
		}
		return nil
	})
}

// knownRole 擋下不是角色的值（例如舊的 BUYER：買家不是角色）。
func knownRole(r model.Role) error {
	if !slices.Contains(model.Roles(), r) {
		return apperr.InvalidInput("%q 不是可以授予或移除的角色", r)
	}
	return nil
}

func (m *Service) isDeleted(ctx context.Context, q *store.Queries, id int64) (bool, error) {
	a, err := q.GetAccount(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("identity：讀取帳號：%w", err)
	}
	return a.DeletedAt != nil, nil
}

// SuspendAccount 停權帳號：下一個請求起回 PermissionDenied（認證時讀停權的狀態），並在同一個 transaction 排入 job 停用
// Clerk 的使用者（撤銷他的 session、之後不能再登入）。已經停權時成功（冪等）。reason 只寫進 log。
func (m *Service) SuspendAccount(ctx context.Context, accountID int64, reason string) error {
	err := m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		a, err := lockLiveAccount(ctx, q, accountID)
		if err != nil {
			return err
		}
		n, err := q.SuspendAccount(ctx, store.SuspendAccountParams{ID: accountID, Now: m.d.Now()})
		if err != nil {
			return fmt.Errorf("identity：停權：%w", err)
		}
		if n == 0 {
			return nil // 已經停權
		}
		return m.enqueueClerkUserJob(ctx, clerkUserActionBan, a.ClerkUserID)
	})
	if err != nil {
		return err
	}
	m.d.Logger.InfoContext(ctx, "identity：停權帳號", "account_id", accountID, "reason", reason)
	return nil
}

// DeleteAccount 刪除帳號（`docs/modules/identity.md` §4）。已經刪除時成功（冪等）；從來不存在時回 NotFound。
func (m *Service) DeleteAccount(ctx context.Context, accountID int64) error {
	return m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		a, err := lockLiveAccount(ctx, q, accountID)
		if apperr.KindOf(err) == apperr.KindNotFound {
			deleted, derr := m.isDeleted(ctx, q, accountID)
			if derr != nil || !deleted {
				return errors.Join(err, derr)
			}
			return nil
		} else if err != nil {
			return err
		}
		return m.deleteAccount(ctx, q, a)
	})
}

// deleteAccount 在呼叫方的 transaction 內刪除已鎖定的帳號：刪角色，帳號只留沒有個資的墓碑（保留 clerk_user_id，
// 同一個 Clerk 使用者不會再建一個帳號），排入 job 刪除 Clerk 的使用者，並發出 AccountDeleted。帳號保存的 Sign in with Apple
// refresh token 在同一個 transaction 內取走並排入撤銷的 job（Clerk 不會代為撤銷）；所有刪除路徑都經過這裡。
func (m *Service) deleteAccount(ctx context.Context, q *store.Queries, a store.IdentityAccount) error {
	now := m.d.Now()
	if err := q.DeleteAccountRoles(ctx, a.ID); err != nil {
		return fmt.Errorf("identity：刪除角色：%w", err)
	}
	if _, err := q.DeleteAccount(ctx, store.DeleteAccountParams{ID: a.ID, Now: now}); err != nil {
		return fmt.Errorf("identity：刪除帳號：%w", err)
	}
	if err := m.enqueueClerkUserJob(ctx, clerkUserActionDelete, a.ClerkUserID); err != nil {
		return err
	}
	if err := m.d.AppleRevoker.RevokeStored(ctx, a.ID); err != nil {
		return err
	}
	m.d.Logger.InfoContext(ctx, "identity：刪除帳號", "account_id", a.ID)
	return events.Publish(ctx, m.d.Events, model.AccountDeleted{AccountID: a.ID, DeletedAt: now})
}

type Deps struct {
	DB        *db.DB
	Events    *events.Bus
	Jobs      jobs.Enqueuer
	Customers []model.Clerk
	// AppleRevoker 在刪除帳號的 transaction 內排入撤銷保存的 Apple token；必填。
	AppleRevoker AppleRevoker
	Now          func() time.Time
	Logger       *slog.Logger
}

// AppleRevoker 取走帳號保存的 Apple refresh token 並排入撤銷；沒有保存時什麼都不做。
type AppleRevoker interface {
	RevokeStored(ctx context.Context, accountID int64) error
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

// CreateForClerk 建立首次登入的帳號；並發建立時回傳 created=false。
func (m *Service) CreateForClerk(ctx context.Context, userID, email, name string) (id int64, created bool, err error) {
	err = m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		var insertErr error
		id, insertErr = q.InsertAccount(ctx, store.InsertAccountParams{ClerkUserID: userID, Email: nonEmpty(email), Now: m.d.Now()})
		if errors.Is(insertErr, pgx.ErrNoRows) {
			return nil
		}
		if insertErr != nil {
			return fmt.Errorf("identity：建立帳號：%w", insertErr)
		}
		created = true
		return events.Publish(ctx, m.d.Events, model.AccountCreated{AccountID: id, Name: name})
	})
	return
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (m *Service) q(ctx context.Context) *store.Queries { return store.New(m.d.DB.Conn(ctx)) }
