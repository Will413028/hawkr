package identity

import "github.com/Will413028/hawkr/backend/internal/modules/identity/internal/model"

// AccountCreated 在建帳的 transaction 內發出。
type AccountCreated = model.AccountCreated

// AccountDeleted 在帳號刪除的 transaction 內發出。
type AccountDeleted = model.AccountDeleted

// AdminRoleRevoked 在撤銷管理員角色的 transaction 內發出。
type AdminRoleRevoked = model.AdminRoleRevoked
