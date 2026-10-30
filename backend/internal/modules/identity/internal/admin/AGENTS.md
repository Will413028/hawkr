# identity/internal/admin

管理員角色。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/identity.md`。

## 不變量

- 授予、撤銷管理員角色與寫 `admin_role_events` 在同一個 transaction；事件只追加；撤銷在同一個 transaction 內發出 `AdminRoleRevoked`，授予不發。規格：§5。
- `AdminAllows` 在呼叫方的 transaction 內以 `FOR SHARE` 讀角色列，與撤銷互相等待。規格：§5。
- 每個請求都讀角色、不快取；零角色沒有任何權限。規格：§5。
