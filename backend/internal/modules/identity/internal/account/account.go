package account

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/auditreason"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/trimtext"
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

// SuspendAccount 永久停權帳號：同一個 transaction 寫停權紀錄與 accounts.suspended_at，並刪除該帳號全部的 session。停權者仍能重新登入，
// 能做什麼由入口依動作的性質決定（A7-D17、A7-D18）。已經停權時不寫第二筆紀錄、不改理由，回傳原本的紀錄與 created=false。紀錄寫入失敗時
// 整個 transaction 回滾，suspended_at 不變。
func (m *Service) SuspendAccount(ctx context.Context, in model.SuspendInput) (rec model.AccountSuspension, created bool, err error) {
	row, err := suspensionRow(in)
	if err != nil {
		return rec, false, err
	}
	err = m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		a, err := lockLiveAccount(ctx, q, in.AccountID)
		if err != nil {
			return err
		}
		if a.SuspendedAt != nil {
			cur, err := q.CurrentAccountSuspension(ctx, in.AccountID)
			if err != nil {
				return fmt.Errorf("identity：讀取停權紀錄：%w", err)
			}
			rec, created = suspensionOf(cur), false
			return nil
		}
		now := m.d.Now()
		row.Now = now
		inserted, err := q.InsertAccountSuspension(ctx, row)
		if err != nil {
			return fmt.Errorf("identity：寫入停權紀錄：%w", err)
		}
		if _, err := q.SuspendAccount(ctx, store.SuspendAccountParams{ID: in.AccountID, Now: now}); err != nil {
			return fmt.Errorf("identity：停權：%w", err)
		}
		if _, err := q.DeleteSessionsOfAccount(ctx, in.AccountID); err != nil {
			return fmt.Errorf("identity：停權時刪除 session：%w", err)
		}
		rec, created = suspensionOf(inserted), true
		return nil
	})
	if err != nil {
		return model.AccountSuspension{}, false, err
	}
	if created {
		m.d.Logger.InfoContext(ctx, "identity：停權帳號", "account_id", in.AccountID, "category", in.Category)
	}
	return rec, created, nil
}

// UnsuspendAccount 手動解除停權：同一個 transaction 清 accounts.suspended_at，並把解除時間與原因記在目前的紀錄上。帳號沒有停權時
// 成功、不做任何事，回傳 released=false。reason 必填，規則同 platform/auditreason。
func (m *Service) UnsuspendAccount(ctx context.Context, accountID int64, reason string) (rec model.AccountSuspension, released bool, err error) {
	reason, err = auditreason.Required(reason, "Reason")
	if err != nil {
		return rec, false, err
	}
	if reason == model.ReleaseReasonAccountDeleted {
		return rec, false, apperr.InvalidInputOn([]string{"Reason"}, "解除原因不能是帳號刪除保留的值")
	}
	err = m.d.DB.InTx(ctx, func(ctx context.Context) error {
		q := m.q(ctx)
		if _, err := lockLiveAccount(ctx, q, accountID); err != nil {
			return err
		}
		now := m.d.Now()
		n, err := q.UnsuspendAccount(ctx, store.UnsuspendAccountParams{ID: accountID, Now: now})
		if err != nil {
			return fmt.Errorf("identity：解除停權：%w", err)
		}
		if n == 0 {
			return nil
		}
		row, err := q.ReleaseAccountSuspension(ctx, store.ReleaseAccountSuspensionParams{AccountID: accountID, Now: now, ReleaseReason: &reason})
		if err != nil {
			return fmt.Errorf("identity：記錄解除停權：%w", err)
		}
		rec, released = suspensionOf(row), true
		return nil
	})
	if err != nil {
		return model.AccountSuspension{}, false, err
	}
	if released {
		m.d.Logger.InfoContext(ctx, "identity：解除停權", "account_id", accountID)
	}
	return rec, released, nil
}

// CurrentSuspension 回傳帳號目前未解除的停權紀錄；沒有停權時 ok 是 false。帳號不存在回 NotFound。
func (m *Service) CurrentSuspension(ctx context.Context, accountID int64) (rec model.AccountSuspension, ok bool, err error) {
	q := m.q(ctx)
	row, err := q.CurrentAccountSuspension(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := m.Account(ctx, accountID); err != nil {
			return rec, false, err
		}
		return rec, false, nil
	}
	if err != nil {
		return rec, false, fmt.Errorf("identity：讀取停權紀錄：%w", err)
	}
	return suspensionOf(row), true, nil
}

// suspensionRow 驗證輸入並轉成寫入的參數：類別必須是政策分類的值，說明與來源參照去掉前後空白。
func suspensionRow(in model.SuspendInput) (store.InsertAccountSuspensionParams, error) {
	if !slices.Contains(policy.Values(), in.Category) {
		return store.InsertAccountSuspensionParams{}, apperr.InvalidInputOn([]string{"Category"}, "停權的理由類別 %q 不是政策分類", in.Category)
	}
	reason, err := auditreason.Optional(in.Reason, "Reason")
	if err != nil {
		return store.InsertAccountSuspensionParams{}, err
	}
	var ref *string
	if r := trimtext.Trim(in.SourceRef); r != "" {
		if utf8.RuneCountInString(r) > maxSourceRefRunes {
			return store.InsertAccountSuspensionParams{}, apperr.InvalidInputOn([]string{"SourceRef"}, "來源參照不得超過 %d 字", maxSourceRefRunes)
		}
		ref = &r
	}
	return store.InsertAccountSuspensionParams{
		AccountID: in.AccountID, ReasonCategory: string(in.Category), ReasonText: reason, SourceRef: ref,
	}, nil
}

// maxSourceRefRunes 是 account_suspensions.source_ref 的字數上限（CHECK 以 char_length，即字元數計）。
const maxSourceRefRunes = 200

func suspensionOf(r store.IdentityAccountSuspension) model.AccountSuspension {
	return model.AccountSuspension{
		ID: r.ID, AccountID: r.AccountID, Kind: model.SuspensionKind(r.Kind), Category: policy.Category(r.ReasonCategory),
		Reason: r.ReasonText, SourceRef: deref(r.SourceRef), SuspendedAt: r.SuspendedAt,
		ReleasedAt: r.ReleasedAt, ReleaseReason: deref(r.ReleaseReason),
	}
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

// deleteAccount 在呼叫方的 transaction 內刪除已鎖定的帳號：解除未解除的停權紀錄（release_reason 為 ACCOUNT_DELETED），刪角色、全部 session 與登入身分，帳號只留沒有個資的墓碑（登入身分留作墓碑，
// 同一個登入身分不會再建一個帳號），並發出 AccountDeleted；所有刪除路徑都經過這裡。
func (m *Service) deleteAccount(ctx context.Context, q *store.Queries, a store.IdentityAccount) error {
	now := m.d.Now()
	if a.SuspendedAt != nil {
		reason := model.ReleaseReasonAccountDeleted
		if _, err := q.ReleaseAccountSuspension(ctx, store.ReleaseAccountSuspensionParams{AccountID: a.ID, Now: now, ReleaseReason: &reason}); err != nil {
			return fmt.Errorf("identity：解除停權紀錄：%w", err)
		}
	}
	if err := q.DeleteAccountRoles(ctx, a.ID); err != nil {
		return fmt.Errorf("identity：刪除角色：%w", err)
	}
	if _, err := q.DeleteAccount(ctx, store.DeleteAccountParams{ID: a.ID, Now: now}); err != nil {
		return fmt.Errorf("identity：刪除帳號：%w", err)
	}
	if _, err := q.DeleteSessionsOfAccount(ctx, a.ID); err != nil {
		return fmt.Errorf("identity：刪除 session：%w", err)
	}
	if err := q.ScrubIdentitiesOfAccount(ctx, store.ScrubIdentitiesOfAccountParams{AccountID: a.ID, Now: now}); err != nil {
		return fmt.Errorf("identity：清除登入身分的個資：%w", err)
	}
	m.d.Logger.InfoContext(ctx, "identity：刪除帳號", "account_id", a.ID)
	return events.Publish(ctx, m.d.Events, model.AccountDeleted{AccountID: a.ID, DeletedAt: now})
}

type Deps struct {
	DB     *db.DB
	Events *events.Bus
	Now    func() time.Time
	Logger *slog.Logger
}

type Service struct{ d Deps }

func New(d Deps) *Service { return &Service{d: d} }

func (m *Service) q(ctx context.Context) *store.Queries { return store.New(m.d.DB.Conn(ctx)) }

// CreateDevAccount 建立一個沒有任何登入身分的帳號（發出 AccountCreated）並回傳 ID。dev 使用者以帳號 ID 識別，不建假的 account_identities。
// 只給本機 dev token 與測試。
func (m *Service) CreateDevAccount(ctx context.Context) (int64, error) {
	var id int64
	err := m.d.DB.InTx(ctx, func(ctx context.Context) error {
		var err error
		id, err = m.q(ctx).InsertFederatedAccount(ctx, store.InsertFederatedAccountParams{Now: m.d.Now()})
		if err != nil {
			return fmt.Errorf("identity：建立 dev 帳號：%w", err)
		}
		return events.Publish(ctx, m.d.Events, model.AccountCreated{AccountID: id, Name: "dev"})
	})
	return id, err
}

// RequireDevAccount 確認帳號存在且未刪除；不存在回 NotFound，已刪除回 Unauthenticated。只給本機 dev token 與測試。
func (m *Service) RequireDevAccount(ctx context.Context, accountID int64) error {
	acct, err := m.q(ctx).GetAccount(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("帳號 %d 不存在", accountID)
	} else if err != nil {
		return fmt.Errorf("identity：讀取 dev 帳號：%w", err)
	}
	if acct.DeletedAt != nil {
		return apperr.Unauthenticated("dev 帳號 %d 已刪除", accountID)
	}
	return nil
}
