# identity/internal/session

顧客 session 的存放、逾時與換發。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/identity.md`。

## 不變量

- session 只存 token 的 SHA-256；閒置逾時 web 7 天、App 14 天，絕對逾時 90 天，`last_used_at` 一天最多寫一次；登入一律換發新 token。停權與刪帳在同一個 transaction 刪該帳號全部 session。
