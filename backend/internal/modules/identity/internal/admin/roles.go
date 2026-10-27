package admin

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/identity/internal/store"
	"github.com/Will413028/hawkr/backend/internal/platform/adminperm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/auditreason"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

// GrantRole 授予已存在的管理員一個角色，角色列與事件在同一個 transaction 內寫入。管理員不存在、角色不是目錄裡的
// 角色、沒有理由回 InvalidInput 或 NotFound；已經有這個角色回 AlreadyExists，不寫事件。
func (m *Service) GrantRole(ctx context.Context, in model.AdminRoleChangeInput) (model.AdminRoleChange, error) {
	reason, err := m.validate(in)
	if err != nil {
		return model.AdminRoleChange{}, err
	}
	var out model.AdminRoleChange
	err = m.d.DB.InTx(ctx, func(ctx context.Context) error {
		id, err := m.resolve(ctx, in)
		if err != nil {
			return err
		}
		q := m.q(ctx)
		now := m.d.Now()
		n, err := q.InsertAdminRole(ctx, store.InsertAdminRoleParams{AdminID: id, Role: string(in.Role), Now: now})
		if err != nil {
			return fmt.Errorf("identity：授予管理員角色：%w", err)
		}
		if n == 0 {
			return apperr.AlreadyExists("管理員 %d 已經有角色 %s", id, in.Role)
		}
		if err := m.insertEvent(ctx, id, in, model.AdminRoleActionGrant, reason, now); err != nil {
			return err
		}
		out = model.AdminRoleChange{AdminID: id, Role: in.Role, Action: model.AdminRoleActionGrant}
		m.d.Logger.InfoContext(ctx, "identity：授予管理員角色", "admin_id", id, "role", in.Role, "source", in.Source)
		return nil
	})
	return out, err
}

// RevokeRole 撤銷管理員的一個角色：刪除角色列、寫事件並發出 AdminRoleRevoked，全在同一個 transaction 內，訂閱者
// （delegation 撤銷進行中的代行）失敗時整個撤銷回滾。管理員沒有這個角色回 FailedPrecondition，不寫事件。撤掉之後
// 沒有任何管理員持有該角色是允許的（CLI 本身就是 break-glass），結果的 HolderRemains 讓 CLI 警告。
func (m *Service) RevokeRole(ctx context.Context, in model.AdminRoleChangeInput) (model.AdminRoleChange, error) {
	reason, err := m.validate(in)
	if err != nil {
		return model.AdminRoleChange{}, err
	}
	var out model.AdminRoleChange
	err = m.d.DB.InTx(ctx, func(ctx context.Context) error {
		id, err := m.resolve(ctx, in)
		if err != nil {
			return err
		}
		q := m.q(ctx)
		now := m.d.Now()
		n, err := q.DeleteAdminRole(ctx, store.DeleteAdminRoleParams{AdminID: id, Role: string(in.Role)})
		if err != nil {
			return fmt.Errorf("identity：撤銷管理員角色：%w", err)
		}
		if n == 0 {
			return apperr.FailedPrecondition("管理員 %d 沒有角色 %s", id, in.Role)
		}
		if err := m.insertEvent(ctx, id, in, model.AdminRoleActionRevoke, reason, now); err != nil {
			return err
		}
		if err := events.Publish(ctx, m.d.Events, model.AdminRoleRevoked{AdminID: id, Role: in.Role, RevokedAt: now}); err != nil {
			return err
		}
		held, err := q.AdminRoleHolderExists(ctx, string(in.Role))
		if err != nil {
			return fmt.Errorf("identity：查詢角色的持有者：%w", err)
		}
		out = model.AdminRoleChange{AdminID: id, Role: in.Role, Action: model.AdminRoleActionRevoke, HolderRemains: held}
		m.d.Logger.InfoContext(ctx, "identity：撤銷管理員角色", "admin_id", id, "role", in.Role, "source", in.Source)
		return nil
	})
	return out, err
}

// AdminAllows 在呼叫方的 transaction 內讀管理員的角色（FOR SHARE）並回答角色是否允許權限 perm。鎖住的角色列讓撤銷
// 與這次讀取互相等待：讀到允許的人，撤銷要等呼叫方的 transaction 結束才會生效，而撤銷已經先進行時這裡讀不到該角色。
// 不在 transaction 內呼叫回錯誤（鎖在 transaction 之外沒有意義）。
func (m *Service) AdminAllows(ctx context.Context, adminID int64, perm adminperm.Permission) (bool, error) {
	if _, ok := db.TxFrom(ctx); !ok {
		return false, errors.New("identity：AdminAllows 必須在呼叫方的 transaction 內呼叫")
	}
	rows, err := m.q(ctx).ListAdminRolesForShare(ctx, adminID)
	if err != nil {
		return false, fmt.Errorf("identity：讀取管理員的角色：%w", err)
	}
	roles := make([]adminperm.Role, len(rows))
	for i, r := range rows {
		roles[i] = adminperm.Role(r)
	}
	return adminperm.Allows(roles, perm), nil
}

func (m *Service) validate(in model.AdminRoleChangeInput) (string, error) {
	switch {
	case (in.AdminID > 0) == (in.Email != ""):
		return "", apperr.InvalidInputOn([]string{"AdminID", "Email"}, "管理員要以 ID 或 email 擇一指定")
	case !slices.Contains(adminperm.Roles(), in.Role):
		return "", apperr.InvalidInputOn([]string{"Role"}, "不是管理員的角色：%q", in.Role)
	case in.Source != model.AdminRoleSourceCLI:
		return "", apperr.InvalidInputOn([]string{"Source"}, "不認得的來源：%q", in.Source)
	}
	return auditreason.Required(in.Reason, "Reason")
}

// resolve 回傳已存在的管理員的 ID；不會建立管理員。
func (m *Service) resolve(ctx context.Context, in model.AdminRoleChangeInput) (int64, error) {
	q := m.q(ctx)
	if in.AdminID > 0 {
		if _, err := q.GetAdmin(ctx, in.AdminID); errors.Is(err, pgx.ErrNoRows) {
			return 0, apperr.NotFound("管理員 %d 不存在", in.AdminID)
		} else if err != nil {
			return 0, fmt.Errorf("identity：讀取管理員：%w", err)
		}
		return in.AdminID, nil
	}
	ids, err := q.AdminIDsByEmail(ctx, in.Email)
	if err != nil {
		return 0, fmt.Errorf("identity：以 email 讀取管理員：%w", err)
	}
	switch len(ids) {
	case 0:
		return 0, apperr.NotFound("沒有 email 是 %s 的管理員（要先以 Access 登入一次）", in.Email)
	case 1:
		return ids[0], nil
	}
	return 0, apperr.InvalidInputOn([]string{"Email"}, "email %s 對應多位管理員，請改用 ID", in.Email)
}

func (m *Service) insertEvent(ctx context.Context, adminID int64, in model.AdminRoleChangeInput, action model.AdminRoleAction, reason string, now time.Time) error {
	var osUser *string
	if in.OSUser != "" {
		osUser = &in.OSUser
	}
	err := m.q(ctx).InsertAdminRoleEvent(ctx, store.InsertAdminRoleEventParams{
		AdminID: adminID, Role: string(in.Role), Action: string(action), Reason: reason,
		Source: string(in.Source), OsUser: osUser, Now: now,
	})
	if err != nil {
		return fmt.Errorf("identity：寫入管理員角色的事件：%w", err)
	}
	return nil
}
